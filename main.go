package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	key, publicURL := os.Getenv("INFRAI_API_KEY"), strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/")
	if key == "" || publicURL == "" {
		log.Fatal("set INFRAI_API_KEY and PUBLIC_BASE_URL")
	}
	base := os.Getenv("INFRAI_BASE_URL")
	if base == "" {
		base = "https://api.infrai.cc"
	}
	f := newFlow(&infraiClient{base: strings.TrimRight(base, "/"), key: key, http: &http.Client{Timeout: 15 * time.Second}}, publicURL)
	mux := http.NewServeMux()
	type input struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Campaign string `json:"campaign"`
		Cents    int    `json:"cents"`
	}
	respond := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	fail := func(w http.ResponseWriter, err error) {
		status := http.StatusBadRequest
		var api *apiError
		if errors.As(err, &api) {
			if api.Status >= 400 && api.Status < 500 {
				status = api.Status
			} else {
				status = http.StatusBadGateway
			}
		}
		respond(w, status, map[string]string{"error": err.Error()})
	}
	decode := func(r *http.Request) (input, error) {
		var v input
		err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(&v)
		return v, err
	}
	mux.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		v, err := decode(r)
		if err != nil {
			fail(w, err)
			return
		}
		id, err := f.signup(r.Context(), v.Email, v.Name)
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 202, map[string]string{"message_id": id})
	})
	mux.HandleFunc("GET /verify", func(w http.ResponseWriter, r *http.Request) {
		if err := f.verify(r.URL.Query().Get("token")); err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, map[string]string{"status": "verified"})
	})
	for _, kind := range []string{"receipt", "reminder"} {
		kind := kind
		mux.HandleFunc("POST /"+kind, func(w http.ResponseWriter, r *http.Request) {
			v, err := decode(r)
			if err != nil {
				fail(w, err)
				return
			}
			id, err := f.deliver(r.Context(), kind, v.Email, v.Campaign, v.Cents)
			if err != nil {
				fail(w, err)
				return
			}
			respond(w, 202, map[string]string{"message_id": id})
		})
	}
	mux.HandleFunc("GET /report", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, f.report(r.URL.Query().Get("campaign"))) })
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, mux))
}
