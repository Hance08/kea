# SPA hide_decimals toggle — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users flip `display.hide_decimals` from a new `/settings` page in the SPA without editing YAML or restarting the server.

**Architecture:** Add `PATCH /api/config` that mutates `*config.Config` in place and persists via an injected save closure (so the api package stays free of viper). The SPA gets a `/settings` route with a single Switch that calls `setConfig` and invalidates the `['server-config']` query, causing every page's amount formatter to re-render.

**Tech Stack:** Go 1.21+, chi, viper, encoding/json. React 18, TanStack Query 5, TanStack Router, Radix UI, sonner, vitest, @testing-library/react.

---

## Design decision: injected `saveConfig`, not viper-in-api

The spec sketched calling `viper.Set` + `viper.WriteConfig` directly from the handler. On a closer look that gives the `internal/api` package a dependency on viper's global state and forces every API test to wire viper.

Instead: add `saveConfig func() error` to `*Server`. The handler mutates `cfg.Display.HideDecimals`, then calls `s.saveConfig()`. In production (`cmd/serve.go`) the closure does `viper.Set("display.hide_decimals", cfg.Display.HideDecimals)` + `viper.WriteConfig()`. In tests, the closure is a stub that records calls. The leak-prevention test lives in `cmd/`, where viper already lives. This matches the existing `switchLedger` injection pattern.

The handler still goes through `viper.WriteConfig` in production, so the spec's "try viper first, fall back to direct YAML marshal if it leaks server defaults" plan still applies — the leak test just lives in `cmd/` instead of `internal/api`.

---

## File map

**Backend:**
- Modify: `internal/api/server.go` — add `saveConfig` field + constructor arg.
- Modify: `internal/api/router.go` — register `PATCH /api/config`.
- Modify: `internal/api/config.go` — add `handlePatchConfig` + request types.
- Modify: `internal/api/testhelper_test.go` — give test helpers a default no-op `saveConfig`, plus a variant that records calls.
- Modify: `internal/api/config_test.go` — add PATCH tests.
- Modify: `cmd/serve.go` — wire the production `saveConfig` closure.
- Create: `cmd/save_config_test.go` — verify viper write doesn't leak server defaults into a minimal user YAML.

**Frontend:**
- Modify: `spa/package.json` — add `@radix-ui/react-switch`.
- Create: `spa/src/components/ui/switch.tsx` — shadcn-style Switch.
- Modify: `spa/src/lib/api.ts` — add `setConfig`.
- Create: `spa/src/routes/settings.tsx` — Settings page.
- Modify: `spa/src/components/Sidebar.tsx` — bottom-pinned Settings link.
- Modify: `spa/src/test/test-app.tsx` — extend `withServerConfig` to accept an override.
- Create: `spa/src/test/api.setConfig.test.ts` — unit test.
- Create: `spa/src/test/settings.test.tsx` — integration test.

---

