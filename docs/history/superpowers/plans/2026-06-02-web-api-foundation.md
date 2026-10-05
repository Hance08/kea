# Web API Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the HTTP foundation for `kea serve` — chi router, middleware chain (request ID, logger, access log, CORS), service-error-to-HTTP mapping, JSON helpers, and `/api/health` + `/api/version` endpoints — so future endpoint specs slot in cleanly.

**Architecture:** Single Go package `internal/api/` exposing `NewServer(cfg, svc, logger) *Server` with a `Run(ctx) error` lifecycle method. `cmd/serve.go` registers a cobra subcommand that builds the server and blocks on `cmd.Context()`. All handlers use a `func(w, r) error` adapter so error mapping lives in one place. Test-driven throughout.

**Tech Stack:** Go 1.25, `github.com/go-chi/chi/v5`, stdlib `net/http`, `log/slog`, `net/http/httptest`, cobra (existing).

**Spec:** [`docs/superpowers/specs/2026-06-02-web-api-foundation-design.md`](../specs/2026-06-02-web-api-foundation-design.md)

---

## File Layout

| File | Responsibility |
|------|----------------|
| `internal/api/errors.go` | `mapError`, `writeError`, `errorBody` |
| `internal/api/errors_test.go` | Unit tests for `mapError` |
| `internal/api/handler.go` | `apiHandler` adapter, `writeJSON`, `decodeJSON` |
| `internal/api/middleware.go` | `requestIDMiddleware`, `loggerMiddleware`, `accessLogMiddleware`, `corsMiddleware`, ctx helpers |
| `internal/api/middleware_test.go` | Middleware unit tests |
| `internal/api/health.go` | `Version` var, `handleHealth`, `handleVersion` |
| `internal/api/router.go` | `(*Server).routes() http.Handler` |
| `internal/api/server.go` | `Server` struct, `NewServer`, `Run` |
| `internal/api/server_test.go` | `Run` lifecycle test (port 0 + cancellation) |
| `internal/api/router_test.go` | HTTP integration tests via `httptest.NewServer` |
| `cmd/serve.go` | `NewServeCmd(svc, cfg) *cobra.Command` |
| `cmd/serve_test.go` | Smoke test for command structure |
| `cmd/root.go` *(modify)* | `rootCmd.AddCommand(NewServeCmd(application.Service, cfg))` |
| `go.mod`, `go.sum` *(modify)* | Add `github.com/go-chi/chi/v5` |

Build order is bottom-up: error mapping → handler adapter → middleware → endpoints → router/server → cobra command → wire into root.

---

## Task 1: Add chi dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add chi via `go get`**

```bash
go get github.com/go-chi/chi/v5@v5.1.0
```

Expected: `go.mod` gets a new require line `github.com/go-chi/chi/v5 v5.1.0`; `go.sum` gets the corresponding hashes. No code changes.

- [ ] **Step 2: Verify the build still compiles**

```bash
go build ./...
```

Expected: clean exit, no output.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add go-chi/chi v5 for web API foundation"
```

---

## Task 2: Error mapping

**Files:**
- Create: `internal/api/errors.go`
- Create: `internal/api/errors_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/api/errors_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hance08/kea/internal/service"
)

func TestMapError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{"validation_error", &service.ValidationError{Field: "name", Message: "required"}, 400, "validation_failed", "name"},
		{"not_found", service.ErrNotFound, 404, "not_found", ""},
		{"not_found_wrapped", fmt.Errorf("account: %w", service.ErrNotFound), 404, "not_found", ""},
		{"already_exists", service.ErrAlreadyExists, 409, "already_exists", ""},
		{"reconciled", service.ErrReconciled, 409, "reconciled", ""},
		{"circular_parent", service.ErrCircularParent, 409, "circular_parent", ""},
		{"not_editable", service.ErrNotEditable, 403, "not_editable", ""},
		{"unknown", errors.New("boom"), 500, "internal", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := mapError(tc.err)
			if status != tc.wantStatus {
				t.Errorf("status: got %d, want %d", status, tc.wantStatus)
			}
			if body.Error != tc.wantCode {
				t.Errorf("error code: got %q, want %q", body.Error, tc.wantCode)
			}
			if body.Field != tc.wantField {
				t.Errorf("field: got %q, want %q", body.Field, tc.wantField)
			}
		})
	}
}

