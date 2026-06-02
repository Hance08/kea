// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware(t *testing.T) {
	var observedID string
	h := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedID = requestIDFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

	if observedID == "" {
		t.Error("request ID not stored in context")
	}
	if got := rr.Header().Get("X-Request-ID"); got == "" {
		t.Error("X-Request-ID response header missing")
	}
	if observedID != rr.Header().Get("X-Request-ID") {
		t.Error("context ID and response header ID disagree")
	}
}

func TestLoggerFromFallback(t *testing.T) {
	l := loggerFrom(context.Background())
	if l == nil {
		t.Fatal("loggerFrom must never return nil")
	}
	l.Info("smoke")
}

func TestLoggerMiddlewareAttachesLogger(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	chain := requestIDMiddleware(loggerMiddleware(base)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loggerFrom(r.Context()).Info("hello")
		w.WriteHeader(http.StatusOK)
	})))

	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log not valid JSON: %v", err)
	}
	if entry["request_id"] == nil || entry["request_id"] == "" {
		t.Errorf("logger entry missing request_id: %v", entry)
	}
}

func TestAccessLogMiddlewareCapturesStatus(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	h := requestIDMiddleware(loggerMiddleware(base)(accessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/coffee", nil))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log not valid JSON: %v", err)
	}
	if entry["status"] != float64(http.StatusTeapot) {
		t.Errorf("status: got %v, want %d", entry["status"], http.StatusTeapot)
	}
	if entry["method"] != "GET" {
		t.Errorf("method: got %v, want GET", entry["method"])
	}
	if entry["path"] != "/coffee" {
		t.Errorf("path: got %v, want /coffee", entry["path"])
	}
}

func TestCORSMiddlewareAllowedOrigin(t *testing.T) {
	h := corsMiddleware([]string{"http://localhost:5173"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Allow-Origin: got %q, want %q", got, "http://localhost:5173")
	}
	if rr.Header().Get("Vary") == "" {
		t.Error("Vary header missing")
	}
}

func TestCORSMiddlewareDisallowedOrigin(t *testing.T) {
	h := corsMiddleware([]string{"http://localhost:5173"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://evil.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin should be empty for disallowed origin, got %q", got)
	}
}

func TestCORSMiddlewarePreflight(t *testing.T) {
	called := false
	h := corsMiddleware([]string{"http://localhost:5173"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("preflight status: got %d, want 204", rr.Code)
	}
	if called {
		t.Error("preflight should short-circuit before next handler")
	}
}

// discardLogger is a convenience for tests that need a logger but ignore output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