## Task 1: Refactor `Server` to accept an injected `saveConfig`

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/testhelper_test.go:71-115` (the `newServerForWriteWithCurrency` and `newServerForWriteWithDisplay` helpers)
- Modify: `internal/api/testhelper_test.go:151-197` (the `newTestServerWithLedger` helper)
- Modify: `cmd/serve.go:37-45`

### Step 1: Add `saveConfig` to `Server` struct

- [ ] Edit `internal/api/server.go`: add field and constructor arg.

```go
// in Server struct, after switchLedger:
saveConfig func() error
```

```go
// in NewServer signature, after switchLedger arg:
saveConfig func() error,
```

```go
// in NewServer body, after switchLedger assignment:
saveConfig: saveConfig,
```

The handler doesn't exist yet — the field will be unused this commit. That's fine; it gets used in Task 3.

### Step 2: Update call site in `cmd/serve.go`

- [ ] Edit `cmd/serve.go:37-45`. Pass a no-op closure for now (the real viper logic comes in Task 2):

```go
srv := api.NewServer(
    application.Config(),
    application.Service,
    application.Registry,
    migrationFS,
    appDir,
    application.SwitchLedger,
    func() error { return nil },
    logger,
)
```

### Step 3: Update all test helpers

- [ ] Edit `internal/api/testhelper_test.go`. Every `NewServer(...)` call gets a `func() error { return nil }` between `switchLedger` and `discardLogger()`.

Lines to update (search for `NewServer(`):
- `newServerForWriteWithCurrency` around line 84
- `newServerForWriteWithDisplay` around line 109
- `newTestServerWithLedger` around line 192

Example diff for one of them:

```go
// before
srv := NewServer(cfg, svc, nil, nil, "", nil, discardLogger())
// after
srv := NewServer(cfg, svc, nil, nil, "", nil, func() error { return nil }, discardLogger())
```

### Step 4: Build + run tests

- [ ] Run: `go build ./... && go test ./internal/api/... ./cmd/...`
- [ ] Expected: PASS (no behavior change yet).

### Step 5: Commit

- [ ] 
```bash
git add internal/api/server.go internal/api/testhelper_test.go cmd/serve.go
git commit -m "refactor(api): inject saveConfig closure into Server"
```

---

## Task 2: Wire viper-backed `saveConfig` + prove no leaks

**Files:**
- Modify: `cmd/serve.go`
- Create: `cmd/save_config_test.go`

### Step 1: Write the failing leak-prevention test

- [ ] Create `cmd/save_config_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestSaveDisplayHideDecimals_NoLeak verifies that writing the
// display.hide_decimals flag back to disk does NOT leak viper-registered
// server defaults (server.host, server.port, server.cors_origins, etc.)
// into the user's config file.
func TestSaveDisplayHideDecimals_NoLeak(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	initial := "defaults:\n  currency: TWD\ndisplay:\n  hide_decimals: true\n"
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write initial yaml: %v", err)
	}

	viper.SetConfigFile(cfgPath)
	setServerDefaults(viper.GetViper())
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("ReadInConfig: %v", err)
	}

	viper.Set("display.hide_decimals", false)
	if err := viper.WriteConfig(); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read written yaml: %v", err)
	}
	written := string(raw)

	for _, leaked := range []string{"server", "host:", "port:", "cors_origins", "database"} {
		if strings.Contains(written, leaked) {
			t.Errorf("written yaml contains leaked key %q.\nFull contents:\n%s", leaked, written)
		}
	}

	if !strings.Contains(written, "hide_decimals: false") {
		t.Errorf("written yaml missing updated hide_decimals=false.\nFull contents:\n%s", written)
	}
	if !strings.Contains(written, "currency: TWD") {
		t.Errorf("written yaml dropped existing currency.\nFull contents:\n%s", written)
	}
}
```

### Step 2: Run the test to see whether it passes or fails

- [ ] Run: `go test ./cmd/ -run TestSaveDisplayHideDecimals_NoLeak -v`
- [ ] Two outcomes:
  - **PASS** → viper doesn't leak; the simple closure in Step 3 works. Proceed.
  - **FAIL** → viper writes server.* into the file. Stop here and switch to the fallback in Step 3b below.

### Step 3 (path A — viper write is clean): Wire production `saveConfig` closure

- [ ] Edit `cmd/serve.go`. Replace the `func() error { return nil }` placeholder from Task 1 with a viper-backed closure that snapshots the current `*config.Config`:

```go
srv := api.NewServer(
    application.Config(),
    application.Service,
    application.Registry,
    migrationFS,
    appDir,
    application.SwitchLedger,
    func() error {
        cfg := application.Config()
        viper.Set("display.hide_decimals", cfg.Display.HideDecimals)
        if err := viper.WriteConfig(); err != nil {
            return fmt.Errorf("failed to persist config: %w", err)
        }
        return nil
    },
    logger,
)
```

Add imports as needed (`fmt`, `github.com/spf13/viper`).

### Step 3b (path B — viper write leaks): switch to direct YAML marshal

Only if Step 2 failed. The closure re-marshals the persisted subset and writes the file directly:

```go
func() error {
    cfg := application.Config()
    persisted := struct {
        Defaults struct {
            Currency string `yaml:"currency"`
        } `yaml:"defaults"`
        Display struct {
            HideDecimals bool `yaml:"hide_decimals"`
        } `yaml:"display"`
    }{}
    persisted.Defaults.Currency = cfg.Defaults.Currency
    persisted.Display.HideDecimals = cfg.Display.HideDecimals

    data, err := yaml.Marshal(persisted)
    if err != nil {
        return fmt.Errorf("marshal config: %w", err)
    }
    if err := os.WriteFile(cfg.ConfigPath, data, 0o600); err != nil {
        return fmt.Errorf("write config: %w", err)
    }
    return nil
}
```

Add `gopkg.in/yaml.v3` to go.mod (already used elsewhere? check first with `go mod why gopkg.in/yaml.v3`; if not, run `go get gopkg.in/yaml.v3`).

Also adjust the leak test from Step 1: it currently asserts on the viper-written file, but if we go path B the test should call the closure (or call into `viper.WriteConfig` to confirm it leaks AND then assert path-B behavior). Simplest: keep the existing test asserting on viper-leak behavior renamed to `TestViperWriteLeaks_NeedsFallback`, and add a new test that calls the path-B closure directly and asserts no leak.

### Step 4: Run all tests

- [ ] Run: `go test ./cmd/...`
- [ ] Expected: PASS.

### Step 5: Commit

- [ ] 
```bash
git add cmd/serve.go cmd/save_config_test.go go.mod go.sum
git commit -m "feat(cmd): persist display.hide_decimals via injected saveConfig"
```

(If you took path B, also include the yaml dependency change in the same commit.)

---

## Task 3: Implement `PATCH /api/config` handler

**Files:**
- Modify: `internal/api/config.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/testhelper_test.go` (add a helper that wires a recording saveConfig)
- Modify: `internal/api/config_test.go`

### Step 1: Write failing tests

- [ ] Edit `internal/api/testhelper_test.go`. Add a new helper that returns the server, the cfg pointer (so tests can assert in-memory mutation), and a slice that records each `saveConfig` call. Place it next to `newServerForWriteWithDisplay`:

```go
// newServerForPatchConfig builds a server whose saveConfig closure records
// every call into the returned slice and returns the slice pointer plus the
// *config.Config so tests can assert in-memory mutation independently of
// persistence. The saveConfig stub returns nil unless tests inject an error
// via *saveErr.
func newServerForPatchConfig(t *testing.T, currency string, hideDecimals bool) (
    ts *httptest.Server,
    cfg *config.Config,
    saveCalls *int,
    saveErr *error,
) {
    t.Helper()

    dbPath := filepath.Join(t.TempDir(), "test.db")
    st, err := store.NewStore(dbPath, migrations.FS)
    if err != nil {
        t.Fatalf("NewStore: %v", err)
    }
    t.Cleanup(func() { _ = st.Close() })

    cfg = config.NewDefault()
    cfg.Defaults.Currency = currency
    cfg.Display.HideDecimals = hideDecimals

    svc := service.NewService(st, st, st, cfg)

    calls := 0
    var injErr error
    save := func() error {
        calls++
        return injErr
    }
    srv := NewServer(cfg, svc, nil, nil, "", nil, save, discardLogger())
    ts = httptest.NewServer(srv.routes())
    t.Cleanup(ts.Close)
    return ts, cfg, &calls, &injErr
}
```

- [ ] Edit `internal/api/config_test.go`. Append PATCH tests:

```go
func TestPatchConfig(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantStatus   int
		wantHide     bool // expected cfg.Display.HideDecimals after the request
		wantSaveCalls int
	}{
		{
			name:          "set_true",
			body:          `{"display":{"hide_decimals":true}}`,
			wantStatus:    http.StatusOK,
			wantHide:      true,
			wantSaveCalls: 1,
		},
		{
			name:          "set_false",
			body:          `{"display":{"hide_decimals":false}}`,
			wantStatus:    http.StatusOK,
			wantHide:      false,
			wantSaveCalls: 1,
		},
		{
			name:          "malformed_json",
			body:          `not json`,
			wantStatus:    http.StatusBadRequest,
			wantHide:      false,
			wantSaveCalls: 0,
		},
		{
			name:          "unknown_top_field",
			body:          `{"defaults":{"currency":"EUR"}}`,
			wantStatus:    http.StatusBadRequest,
			wantHide:      false,
			wantSaveCalls: 0,
		},
		{
			name:          "empty_body",
			body:          `{}`,
			wantStatus:    http.StatusBadRequest,
			wantHide:      false,
			wantSaveCalls: 0,
		},
		{
			name:          "empty_display",
			body:          `{"display":{}}`,
			wantStatus:    http.StatusBadRequest,
			wantHide:      false,
			wantSaveCalls: 0,
		},
		{
			name:          "wrong_type",
			body:          `{"display":{"hide_decimals":"yes"}}`,
			wantStatus:    http.StatusBadRequest,
			wantHide:      false,
			wantSaveCalls: 0,
		},
		{
			name:          "unknown_display_field",
			body:          `{"display":{"foo":true}}`,
			wantStatus:    http.StatusBadRequest,
			wantHide:      false,
			wantSaveCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, cfg, saveCalls, _ := newServerForPatchConfig(t, "USD", false)

			req, err := http.NewRequest(http.MethodPatch, ts.URL+"/api/config", strings.NewReader(tt.body))
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("PATCH: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status: got %d, want %d. body=%s", resp.StatusCode, tt.wantStatus, body)
			}
			if cfg.Display.HideDecimals != tt.wantHide {
				t.Errorf("cfg.Display.HideDecimals: got %v, want %v", cfg.Display.HideDecimals, tt.wantHide)
			}
			if *saveCalls != tt.wantSaveCalls {
				t.Errorf("saveCalls: got %d, want %d", *saveCalls, tt.wantSaveCalls)
			}
		})
	}
}

