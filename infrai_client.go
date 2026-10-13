package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *apiError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}
type infraiClient struct {
	base, key string
	http      *http.Client
}

func (c *infraiClient) post(ctx context.Context, path string, body any, id string) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", id)
		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if err != nil {
			return nil, err
		}
		var env envelope
		decoded := json.Unmarshal(raw, &env) == nil
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(strings.TrimSpace(res.Header.Get("Retry-After"))); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if decoded && !env.OK {
			if env.Error != nil {
				env.Error.Status = res.StatusCode
				return nil, env.Error
			}
			return nil, &apiError{Code: "REJECTED", Message: "request rejected", Status: res.StatusCode}
		}
		if res.StatusCode >= 400 {
			return nil, fmt.Errorf("Infrai HTTP %d", res.StatusCode)
		}
		if !decoded || !env.OK {
			return nil, errors.New("invalid Infrai response envelope")
		}
		return env.Data, nil
	}
	return nil, errors.New("retry limit reached")
}

func (c *infraiClient) createUser(ctx context.Context, email, name, id string) error {
	_, err := c.post(ctx, "/v1/auth/user/create", map[string]any{"email": email, "name": name, "idempotency_key": id}, id)
	return err
}
func (c *infraiClient) send(ctx context.Context, to, subject, body, id string) (string, error) {
	raw, err := c.post(ctx, "/v1/email/send", map[string]string{"to": to, "subject": subject, "body": body}, id)
	if err != nil {
		return "", err
	}
	var data struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", err
	}
	if data.MessageID == "" {
		return "", errors.New("missing message_id")
	}
	return data.MessageID, nil
}
