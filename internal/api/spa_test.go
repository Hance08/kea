// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testSPAFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":              {Data: []byte("<!doctype html><html><body>shell</body></html>")},
		"favicon.svg":             {Data: []byte("<svg/>")},
		"assets/app-abc123.js":    {Data: []byte("console.log('app');")},
		"assets/style-def456.css": {Data: []byte("body{}")},
	}
}

func doSPARequest(t *testing.T, h http.Handler, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr.Result()
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func TestSPAHandler_ServesIndexAtRoot(t *testing.T) {
	h := spaHandler(testSPAFS())
	resp := doSPARequest(t, h, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	if body := readBody(t, resp); !strings.Contains(body, "shell") {
		t.Errorf("body does not contain shell marker: %q", body)
	}
}

func TestSPAHandler_ServesNamedFile(t *testing.T) {
	h := spaHandler(testSPAFS())
	resp := doSPARequest(t, h, "/index.html")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body := readBody(t, resp); !strings.Contains(body, "shell") {
		t.Errorf("expected shell body, got %q", body)
	}
}

func TestSPAHandler_ServesHashedAssetWithImmutableCache(t *testing.T) {
	h := spaHandler(testSPAFS())
	resp := doSPARequest(t, h, "/assets/app-abc123.js")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q, want to contain javascript", ct)
	}
	cc := resp.Header.Get("Cache-Control")
	if !strings.Contains(cc, "immutable") || !strings.Contains(cc, "max-age=31536000") {
		t.Errorf("Cache-Control = %q, want immutable + max-age=31536000", cc)
	}
	if body := readBody(t, resp); !strings.Contains(body, "console.log") {
		t.Errorf("body does not contain JS marker: %q", body)
	}
}

func TestSPAHandler_ServesFaviconWithoutImmutableCache(t *testing.T) {
	h := spaHandler(testSPAFS())
	resp := doSPARequest(t, h, "/favicon.svg")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "svg") {
		t.Errorf("Content-Type = %q, want to contain svg", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, should not be immutable for non-asset path", cc)
	}
}

func TestSPAHandler_FallsBackToIndexForUnknownPath(t *testing.T) {
	h := spaHandler(testSPAFS())
	resp := doSPARequest(t, h, "/accounts")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", ct)
	}
	if body := readBody(t, resp); !strings.Contains(body, "shell") {
		t.Errorf("expected shell body for SPA fallback, got %q", body)
	}
}

func TestSPAHandler_FallsBackForNestedUnknownPath(t *testing.T) {
	h := spaHandler(testSPAFS())
	resp := doSPARequest(t, h, "/accounts/123/edit")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body := readBody(t, resp); !strings.Contains(body, "shell") {
		t.Errorf("expected shell body for nested fallback, got %q", body)
	}
}

func TestSPAHandler_RejectsPathTraversal(t *testing.T) {
	h := spaHandler(testSPAFS())
	resp := doSPARequest(t, h, "/../etc/passwd")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