func TestPatchConfig_DoesNotTouchOtherFields(t *testing.T) {
	ts, cfg, _, _ := newServerForPatchConfig(t, "TWD", false)
	originalCurrency := cfg.Defaults.Currency
	originalHost := cfg.Server.Host
	originalPort := cfg.Server.Port

	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/config",
		strings.NewReader(`{"display":{"hide_decimals":true}}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer resp.Body.Close()

	if cfg.Defaults.Currency != originalCurrency {
		t.Errorf("Defaults.Currency changed: got %q, want %q", cfg.Defaults.Currency, originalCurrency)
	}
	if cfg.Server.Host != originalHost {
		t.Errorf("Server.Host changed: got %q, want %q", cfg.Server.Host, originalHost)
	}
	if cfg.Server.Port != originalPort {
		t.Errorf("Server.Port changed: got %d, want %d", cfg.Server.Port, originalPort)
	}
}

func TestPatchConfig_RoundTrip(t *testing.T) {
	ts, _, _, _ := newServerForPatchConfig(t, "USD", false)

	patch := func(value bool) {
		body := fmt.Sprintf(`{"display":{"hide_decimals":%t}}`, value)
		req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PATCH: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PATCH status: %d", resp.StatusCode)
		}
	}

	getHide := func() bool {
		resp, err := http.Get(ts.URL + "/api/config")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		var parsed struct {
			Display struct {
				HideDecimals bool `json:"hide_decimals"`
			} `json:"display"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return parsed.Display.HideDecimals
	}

	patch(true)
	if got := getHide(); got != true {
		t.Errorf("after PATCH true: GET returned %v", got)
	}
	patch(false)
	if got := getHide(); got != false {
		t.Errorf("after PATCH false: GET returned %v", got)
	}
}

