# SPA Static Serving Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `kea serve` deliver the built React SPA from the same origin as the API, with proper SPA-fallback routing and cache headers, while keeping `go build` working when `spa/dist` has not been built locally.

**Architecture:** A new `internal/web` package embeds `spa/dist` via `//go:embed`. The chi router in `internal/api` mounts a new `spaHandler(fs.FS)` at `/*` after the existing `/api/*` block; chi's most-specific-wins matching keeps the API surface untouched. A committed placeholder `spa/dist/index.html` lets `//go:embed` succeed before a real Vite build runs.

**Tech Stack:** Go 1.22+ (`http.ServeContent`, `fs.Sub`), `embed`, `io/fs`, `testing/fstest`, chi v5, existing service+store layers (untouched).

**Spec:** [2026-06-13-spa-static-serving-design.md](2026-06-13-spa-static-serving-design.md)

---

## File Structure

**Create:**
- `internal/web/embed.go` — embeds `spa/dist` and exposes `FS() fs.FS`.
- `internal/api/spa.go` — `spaHandler(fsys fs.FS) http.Handler` with SPA fallback and cache headers.
- `internal/api/spa_test.go` — unit tests for `spaHandler` against `fstest.MapFS`.
- `spa/dist/index.html` — committed placeholder so `//go:embed` always finds at least one file.

**Modify:**
- `.gitignore` — keep ignoring `spa/dist/` but unignore the placeholder.
- `Makefile` — add `build-all` and `spa-clean` targets.
- `internal/api/router.go` — mount `spaHandler` at `/*` and import `internal/web`.
- `internal/api/router_test.go` — assertion that `/api/health` still wins over the SPA route.

---

## Task 1: Commit the SPA dist placeholder and unignore it

**Files:**
- Create: `spa/dist/index.html`
- Modify: `.gitignore`

- [ ] **Step 1: Write the placeholder index.html**