func TestMapErrorInternalHidesDetail(t *testing.T) {
	_, body := mapError(errors.New("database password=hunter2 leaked"))
	if body.Message != "internal server error" {
		t.Errorf("internal error must not expose detail in body; got %q", body.Message)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/...`

Expected: compile failure — `api` package does not exist yet.

- [ ] **Step 3: Create `errors.go`**

Create `internal/api/errors.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"errors"
	"net/http"

	"github.com/hance08/kea/internal/service"
)

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, body := mapError(err)
	loggerFrom(r.Context()).Error("request failed",
		"error_code", body.Error,
		"status", status,
		"err", err.Error(),
	)
	_ = writeJSON(w, status, body)
}

func mapError(err error) (int, errorBody) {
	var verr *service.ValidationError
	switch {
	case errors.As(err, &verr):
		return http.StatusBadRequest, errorBody{
			Error: "validation_failed", Message: verr.Message, Field: verr.Field,
		}
	case errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound, errorBody{Error: "not_found", Message: err.Error()}
	case errors.Is(err, service.ErrAlreadyExists):
		return http.StatusConflict, errorBody{Error: "already_exists", Message: err.Error()}
	case errors.Is(err, service.ErrReconciled):
		return http.StatusConflict, errorBody{Error: "reconciled", Message: err.Error()}
	case errors.Is(err, service.ErrCircularParent):
		return http.StatusConflict, errorBody{Error: "circular_parent", Message: err.Error()}
	case errors.Is(err, service.ErrNotEditable):
		return http.StatusForbidden, errorBody{Error: "not_editable", Message: err.Error()}
	default:
		return http.StatusInternalServerError, errorBody{Error: "internal", Message: "internal server error"}
	}
}
```

Note: `writeError` calls `loggerFrom` and `writeJSON`, which don't exist yet. The package won't compile alone — that's fine; Task 3 adds `writeJSON` and Task 4 adds `loggerFrom`. The `errors_test.go` only exercises `mapError`, which is self-contained, so the test will pass once the whole package compiles after Task 4. We'll re-run tests at the end of Task 4.

- [ ] **Step 4: Commit work-in-progress**

```bash
git add internal/api/errors.go internal/api/errors_test.go
git commit -m "feat(api): add service-error-to-HTTP mapping"
```

---

## Task 3: Handler adapter and JSON helpers

**Files:**
- Create: `internal/api/handler.go`

- [ ] **Step 1: Create `handler.go`**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"net/http"

	"github.com/hance08/kea/internal/service"
)

// apiHandler adapts a func returning error to http.Handler.
// Errors flow through writeError for centralized status mapping.
type apiHandler func(w http.ResponseWriter, r *http.Request) error

func (h apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		writeError(w, r, err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return &service.ValidationError{Field: "body", Message: "invalid JSON: " + err.Error()}
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles (will still fail on missing loggerFrom)**

Run: `go build ./internal/api/...`

Expected: failure mentioning `undefined: loggerFrom`. This is fine — Task 4 adds it.

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler.go
git commit -m "feat(api): add apiHandler adapter and JSON helpers"
```

---

## Task 4: Middleware

**Files:**
- Create: `internal/api/middleware.go`
- Create: `internal/api/middleware_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/api/middleware_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/...`

Expected: compile failure — `requestIDMiddleware`, `loggerMiddleware`, `accessLogMiddleware`, `corsMiddleware`, `requestIDFrom`, `loggerFrom` all undefined.

- [ ] **Step 3: Create `middleware.go`**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyLogger
)

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

func loggerMiddleware(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l := base.With("request_id", requestIDFrom(r.Context()))
			ctx := context.WithValue(r.Context(), ctxKeyLogger, l)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func loggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.status = http.StatusOK
		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(b)
}

