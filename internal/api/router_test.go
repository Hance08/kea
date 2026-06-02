// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hance08/kea/internal/config"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:        "localhost",
			Port:        0,
			CORSOrigins: []string{"http://localhost:5173"},
		},
	}
	srv := NewServer(cfg, nil, discardLogger())
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	return ts
}

func TestHealthEndpoint(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header missing")
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body status: got %q, want %q", body["status"], "ok")
	}
}

func TestVersionEndpoint(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/version")
	if err != nil {
		t.Fatalf("GET /api/version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["version"] != Version {
		t.Errorf("body version: got %q, want %q", body["version"], Version)
	}
}

func TestUnknownRouteReturns404(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/unknown")
	if err != nil {
		t.Fatalf("GET /api/unknown: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestCORSPreflightAllowed(t *testing.T) {
	ts := newTestServer(t)

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Allow-Origin: got %q, want %q", got, "http://localhost:5173")
	}
}

func TestCORSDisallowedOrigin(t *testing.T) {
	ts := newTestServer(t)

	req, _ := http.NewRequest("GET", ts.URL+"/api/health", nil)
	req.Header.Set("Origin", "http://evil.example")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin should be empty for disallowed origin, got %q", got)
	}
}