Create `spa/dist/index.html` with this exact content:

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <title>kea — SPA not built</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <style>
      body { font-family: system-ui, sans-serif; max-width: 40rem; margin: 4rem auto; padding: 0 1rem; color: #222; }
      code { background: #f4f4f4; padding: 0.15rem 0.35rem; border-radius: 3px; }
    </style>
  </head>
  <body>
    <h1>kea — SPA not built</h1>
    <p>This is the placeholder shell that ships with the Go binary when the SPA has not been built.</p>
    <p>Run <code>make spa-build</code> (or <code>make build-all</code>) to produce the real SPA bundle.</p>
  </body>
</html>
```

- [ ] **Step 2: Verify the file is currently ignored**

Run: `git check-ignore -v spa/dist/index.html`

Expected: a line like `.gitignore:56:spa/dist/    spa/dist/index.html` — confirms it is ignored before the change.

- [ ] **Step 3: Update .gitignore to unignore the placeholder**

In `.gitignore`, find the existing block:

```
# SPA workspace
spa/node_modules/
spa/dist/
spa/.env
spa/.env.*
!spa/.env.example
```

Change it to:

```
# SPA workspace
spa/node_modules/
spa/dist/
!spa/dist/index.html
spa/.env
spa/.env.*
!spa/.env.example
```

- [ ] **Step 4: Verify the file is now tracked-eligible**

Run: `git check-ignore -v spa/dist/index.html; echo exit=$?`

Expected: `exit=1` (no ignore match — the `!` rule wins).

- [ ] **Step 5: Stage and commit**

```bash
git add .gitignore spa/dist/index.html
git commit -m "chore(spa): commit placeholder dist/index.html for go:embed"
```

---

## Task 2: Add the embedded asset package

**Files:**
- Create: `internal/web/embed.go`

- [ ] **Step 1: Create the package**

Write `internal/web/embed.go` with this exact content:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

// Package web embeds the built SPA bundle (spa/dist) so the Go binary can
// serve the frontend from the same origin as the API.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the SPA dist tree rooted so that index.html is at the root.
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
```

- [ ] **Step 2: Symlink dist into the package**

`//go:embed` only sees paths relative to the package directory. Create a symlink from `internal/web/dist` to `spa/dist` so the directive resolves:

```bash
ln -s ../../spa/dist internal/web/dist
```

Verify: `ls -la internal/web/dist` shows `dist -> ../../spa/dist`.

- [ ] **Step 3: Verify the package compiles**

Run: `go build ./internal/web/...`

Expected: exits 0 with no output.

- [ ] **Step 4: Sanity-check the embed at runtime**

Run:

```bash
go run -tags ignore - <<'EOF'
package main

import (
	"fmt"
	"io/fs"

	"github.com/hance08/kea/internal/web"
)

func main() {
	_ = fs.WalkDir(web.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	})
}
EOF
```

Expected output includes `.` and `index.html` at minimum.

If the inline run is awkward, skip this step — Task 3 tests exercise the same path.

- [ ] **Step 5: Commit**

```bash
git add internal/web/embed.go internal/web/dist
git commit -m "feat(web): embed spa/dist into the binary via go:embed"
```

---

## Task 3: Add the `spaHandler` (TDD)

**Files:**
- Create: `internal/api/spa.go`
- Test: `internal/api/spa_test.go`

- [ ] **Step 1: Write the failing test file**

Create `internal/api/spa_test.go` with this exact content:

```go
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

// testSPAFS is a small in-memory tree that mimics a real vite dist layout.
func testSPAFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":             {Data: []byte("<!doctype html><html><body>shell</body></html>")},
		"favicon.svg":            {Data: []byte("<svg/>")},
		"assets/app-abc123.js":   {Data: []byte("console.log('app');")},
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
		t.Errorf("Cache-Control = %q, should not be immutable for non-assets path", cc)
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
```

- [ ] **Step 2: Run tests to verify they fail with "undefined: spaHandler"**

Run: `go test ./internal/api/ -run TestSPAHandler -v`

Expected: build failure with `undefined: spaHandler` (function does not exist yet).

- [ ] **Step 3: Implement `spaHandler`**

Create `internal/api/spa.go` with this exact content:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"bytes"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// (path is used by serveFSFile for mime.TypeByExtension on the extension.)

const (
	spaIndexFile      = "index.html"
	cacheControlShell = "no-cache"
	cacheControlAsset = "public, max-age=31536000, immutable"
)

// spaHandler serves the SPA assets in fsys, falling back to index.html for any
// request whose path does not match an existing regular file. This gives the
// client-side router control of deep-link URLs while still allowing real files
// (hashed JS/CSS, favicon, etc.) to be served directly.
func spaHandler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Defense in depth: reject any request whose path contains a parent-dir
		// segment. fs.FS resolution cannot actually escape the root, but rejecting
		// outright keeps the contract simple and avoids surprising 200s on the
		// SPA fallback when a caller tries to traverse.
		if strings.Contains(r.URL.Path, "..") {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		fsPath := strings.TrimPrefix(r.URL.Path, "/")
		if fsPath == "" {
			fsPath = spaIndexFile
		}

		info, err := fs.Stat(fsys, fsPath)
		switch {
		case err == nil && info.Mode().IsRegular():
			serveFSFile(w, r, fsys, fsPath)
			return
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Not found (or not a regular file) → SPA fallback to index.html.
		serveIndex(w, r, fsys)
	})
}

func serveFSFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, fsPath string) {
	data, err := fs.ReadFile(fsys, fsPath)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if ct := mime.TypeByExtension(path.Ext(fsPath)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if strings.HasPrefix(fsPath, "assets/") {
		w.Header().Set("Cache-Control", cacheControlAsset)
	} else {
		w.Header().Set("Cache-Control", cacheControlShell)
	}

	http.ServeContent(w, r, fsPath, time.Time{}, bytes.NewReader(data))
}

func serveIndex(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	data, err := fs.ReadFile(fsys, spaIndexFile)
	if err != nil {
		http.Error(w, "spa shell missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControlShell)
	http.ServeContent(w, r, spaIndexFile, time.Time{}, bytes.NewReader(data))
}
```

- [ ] **Step 4: Run tests to verify they all pass**

Run: `go test ./internal/api/ -run TestSPAHandler -v`

Expected: 7 PASS lines (one per test function) and final `ok` line.

- [ ] **Step 5: Run the full package test suite to check for regressions**

Run: `go test ./internal/api/...`

Expected: `ok  	github.com/hance08/kea/internal/api`.

- [ ] **Step 6: Commit**

```bash
git add internal/api/spa.go internal/api/spa_test.go
git commit -m "feat(api): add spaHandler with SPA fallback and cache headers"
```

---

## Task 4: Wire `spaHandler` into the router

**Files:**
- Modify: `internal/api/router.go`
- Test: `internal/api/router_test.go`

- [ ] **Step 1: Inspect the current router test file**

Run: `head -40 internal/api/router_test.go`

Read it so the new test fits the existing style (helpers, naming).

- [ ] **Step 2: Add a router-level test asserting precedence**

Append this test to `internal/api/router_test.go`:

```go
func TestRouter_SPADoesNotShadowAPI(t *testing.T) {
	ts, _ := newServerWithStore(t)
	defer ts.Close()

	// API route still wins.
	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("api status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("api Content-Type = %q, want application/json", ct)
	}

	// Unknown non-API path falls back to the SPA shell.
	resp2, err := http.Get(ts.URL + "/accounts")
	if err != nil {
		t.Fatalf("GET /accounts: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("spa fallback status = %d, want 200", resp2.StatusCode)
	}
	if ct := resp2.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("spa fallback Content-Type = %q, want text/html", ct)
	}
}
```

If `strings` and `net/http` are not already imported in this test file, add them to the import block. Run `head -30 internal/api/router_test.go` to confirm.

- [ ] **Step 3: Run the new test and confirm it fails**

Run: `go test ./internal/api/ -run TestRouter_SPADoesNotShadowAPI -v`

Expected: `/accounts` returns 404 (chi's default Not Found handler) because `spaHandler` is not yet mounted. Test FAILs on the second assertion.

- [ ] **Step 4: Mount `spaHandler` in the router**

In `internal/api/router.go`, add the import (alphabetical) and the catch-all route. The final file should look like this:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/hance08/kea/internal/web"
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
		r.Method(http.MethodGet, "/config", apiHandler(s.handleGetConfig))
		r.Method(http.MethodGet, "/accounts", apiHandler(s.handleListAccounts))
		r.Method(http.MethodPost, "/accounts", apiHandler(s.handleCreateAccount))
		r.Method(http.MethodGet, "/accounts/tree", apiHandler(s.handleAccountTree))
		r.Method(http.MethodGet, "/accounts/by-name", apiHandler(s.handleAccountByName))
		r.Method(http.MethodGet, "/accounts/{id}", apiHandler(s.handleAccountByID))
		r.Method(http.MethodPatch, "/accounts/{id}", apiHandler(s.handleUpdateAccount))
		r.Method(http.MethodDelete, "/accounts/{id}", apiHandler(s.handleDeleteAccount))
		r.Method(http.MethodGet, "/accounts/{id}/balance", apiHandler(s.handleAccountBalance))
		r.Method(http.MethodGet, "/balances", apiHandler(s.handleListBalances))
		r.Method(http.MethodGet, "/accounts/{id}/unreconciled", apiHandler(s.handleListUnreconciled))
		r.Method(http.MethodPost, "/accounts/{id}/reconcile/preview", apiHandler(s.handleReconcilePreview))
		r.Method(http.MethodPost, "/accounts/{id}/reconcile", apiHandler(s.handleReconcileCommit))
		r.Method(http.MethodGet, "/transactions", apiHandler(s.handleListTransactions))
		r.Method(http.MethodPost, "/transactions", apiHandler(s.handleCreateTransaction))
		r.Method(http.MethodGet, "/transactions/{id}", apiHandler(s.handleTransactionByID))
		r.Method(http.MethodDelete, "/transactions/{id}", apiHandler(s.handleDeleteTransaction))
		r.Method(http.MethodPatch, "/transactions/{id}", apiHandler(s.handleUpdateTransaction))
		r.Method(http.MethodPatch, "/transactions/{id}/status", apiHandler(s.handleUpdateTransactionStatus))
		r.Method(http.MethodGet, "/reports/income-statement", apiHandler(s.handleIncomeStatement))
		r.Method(http.MethodGet, "/reports/income-breakdown", apiHandler(s.handleIncomeBreakdown))
		r.Method(http.MethodGet, "/reports/expense-breakdown", apiHandler(s.handleExpenseBreakdown))
		r.Method(http.MethodGet, "/reports/balance-sheet", apiHandler(s.handleBalanceSheet))
		r.Method(http.MethodGet, "/reports/net-worth", apiHandler(s.handleNetWorth))
		r.Method(http.MethodGet, "/ledgers/active", apiHandler(s.handleActiveLedger))
		r.Method(http.MethodGet, "/ledgers", apiHandler(s.handleListLedgers))
		r.Method(http.MethodPost, "/ledgers", apiHandler(s.handleCreateLedger))
		r.Method(http.MethodPost, "/ledgers/switch", apiHandler(s.handleSwitchLedger))
		r.Method(http.MethodDelete, "/ledgers/{name}", apiHandler(s.handleDeleteLedger))
	})

	r.Handle("/*", spaHandler(web.FS()))

	return r
}
```

- [ ] **Step 5: Run the router test and confirm it passes**

Run: `go test ./internal/api/ -run TestRouter_SPADoesNotShadowAPI -v`

Expected: PASS.

- [ ] **Step 6: Run the full test suite**

Run: `go test ./...`

Expected: every package reports `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/api/router.go internal/api/router_test.go
git commit -m "feat(api): serve SPA from / with /api/* precedence"
```

---

## Task 5: Add `build-all` and `spa-clean` Makefile targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add the targets**

Append to `Makefile`:

```make
build-all: spa-build build

spa-clean:
	rm -rf spa/dist/assets
	git checkout spa/dist/index.html
```

The full Makefile should now end with:

```make
spa-build:
	cd spa && npm run build

build-all: spa-build build

spa-clean:
	rm -rf spa/dist/assets
	git checkout spa/dist/index.html
```

- [ ] **Step 2: Verify `make build` still works against the placeholder**

Run: `make build`

Expected: produces a `kea` (or `kea_test`) binary with no errors. The binary embeds only the placeholder.

- [ ] **Step 3: Verify `make build-all` runs the SPA build then the Go build**

Run: `make build-all`

Expected: vite build output, then a Go build with no errors. After completion, `ls spa/dist/` shows both `index.html` (now the real SPA shell) and an `assets/` directory.

If `npm` is not available in the current environment, skip this step and call it out in the commit message — CI/release runs it.

- [ ] **Step 4: Verify `make spa-clean` restores the placeholder**

Run: `make spa-clean && cat spa/dist/index.html | head -5`

Expected: the placeholder content (`<!DOCTYPE html>` + `<title>kea — SPA not built</title>`) is restored, and `spa/dist/assets/` no longer exists.

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "build: add build-all and spa-clean Makefile targets"
```

---

## Task 6: End-to-end smoke check via `kea serve`

**Files:** none — manual verification.

- [ ] **Step 1: Build the binary with the real SPA**

Run: `make spa-clean && make build-all`

Expected: clean run, fresh `spa/dist/assets/` and real `index.html` produced, Go binary produced.

- [ ] **Step 2: Start the server in the background**

Run: `./kea serve &` (or `./kea_test serve &` depending on Makefile target). Capture the PID: `SERVE_PID=$!`.

Expected: server logs `server listening` on `127.0.0.1:8080` (or whichever host/port the config defaults to).

- [ ] **Step 3: Hit the API**

Run: `curl -sS -i http://127.0.0.1:8080/api/health`

Expected: `HTTP/1.1 200 OK`, `Content-Type: application/json`, body `{"status":"ok"}`.

- [ ] **Step 4: Hit the SPA root**

Run: `curl -sS -i http://127.0.0.1:8080/`

Expected: `HTTP/1.1 200 OK`, `Content-Type: text/html…`, `Cache-Control: no-cache`, body is the built SPA shell (contains `<div id="root">` or similar Vite-injected markup — not the placeholder `<h1>kea — SPA not built</h1>`).

- [ ] **Step 5: Hit a deep link**

Run: `curl -sS -i http://127.0.0.1:8080/accounts`

Expected: `HTTP/1.1 200 OK`, same shell body as Step 4 (SPA fallback).

- [ ] **Step 6: Hit a hashed asset**

Run:

```bash
ASSET=$(ls spa/dist/assets | head -1)
curl -sS -i "http://127.0.0.1:8080/assets/$ASSET" | head -10
```

Expected: `HTTP/1.1 200 OK`, `Cache-Control: public, max-age=31536000, immutable`, content-type appropriate for the file extension.

- [ ] **Step 7: Stop the server**

Run: `kill $SERVE_PID`

Expected: server logs `server shutting down` and exits cleanly.

- [ ] **Step 8: No commit**

This task verifies the integration; no code changes to commit.

---

## Self-review notes

- Spec coverage: every item in the spec maps to a task. Asset packaging → Task 2. Placeholder + gitignore → Task 1. Router wiring + cache headers + fallback → Tasks 3–4. Makefile → Task 5. Tests → Tasks 3 + 4. End-to-end check → Task 6.
- Type/name consistency: `spaHandler(fs.FS) http.Handler`, `web.FS() fs.FS`, `cacheControlAsset`/`cacheControlShell` constants are used identically wherever they appear.
- Path traversal test uses `/../etc/passwd`; matching guard in `spaHandler` rejects any cleaned path that starts with `/../` or equals `/..` → returns 400, matching the test.