func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		loggerFrom(r.Context()).Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func corsMiddleware(allowed []string) func(http.Handler) http.Handler {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := set[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run tests to verify Task 2 + Task 4 both pass**

Run: `go test ./internal/api/...`

Expected: all tests pass (errors_test + middleware_test).

- [ ] **Step 5: Commit**

```bash
git add internal/api/middleware.go internal/api/middleware_test.go
git commit -m "feat(api): add request-ID, logger, access-log, and CORS middleware"
```

---

## Task 5: Health and version endpoints

**Files:**
- Create: `internal/api/health.go`

- [ ] **Step 1: Create `health.go`**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import "net/http"

// Version is the server version string. Override at build time via:
//
//	-ldflags "-X github.com/hance08/kea/internal/api.Version=v0.1.0"
var Version = "dev"

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, http.StatusOK, map[string]string{"version": Version})
}
```

These reference `*Server`, which doesn't exist until Task 6. The package will not compile until then — Task 6 adds the integration tests that exercise these handlers.

- [ ] **Step 2: Commit work-in-progress**

```bash
git add internal/api/health.go
git commit -m "feat(api): add health and version handlers"
```

---

## Task 6: Server, router, and HTTP integration tests

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/router.go`
- Create: `internal/api/server_test.go`
- Create: `internal/api/router_test.go`

- [ ] **Step 1: Write the failing integration tests**

Create `internal/api/router_test.go`:

```go
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

```

Create `internal/api/server_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"context"
	"testing"
	"time"

	"github.com/hance08/kea/internal/config"
)

func TestRunReturnsWhenContextCancelled(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 0, // Let the kernel pick a free port.
		},
	}
	srv := NewServer(cfg, nil, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	// Give Run a moment to start the listener goroutine.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of context cancellation")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/...`

Expected: compile failure — `NewServer`, `(*Server).routes` undefined.

- [ ] **Step 3: Create `server.go`**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/service"
)

const shutdownTimeout = 5 * time.Second

type Server struct {
	cfg    *config.Config
	svc    *service.Service
	logger *slog.Logger
	http   *http.Server
}

func NewServer(cfg *config.Config, svc *service.Service, logger *slog.Logger) *Server {
	s := &Server{cfg: cfg, svc: svc, logger: logger}
	addr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	s.http = &http.Server{
		Addr:              addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("server listening", "addr", listener.Addr().String())
		if err := s.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		s.logger.Info("server shutting down")
		return s.http.Shutdown(shutdownCtx)
	}
}
```

Note: we use `net.Listen` + `Serve` instead of `ListenAndServe` so port 0 in tests resolves to a real port we can log and so listen failures surface synchronously from `Run`.

- [ ] **Step 4: Create `router.go`**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(requestIDMiddleware)
	r.Use(loggerMiddleware(s.logger))
	r.Use(accessLogMiddleware)
	r.Use(corsMiddleware(s.cfg.Server.CORSOrigins))

	r.Route("/api", func(r chi.Router) {
		r.Method(http.MethodGet, "/health", apiHandler(s.handleHealth))
		r.Method(http.MethodGet, "/version", apiHandler(s.handleVersion))
	})

	return r
}
```

- [ ] **Step 5: Run the full test suite**

Run: `go test ./internal/api/... -v`

Expected: all tests in the `api` package pass — `TestMapError`, `TestMapErrorInternalHidesDetail`, `TestRequestIDMiddleware`, `TestLoggerFromFallback`, `TestLoggerMiddlewareAttachesLogger`, `TestAccessLogMiddlewareCapturesStatus`, `TestCORSMiddleware*`, `TestHealthEndpoint`, `TestVersionEndpoint`, `TestUnknownRouteReturns404`, `TestCORSPreflightAllowed`, `TestCORSDisallowedOrigin`, `TestRunReturnsWhenContextCancelled`.

- [ ] **Step 6: Commit**

```bash
git add internal/api/server.go internal/api/router.go internal/api/server_test.go internal/api/router_test.go
git commit -m "feat(api): add Server lifecycle and chi router"
```

