package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

type gateway interface {
	createUser(context.Context, string, string, string) error
	send(context.Context, string, string, string, string) (string, error)
}
type event struct {
	Kind, Campaign string
	Cents          int
}
type flow struct {
	mu        sync.Mutex
	api       gateway
	publicURL string
	pending   map[string]string
	verified  map[string]bool
	events    []event
}

func newFlow(api gateway, publicURL string) *flow {
	return &flow{api: api, publicURL: publicURL, pending: make(map[string]string), verified: make(map[string]bool)}
}
func randomID() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func (f *flow) signup(ctx context.Context, email, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") || name == "" {
		return "", errors.New("email and name required")
	}
	if f.verified[email] {
		return "", errors.New("already verified")
	}
	token, err := randomID()
	if err != nil {
		return "", err
	}
	if err = f.api.createUser(ctx, email, name, "signup:"+email); err != nil {
		return "", err
	}
	link := f.publicURL + "/verify?token=" + url.QueryEscape(token)
	id, err := f.api.send(ctx, email, "Verify your nonprofit signup", "Open this link to verify your email: "+link, "verify:"+email)
	if err != nil {
		return "", err
	}
	f.pending[token] = email
	return id, nil
}
func (f *flow) verify(token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	email, ok := f.pending[token]
	if !ok {
		return errors.New("invalid verification link")
	}
	delete(f.pending, token)
	f.verified[email] = true
	return nil
}
func (f *flow) deliver(ctx context.Context, kind, email, campaign string, cents int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	email = strings.ToLower(strings.TrimSpace(email))
	if !f.verified[email] {
		return "", errors.New("email not verified")
	}
	if campaign == "" || (kind == "receipt" && cents <= 0) {
		return "", errors.New("campaign and positive receipt amount required")
	}
	subject, body := "Volunteer reminder", fmt.Sprintf("Your volunteer shift for %s is coming up.", campaign)
	if kind == "receipt" {
		subject = "Donation receipt"
		body = fmt.Sprintf("Thank you for your %d-cent donation to %s.", cents, campaign)
	}
	id, err := f.api.send(ctx, email, subject, body, kind+":"+campaign+":"+email+fmt.Sprint(cents))
	if err != nil {
		return "", err
	}
	f.events = append(f.events, event{kind, campaign, cents})
	return id, nil
}

type campaignReport struct {
	Receipts     int `json:"receipts"`
	DonatedCents int `json:"donated_cents"`
	Reminders    int `json:"reminders"`
}

func (f *flow) report(campaign string) campaignReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out campaignReport
	for _, e := range f.events {
		if e.Campaign != campaign {
			continue
		}
		if e.Kind == "receipt" {
			out.Receipts++
			out.DonatedCents += e.Cents
		} else {
			out.Reminders++
		}
	}
	return out
}