func TestPatchConfig_SaveErrorReturns500(t *testing.T) {
	ts, _, _, saveErr := newServerForPatchConfig(t, "USD", false)
	*saveErr = errors.New("disk full")

	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/config",
		strings.NewReader(`{"display":{"hide_decimals":true}}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}
```

- [ ] Update imports at the top of `config_test.go` to include `errors` and `fmt`.

### Step 2: Run tests to verify they fail

- [ ] Run: `go test ./internal/api/ -run TestPatchConfig -v`
- [ ] Expected: FAIL (PATCH /api/config returns 404 because the route doesn't exist yet).

### Step 3: Add the route

- [ ] Edit `internal/api/router.go:27`. Immediately after the GET line:

```go
r.Method(http.MethodGet, "/config", apiHandler(s.handleGetConfig))
r.Method(http.MethodPatch, "/config", apiHandler(s.handlePatchConfig))
```

### Step 4: Implement the handler

- [ ] Edit `internal/api/config.go`. Append the patch request types and handler:

```go
type configPatchRequest struct {
	Display *configDisplayPatch `json:"display"`
}

type configDisplayPatch struct {
	HideDecimals *bool `json:"hide_decimals"`
}

func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) error {
	var req configPatchRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return &service.ValidationError{Field: "body", Message: "invalid JSON: " + err.Error()}
	}
	if req.Display == nil || req.Display.HideDecimals == nil {
		return &service.ValidationError{Field: "display.hide_decimals", Message: "required"}
	}

	cfg := s.svc.Config()
	cfg.Display.HideDecimals = *req.Display.HideDecimals

	if err := s.saveConfig(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return writeJSON(w, http.StatusOK, configResponse{
		Defaults: configDefaults{Currency: cfg.Defaults.Currency},
		Display:  configDisplay{HideDecimals: cfg.Display.HideDecimals},
	})
}
```

- [ ] Update the imports at the top of `config.go`:

```go
import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hance08/kea/internal/service"
)
```

(`service` is the existing validation-error type used elsewhere in the api package — wrapping our 400 in it routes through `mapError` and produces the standard `{"error":"validation_failed","message":...}` body. The save error wraps with `%w` and falls through to the default 500 case in `mapError`.)

### Step 5: Run tests to verify pass

- [ ] Run: `go test ./internal/api/ -run TestPatchConfig -v`
- [ ] Expected: PASS for all eight `TestPatchConfig` subtests plus `_DoesNotTouchOtherFields`, `_RoundTrip`, and `_SaveErrorReturns500`.

- [ ] Run full api suite: `go test ./internal/api/`
- [ ] Expected: PASS (no regressions in existing GET tests).

### Step 6: Commit

- [ ] 
```bash
git add internal/api/config.go internal/api/router.go internal/api/config_test.go internal/api/testhelper_test.go
git commit -m "feat(api): add PATCH /api/config for display settings"
```

---

## Task 4: Add `@radix-ui/react-switch` + create `Switch` primitive

**Files:**
- Modify: `spa/package.json`
- Create: `spa/src/components/ui/switch.tsx`

### Step 1: Install the dependency

- [ ] Run from the `spa/` directory: `npm install @radix-ui/react-switch@^1`
- [ ] Verify `spa/package.json` now lists `@radix-ui/react-switch` in `dependencies`.

### Step 2: Create the Switch component

- [ ] Create `spa/src/components/ui/switch.tsx`:

```tsx
import * as SwitchPrimitives from '@radix-ui/react-switch';
import * as React from 'react';
import { cn } from '@/lib/cn';

export const Switch = React.forwardRef<
  React.ElementRef<typeof SwitchPrimitives.Root>,
  React.ComponentPropsWithoutRef<typeof SwitchPrimitives.Root>
>(({ className, ...props }, ref) => (
  <SwitchPrimitives.Root
    className={cn(
      'peer inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors',
      'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
      'disabled:cursor-not-allowed disabled:opacity-50',
      'data-[state=checked]:bg-primary data-[state=unchecked]:bg-input',
      className,
    )}
    {...props}
    ref={ref}
  >
    <SwitchPrimitives.Thumb
      className={cn(
        'pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform',
        'data-[state=checked]:translate-x-4 data-[state=unchecked]:translate-x-0',
      )}
    />
  </SwitchPrimitives.Root>
));
Switch.displayName = 'Switch';
```

### Step 3: Verify it builds

- [ ] Run from `spa/`: `npm run build` (or at minimum `npx tsc -b`).
- [ ] Expected: success.

### Step 4: Commit

- [ ] 
```bash
git add spa/package.json spa/package-lock.json spa/src/components/ui/switch.tsx
git commit -m "feat(spa): add Switch UI primitive (Radix)"
```

---

## Task 5: Add `setConfig` API client function

**Files:**
- Modify: `spa/src/lib/api.ts`
- Create: `spa/src/test/api.setConfig.test.ts`

### Step 1: Write failing test

- [ ] Create `spa/src/test/api.setConfig.test.ts`:

```ts
import { ApiError, setConfig } from '@/lib/api';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const errorResponse = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });

let fetchSpy: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchSpy = vi.fn();
  vi.stubGlobal('fetch', fetchSpy);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('sends PATCH /api/config with the supplied body', async () => {
  fetchSpy.mockResolvedValueOnce(
    okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: true } }),
  );

  const result = await setConfig({ display: { hide_decimals: true } });

  expect(fetchSpy).toHaveBeenCalledTimes(1);
  const [url, init] = fetchSpy.mock.calls[0];
  expect(url).toBe('/api/config');
  expect(init?.method).toBe('PATCH');
  expect(init?.headers).toEqual({ 'Content-Type': 'application/json' });
  expect(init?.body).toBe('{"display":{"hide_decimals":true}}');
  expect(result).toEqual({ defaults: { currency: 'USD' }, display: { hide_decimals: true } });
});

test('rejects with ApiError when the server returns 400', async () => {
  fetchSpy.mockResolvedValueOnce(
    errorResponse(400, { error: 'validation_failed', message: 'bad body', field: 'display.hide_decimals' }),
  );

  await expect(setConfig({ display: { hide_decimals: true } })).rejects.toMatchObject({
    name: 'ApiError',
    status: 400,
    message: 'bad body',
    field: 'display.hide_decimals',
  });
  expect(setConfig({ display: { hide_decimals: true } })).rejects.toBeInstanceOf(ApiError);
});
```

### Step 2: Run to verify failure

- [ ] Run from `spa/`: `npx vitest run src/test/api.setConfig.test.ts`
- [ ] Expected: FAIL with "setConfig is not exported from @/lib/api" or similar.

### Step 3: Implement `setConfig`

- [ ] Edit `spa/src/lib/api.ts`. Append after the existing `getConfig` (line 50):

```ts
export interface ConfigPatch {
  display?: { hide_decimals?: boolean };
}

export function setConfig(patch: ConfigPatch): Promise<ServerConfig> {
  return apiFetch<ServerConfig>('/api/config', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  });
}
```

### Step 4: Run test to verify pass

- [ ] Run from `spa/`: `npx vitest run src/test/api.setConfig.test.ts`
- [ ] Expected: PASS (both tests).

### Step 5: Commit

- [ ] 
```bash
git add spa/src/lib/api.ts spa/src/test/api.setConfig.test.ts
git commit -m "feat(spa): add setConfig API client"
```

---

## Task 6: Add `/settings` route with the hide_decimals toggle

**Files:**
- Modify: `spa/src/test/test-app.tsx` — extend `withServerConfig` (optional but cleaner)
- Create: `spa/src/routes/settings.tsx`
- Create: `spa/src/test/settings.test.tsx`

### Step 1: Extend `withServerConfig` to accept overrides

- [ ] Edit `spa/src/test/test-app.tsx`. Replace lines 26-31 with a version that deep-merges an override:

```tsx
export function withServerConfig(
  children: ReactNode,
  override?: Partial<ServerConfig>,
) {
  const config: ServerConfig = {
    defaults: { currency: 'USD', ...(override?.defaults ?? {}) },
    display: { hide_decimals: false, ...(override?.display ?? {}) },
  };
  return <ServerConfigContext.Provider value={config}>{children}</ServerConfigContext.Provider>;
}
```

### Step 2: Run existing tests to confirm nothing breaks

- [ ] Run from `spa/`: `npx vitest run`
- [ ] Expected: PASS (existing call sites pass no second argument, so the default merge resolves to the previous defaults).

### Step 3: Write the failing integration test

- [ ] Create `spa/src/test/settings.test.tsx`:

```tsx
import SettingsRouteModule from '@/routes/settings';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { withServerConfig } from './test-app';

const SettingsPage = SettingsRouteModule.Route.options.component as React.FC;

let fetchSpy: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchSpy = vi.fn();
  vi.stubGlobal('fetch', fetchSpy);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderSettings(hideDecimals: boolean) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  // Pre-seed the cache so useServerConfig (used inside the page) reflects the
  // initial value, AND the wrapping Provider serves the same value.
  queryClient.setQueryData(['server-config'], {
    defaults: { currency: 'USD' },
    display: { hide_decimals: hideDecimals },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  const tree = render(
    <QueryClientProvider client={queryClient}>
      {withServerConfig(<SettingsPage />, { display: { hide_decimals: hideDecimals } })}
    </QueryClientProvider>,
  );
  return { ...tree, queryClient, invalidateSpy };
}

test('renders the hide_decimals switch with the current value', () => {
  renderSettings(false);
  const sw = screen.getByRole('switch', { name: /hide decimal places/i });
  expect(sw).toHaveAttribute('aria-checked', 'false');
});

test('clicking the switch PATCHes /api/config and invalidates server-config', async () => {
  fetchSpy.mockResolvedValueOnce(
    new Response(JSON.stringify({ defaults: { currency: 'USD' }, display: { hide_decimals: true } }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  );

  const { invalidateSpy } = renderSettings(false);

  await userEvent.click(screen.getByRole('switch', { name: /hide decimal places/i }));

  await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(1));
  const [url, init] = fetchSpy.mock.calls[0];
  expect(url).toBe('/api/config');
  expect(init?.method).toBe('PATCH');
  expect(init?.body).toBe('{"display":{"hide_decimals":true}}');

  await waitFor(() =>
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['server-config'] }),
  );
});

test('shows an error toast and does not invalidate when PATCH fails', async () => {
  fetchSpy.mockResolvedValueOnce(
    new Response(JSON.stringify({ error: 'validation_failed', message: 'bad' }), {
      status: 400,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
  const { invalidateSpy } = renderSettings(false);

  await userEvent.click(screen.getByRole('switch', { name: /hide decimal places/i }));

  await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(1));
  // No invalidate on error.
  expect(invalidateSpy).not.toHaveBeenCalledWith({ queryKey: ['server-config'] });
});
```

### Step 4: Run to verify failure

- [ ] Run from `spa/`: `npx vitest run src/test/settings.test.tsx`
- [ ] Expected: FAIL with "Cannot find module '@/routes/settings'" or similar.

### Step 5: Implement the settings route

- [ ] Create `spa/src/routes/settings.tsx`:

```tsx
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { setConfig } from '@/lib/api';
import { useServerConfig } from '@/lib/server-config';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { toast } from 'sonner';

export const Route = createFileRoute('/settings')({ component: SettingsPage });

function SettingsPage() {
  const cfg = useServerConfig();
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: (hide: boolean) => setConfig({ display: { hide_decimals: hide } }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['server-config'] });
    },
    onError: (err: Error) => {
      toast.error(`Failed to update setting: ${err.message}`);
    },
  });

  return (
    <div className="max-w-xl space-y-6">
      <h1 className="text-2xl font-semibold">Settings</h1>
      <Card>
        <CardHeader>
          <CardTitle>Display</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-between gap-4">
          <div className="space-y-1">
            <Label htmlFor="hide-decimals" className="text-sm">
              Hide decimal places in amounts
            </Label>
            <p className="text-sm text-muted-foreground">
              When on, amounts like $12.00 display as $12 across all pages.
            </p>
          </div>
          <Switch
            id="hide-decimals"
            aria-label="Hide decimal places in amounts"
            checked={cfg.display.hide_decimals}
            disabled={mutation.isPending}
            onCheckedChange={(checked) => mutation.mutate(checked)}
          />
        </CardContent>
      </Card>
    </div>
  );
}

export default Route;
```

(The `export default Route` is so the test can pull the component via `SettingsRouteModule.Route`. The TanStack Router file-based router only requires the named `Route` export; the default is just a test-import convenience.)

### Step 6: Regenerate the route tree

- [ ] Run from `spa/`: `npm run dev` once briefly (or `npx tsr generate` if that script is wired) so `routeTree.gen.ts` picks up `/settings`. Stop after the regeneration step.
- [ ] Verify `spa/src/routeTree.gen.ts` now references `SettingsRoute` / `/settings`.

(If the project relies on `vite-plugin-router` auto-generating on dev/build, running `npm run build` from `spa/` is sufficient.)

### Step 7: Run the test to verify pass

- [ ] Run from `spa/`: `npx vitest run src/test/settings.test.tsx`
- [ ] Expected: PASS (all three tests).

### Step 8: Run the full SPA suite

- [ ] Run from `spa/`: `npx vitest run`
- [ ] Expected: PASS.

### Step 9: Commit

- [ ] 
```bash
git add spa/src/routes/settings.tsx spa/src/test/settings.test.tsx spa/src/test/test-app.tsx spa/src/routeTree.gen.ts
git commit -m "feat(spa): add /settings route with hide_decimals toggle"
```

---

## Task 7: Add a Settings link to the sidebar (pinned to bottom)

**Files:**
- Modify: `spa/src/components/Sidebar.tsx`

### Step 1: Update Sidebar layout

- [ ] Edit `spa/src/components/Sidebar.tsx`. Replace the file with:

```tsx
import { LedgerSwitcher } from '@/components/LedgerSwitcher';
import { cn } from '@/lib/cn';
import { Link, useRouterState } from '@tanstack/react-router';

interface NavItem {
  label: string;
  to?: string;
  prefix?: boolean; // when true, isActive matches any pathname starting with `to`
}

const NAV: NavItem[] = [
  { label: 'Balances', to: '/balances' },
  { label: 'Accounts', to: '/accounts' },
  { label: 'Transactions', to: '/transactions' },
  { label: 'Reports', to: '/reports', prefix: true },
  { label: 'Reconcile' },
];

const FOOTER_NAV: NavItem[] = [{ label: 'Settings', to: '/settings' }];

export function Sidebar() {
  const { location } = useRouterState();
  return (
    <nav
      aria-label="Main navigation"
      className="flex w-56 shrink-0 flex-col border-r bg-muted/30 p-4"
    >
      <LedgerSwitcher />
      <NavList items={NAV} pathname={location.pathname} className="flex-1" />
      <NavList items={FOOTER_NAV} pathname={location.pathname} />
    </nav>
  );
}

function NavList({
  items,
  pathname,
  className,
}: {
  items: NavItem[];
  pathname: string;
  className?: string;
}) {
  return (
    <ul className={cn('space-y-1', className)}>
      {items.map((item) => {
        if (!item.to) {
          return (
            <li key={item.label}>
              <span
                aria-disabled="true"
                className="block cursor-not-allowed rounded px-3 py-2 text-sm text-muted-foreground"
                title="Coming soon"
              >
                {item.label}
              </span>
            </li>
          );
        }
        const isActive = item.prefix
          ? pathname === item.to || pathname.startsWith(`${item.to}/`)
          : pathname === item.to;
        return (
          <li key={item.label}>
            <Link
              to={item.to}
              className={cn(
                'block rounded px-3 py-2 text-sm transition-colors',
                isActive ? 'bg-primary text-primary-foreground font-medium' : 'hover:bg-muted',
              )}
            >
              {item.label}
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
```

The `flex-1` on the primary list pushes the footer list to the bottom of the sidebar.

### Step 2: Run SPA tests + build to confirm no regressions

- [ ] Run from `spa/`: `npx vitest run && npm run build`
- [ ] Expected: PASS.

### Step 3: Manual smoke check

- [ ] Start the backend (`make run` from repo root) and the dev SPA (`npm run dev` from `spa/`).
- [ ] Open the SPA in the browser.
- [ ] Confirm the Settings link is pinned to the bottom of the sidebar.
- [ ] Click Settings; toggle the switch; verify amounts on `/balances` and `/transactions` change precision without a reload.

### Step 4: Commit

- [ ] 
```bash
git add spa/src/components/Sidebar.tsx
git commit -m "feat(spa): add Settings link to sidebar bottom"
```

---

## Self-review

**Spec coverage:**
- `PATCH /api/config` with body validation + unknown-field rejection → Task 3 (handler) + Task 3 tests (`unknown_top_field`, `unknown_display_field`, `wrong_type`, `empty_body`, `empty_display`, `malformed_json`).
- Mutates only `display.hide_decimals` → Task 3 `TestPatchConfig_DoesNotTouchOtherFields`.
- In-memory cfg update visible to subsequent GETs → Task 3 `TestPatchConfig_RoundTrip`.
- Viper write doesn't leak server defaults → Task 2 `TestSaveDisplayHideDecimals_NoLeak`, with path B fallback documented.
- `/settings` route with single toggle using existing UI primitives → Task 6.
- `setConfig` in api.ts wrapping PATCH → Task 5.
- Invalidate `['server-config']` on success → Task 6 settings page + integration test.
- Sidebar Settings link in existing nav style → Task 7.
- Unit test for setConfig with mocked fetch → Task 5.
- Integration test for the settings page (click + assert PATCH body + cache invalidation) → Task 6.
- `withServerConfig` extended for override → Task 6 Step 1.

**Placeholder scan:** No "TBD" / "TODO" / "fill in details". The viper-vs-fallback decision is the one place with a branch, and both branches contain concrete code rather than handwaving.

**Type consistency:**
- Handler reads `*configPatchRequest` with `Display *configDisplayPatch` and `HideDecimals *bool` — tests' bodies (`{"display":{"hide_decimals":true}}`) match.
- `saveConfig func() error` in the Server struct matches the closure signature in `cmd/serve.go` and the recording stub in test helpers.
- `setConfig(patch: ConfigPatch)` returns `Promise<ServerConfig>` — the route's `useMutation` consumes it as `() => setConfig(...)` and the test asserts the resolved `ServerConfig` shape.
- `Switch` props: `checked`, `disabled`, `onCheckedChange`, `id`, `aria-label` — all are valid `SwitchPrimitives.Root` props.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-06-18-spa-hide-decimals-toggle.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