---

## Task 7: `kea serve` cobra command

**Files:**
- Create: `cmd/serve.go`
- Create: `cmd/serve_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/serve_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"testing"

	"github.com/hance08/kea/internal/config"
)

func TestNewServeCmdShape(t *testing.T) {
	cmd := NewServeCmd(nil, &config.Config{})
	if cmd.Use != "serve" {
		t.Errorf("Use: got %q, want %q", cmd.Use, "serve")
	}
	if cmd.Short == "" {
		t.Error("Short description must not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE must be set")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/... -run TestNewServeCmd`

Expected: compile failure — `NewServeCmd` undefined.

- [ ] **Step 3: Create `cmd/serve.go`**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/hance08/kea/internal/api"
	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/service"
)

func NewServeCmd(svc *service.Service, cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the local web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
			srv := api.NewServer(cfg, svc, logger)
			return srv.Run(cmd.Context())
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/... -run TestNewServeCmd -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/serve.go cmd/serve_test.go
git commit -m "feat(cmd): add kea serve command"
```

---

## Task 8: Wire `serve` into root and verify end-to-end

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Find the existing AddCommand block**

Open `cmd/root.go` and locate the cluster of `rootCmd.AddCommand(...)` calls (around line 145–150 at time of writing). For reference:

```go
rootCmd.AddCommand(account.NewAccountCmd(application.Service))
rootCmd.AddCommand(transaction.NewTransactionCmd(application.Service))
rootCmd.AddCommand(NewAddCmd(application.Service))
rootCmd.AddCommand(NewInfoCmd(application.Service))
rootCmd.AddCommand(NewReportCmd(application.Service))
rootCmd.AddCommand(NewReconcileCmd(application.Service))
```

- [ ] **Step 2: Add the serve registration**

Add the following line at the end of that block:

```go
rootCmd.AddCommand(NewServeCmd(application.Service, cfg))
```

- [ ] **Step 3: Run the full test suite**

Run: `go test ./...`

Expected: all packages pass, no failures.

- [ ] **Step 4: Build and smoke-test `kea serve`**

Run:

```bash
go build -o /tmp/kea ./cmd/kea
/tmp/kea serve &
SERVE_PID=$!
sleep 1
curl -sS -o /tmp/health.json -w "HTTP %{http_code}\n" http://localhost:8080/api/health
curl -sS -o /tmp/version.json -w "HTTP %{http_code}\n" http://localhost:8080/api/version
cat /tmp/health.json; echo
cat /tmp/version.json; echo
kill -INT $SERVE_PID
wait $SERVE_PID 2>/dev/null
```

Expected output:

```
HTTP 200
HTTP 200
{"status":"ok"}
{"version":"dev"}
```

And the server should log a "server shutting down" line before exiting cleanly.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go
git commit -m "feat(cmd): register serve subcommand in root"
```

- [ ] **Step 6: Final verification**

Run:

```bash
go test ./... && go vet ./...
```

Expected: clean exit on both. No new lint warnings.

---

## Acceptance Criteria

- [ ] `kea serve` starts a server on `cfg.Server.Host:cfg.Server.Port` (default `localhost:8080`).
- [ ] `GET /api/health` → 200 `{"status":"ok"}`.
- [ ] `GET /api/version` → 200 `{"version":"dev"}` (or ldflags override).
- [ ] Every response carries an `X-Request-ID` header.
- [ ] CORS preflight from `http://localhost:5173` is allowed; from other origins, no CORS headers are set.
- [ ] SIGINT and SIGTERM cause graceful shutdown within 5 seconds.
- [ ] Service sentinels map to their documented status codes (`mapError` table-driven test passes).
- [ ] Internal errors do not leak detail into response bodies (only into logs).
- [ ] `go test ./...` is clean.

---

## Out of Scope (Deferred to Future Specs)

- Domain endpoints (accounts, transactions, reports, reconcile).
- React SPA architecture and embedding.
- Live ledger-switch propagation to a running server (#119).
- Authentication, TLS, metrics, rate limiting.
