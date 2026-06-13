# SPA Static Serving in `kea serve` — Design

**Date:** 2026-06-13
**Status:** Approved (verbal) — pending spec review

## Problem

`kea serve` ([cmd/serve.go](../../cmd/serve.go)) currently runs a JSON API at `/api/*` ([internal/api/router.go](../../internal/api/router.go)) but does not serve the React SPA in [spa/](../../spa/). To use the web UI today, a developer must run `npm run dev` separately, which proxies `/api` to `:8080`. A single `kea serve` binary should be enough to run the full app for end users.

Out of scope: SPA-side auth, CSRF, gzip/brotli pre-compression, CORS adjustments, dev-mode unification.

## Goals

1. `kea serve` serves the SPA from the same origin as the API.
2. Deep links (e.g. `/accounts`, `/reports/balance-sheet`) work on refresh — non-asset, non-`/api/*` paths fall back to `index.html`.
3. The Go binary is self-contained: assets are embedded via `//go:embed`.
4. `go build` succeeds even when `spa/dist` has not been built locally.
5. Static-asset caching is correct: hashed asset files are cached aggressively; the SPA shell is not.

## Design

### Asset packaging

A new package `internal/web/` holds the embedded SPA dist.

```go
// internal/web/embed.go
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
        panic(err) // embed path is a compile-time constant
    }
    return sub
}
```

The `all:` prefix ensures dotfiles and underscore-prefixed files are included — Vite can emit such names for chunks/assets.

### Placeholder so `go build` always works

`//go:embed` fails the compile if the directory is empty. We commit a tiny placeholder:

- `spa/dist/index.html`: minimal HTML that displays "SPA not built — run `make spa-build`".
- `.gitignore` keeps ignoring `spa/dist/` but unignores the placeholder:

```
spa/dist/
!spa/dist/index.html
```

`vite build` overwrites this `index.html` and writes `assets/*` locally. The placeholder is restored on `make spa-clean`. Bare `make build` always compiles; `make build-all` runs `spa-build` first for releases.

### Router wiring

In [internal/api/router.go](../../internal/api/router.go), after the existing `r.Route("/api", ...)` block:

```go
r.Handle("/*", spaHandler(web.FS()))
```

Chi matches more-specific routes first, so `/api/*` continues to win — no risk of the SPA handler shadowing API routes. Existing middleware (recoverer, request ID, logger, access log, CORS) wraps the SPA handler as well, which is what we want.

### `spaHandler`

New file `internal/api/spa.go`. Signature takes an `fs.FS` so tests can pass `fstest.MapFS`.

Algorithm per request:

1. Strip leading `/` from `r.URL.Path`. Empty path → `index.html`.
2. Reject any path containing `..` segments as a defense-in-depth check (the embedded FS shouldn't allow escape, but the rule is cheap).
3. Try `fs.Stat(fsys, path)`. If it exists and is a regular file → serve it.
4. Otherwise → serve `index.html` with status 200 (SPA fallback).

Serving rules:

- Files under `assets/` (hashed names from Vite): `Cache-Control: public, max-age=31536000, immutable`.
- All other files including `index.html` and the fallback: `Cache-Control: no-cache`.
- Content-Type is set via `mime.TypeByExtension` on the served path, falling back to `application/octet-stream`. For the fallback case, set explicitly to `text/html; charset=utf-8`.
- Use `http.ServeContent` with the embedded file's bytes and a zero `time.Time{}` as ModTime. This disables `If-Modified-Since` handling, which we don't need: hashed asset URLs change on rebuild, and `immutable` makes revalidation moot for them; the shell uses `no-cache` so the browser always re-asks.

To keep the implementation simple: read the bytes into memory and use `bytes.NewReader` with `http.ServeContent`. The dist is small (hundreds of KB to a few MB) and lives in the binary anyway.

### Makefile

```make
build-all: spa-build build

spa-clean:
	rm -rf spa/dist/assets
	git checkout spa/dist/index.html
```

`build`, `run`, `spa-build`, `spa-dev` stay as-is. CI/release uses `build-all`.

## Testing

New `internal/api/spa_test.go` using `net/http/httptest` and `testing/fstest.MapFS`. `spaHandler` accepts `fs.FS`, so each test injects its own file tree — no dependence on the real embed.

Cases:

- `GET /` → 200, body is the test FS's `index.html`, `Content-Type: text/html; charset=utf-8`, `Cache-Control: no-cache`.
- `GET /index.html` → same as `/`.
- `GET /assets/app-abc123.js` (file present in test FS) → 200, `Content-Type` starts with `application/javascript` (or `text/javascript`, depending on Go version's mime table — assert prefix), `Cache-Control` contains `immutable`.
- `GET /favicon.svg` (file present in test FS) → 200, `Content-Type: image/svg+xml`, no `immutable` header.
- `GET /accounts` (not in test FS) → 200, body equals `index.html`, `Content-Type: text/html; charset=utf-8` — SPA fallback works.
- `GET /accounts/123` (not in test FS) → same fallback.
- A request whose cleaned path contains `..` → 400 Bad Request.
- Routing precedence smoke test: mount the full router with a stub API handler, hit `/api/health` → API responds; hit `/something` → SPA handler responds.

## Acceptance

- `make build-all` produces a `kea` binary that, when run with `kea serve`, returns the built SPA at `/` and any deep-link path, and returns API responses at `/api/*`.
- `make build` (no spa-build) produces a binary that serves the placeholder HTML at `/` — useful signal in dev that the SPA is stale.
- All new tests in `internal/api/spa_test.go` pass under `go test ./...`.
- No changes to existing API responses or routes; existing API tests still pass.

## Non-goals / deferred

- SPA auth, login flow, CSRF.
- Response compression (gzip/brotli).
- HTTP/2 push or preload hints.
- Dev-mode unification (Vite dev server + Go binary).
- Selectable disk-based asset directory at runtime.
