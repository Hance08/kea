# `/api/config` Endpoint and SPA Runtime Config Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `GET /api/config` returning the server's startup defaults, and have the SPA fetch it at boot so the dashboard's "default currency" matches the Go-side `cfg.Defaults.Currency` instead of drifting against a separate `VITE_DEFAULT_CURRENCY` env var.

**Architecture:** A read-only endpoint in `internal/api/` returns `{"defaults": {"currency": "..."}}` (mirroring the Go `Config` layout). The SPA fetches it once at boot via TanStack Query, gates child route rendering on success through a new `ServerConfigProvider` at the root, and exposes the value via a `useServerConfig()` hook. The previous `VITE_DEFAULT_CURRENCY` env var is removed from the codebase entirely (`.env`, `.env.example`, `vite-env.d.ts`, `README.md`, and `routes/balances.tsx`).

**Tech Stack:** Go (chi router, stdlib `httptest`, `testify`), React + TypeScript + TanStack Query + TanStack Router + Vitest + @testing-library/react.

**Spec:** [docs/superpowers/specs/2026-06-09-api-config-endpoint-design.md](../specs/2026-06-09-api-config-endpoint-design.md)

---

## File Map

**Created:**
- `internal/api/config.go` — handler + response types
- `internal/api/config_test.go` — table test for populated + empty currency
- `spa/src/lib/server-config.tsx` — `ServerConfigProvider` + `useServerConfig` hook
- `spa/src/test/server-config.test.tsx` — provider/hook unit test

**Modified:**
- `internal/api/router.go` — register `GET /api/config`
- `internal/api/testhelper_test.go` — add `newServerForWriteWithCurrency` helper
- `spa/src/lib/types.ts` — add `ServerConfig` interface
- `spa/src/lib/api.ts` — add `getConfig()`
- `spa/src/main.tsx` — wrap router with `ServerConfigProvider`
- `spa/src/routes/balances.tsx` — replace env var with `useServerConfig()`
- `spa/src/test/setup.tsx` — wrap `makeTestApp` with `ServerConfigProvider` + URL-aware fetch stub helper
- `spa/src/test/balances.test.tsx` — URL-aware fetch stub returning both `/api/config` and `/api/balances`
- `spa/README.md` — remove `VITE_DEFAULT_CURRENCY` doc section + remove "upcoming `GET /api/config`" bullet from Status

**Deleted (lines, not files):**
- `spa/.env` — remove the `VITE_DEFAULT_CURRENCY=TWD` line (delete file if it becomes empty)
- `spa/.env.example` — remove the `VITE_DEFAULT_CURRENCY=USD` line (delete file if it becomes empty)
- `spa/src/vite-env.d.ts` — remove `readonly VITE_DEFAULT_CURRENCY?: string;` line

---

## Task 1: Add `newServerForWriteWithCurrency` test helper

The existing `newServerForWrite` hardcodes `cfg.Defaults.Currency = "USD"`. The new endpoint test needs both populated and empty currency cases, so we extract a parameterized variant first. This is pure refactor: `newServerForWrite` keeps its existing signature by delegating.

**Files:**
- Modify: `internal/api/testhelper_test.go`

- [ ] **Step 1: Add the parameterized helper and refactor `newServerForWrite` to delegate**

Open `internal/api/testhelper_test.go`. Locate the existing `newServerForWrite` function (around line 72). Replace it with:

```go
// newServerForWriteWithCurrency is a variant of newServerForWrite that lets the
// caller choose cfg.Defaults.Currency. Used by /api/config tests that exercise
// both populated and empty defaults.
func newServerForWriteWithCurrency(t *testing.T, currency string) (*httptest.Server, *service.Service, *store.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(dbPath, migrations.FS)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := config.NewDefault()
	cfg.Defaults.Currency = currency

	svc := service.NewService(st, st, st, cfg)
	srv := NewServer(cfg, svc, nil, nil, "", nil, discardLogger())
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

	return ts, svc, st
}

// newServerForWrite is a variant of newServerWithStore that also returns the
// underlying *store.Store, so write tests can manipulate reconcile state and
// inject a parent cycle directly via the repo (bypassing the service layer
// where convenient).
func newServerForWrite(t *testing.T) (*httptest.Server, *service.Service, *store.Store) {
	t.Helper()
	return newServerForWriteWithCurrency(t, "USD")
}
```

- [ ] **Step 2: Verify existing tests still pass (no behavior change for `newServerForWrite`)**

Run: `go test ./internal/api/...`
Expected: PASS — all existing API tests still pass because `newServerForWrite("USD")` is the same as the old behavior.

- [ ] **Step 3: No commit yet — this helper change is part of the Go-side commit in Task 4.**

---

## Task 2: Write failing test for `GET /api/config`

TDD: write the test before the handler exists. The test should fail at the routing layer (404) because no route is registered.

**Files:**
- Create: `internal/api/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/api/config_test.go` with the SPDX header and a table-driven test:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetConfig(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		wantBody string
	}{
		{"populated", "USD", `{"defaults":{"currency":"USD"}}`},
		{"empty", "", `{"defaults":{"currency":""}}`},
		{"non_default", "TWD", `{"defaults":{"currency":"TWD"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _, _ := newServerForWriteWithCurrency(t, tt.currency)

			resp, err := http.Get(ts.URL + "/api/config")
			if err != nil {
				t.Fatalf("GET /api/config: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("status: got %d, want 200", resp.StatusCode)
			}
			if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type: got %q, want application/json", ct)
			}

			raw, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			got := strings.TrimSpace(string(raw))
			if got != tt.wantBody {
				t.Errorf("body: got %q, want %q", got, tt.wantBody)
			}

			// Also verify it round-trips into the expected struct shape.
			var parsed struct {
				Defaults struct {
					Currency string `json:"currency"`
				} `json:"defaults"`
			}
			if err := json.Unmarshal(raw, &parsed); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if parsed.Defaults.Currency != tt.currency {
				t.Errorf("parsed currency: got %q, want %q", parsed.Defaults.Currency, tt.currency)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestGetConfig -v`
Expected: FAIL — all three subtests fail with `status: got 404, want 200` (route not registered yet).

---

## Task 3: Implement `handleGetConfig` and register the route

Add the handler and wire it into the router. The two changes go together because the test cannot pass without both.

**Files:**
- Create: `internal/api/config.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create the handler**

Create `internal/api/config.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import "net/http"

type configResponse struct {
	Defaults configDefaults `json:"defaults"`
}

type configDefaults struct {
	Currency string `json:"currency"`
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, http.StatusOK, configResponse{
		Defaults: configDefaults{
			Currency: s.svc.Config().Defaults.Currency,
		},
	})
}
```

- [ ] **Step 2: Register the route**

Open `internal/api/router.go`. Inside the `r.Route("/api", ...)` block, insert this line immediately after the existing `/version` registration (around line 24):

```go
			r.Method(http.MethodGet, "/config", apiHandler(s.handleGetConfig))
```

The resulting block (lines 23-25 area) should read:

```go
			r.Method(http.MethodGet, "/health", apiHandler(s.handleHealth))
			r.Method(http.MethodGet, "/version", apiHandler(s.handleVersion))
			r.Method(http.MethodGet, "/config", apiHandler(s.handleGetConfig))
			r.Method(http.MethodGet, "/accounts", apiHandler(s.handleListAccounts))
```

- [ ] **Step 3: Run the test to verify it passes**

Run: `go test ./internal/api/ -run TestGetConfig -v`
Expected: PASS — all three subtests pass.

- [ ] **Step 4: Run the full Go suite as a regression check**

Run: `go test ./... && go build ./...`
Expected: PASS.

---

## Task 4: Commit the Go-side change

**Files:**
- Modified: `internal/api/router.go`, `internal/api/testhelper_test.go`
- Created: `internal/api/config.go`, `internal/api/config_test.go`

- [ ] **Step 1: Stage and commit**

```bash
git add internal/api/config.go internal/api/config_test.go internal/api/router.go internal/api/testhelper_test.go
git commit -m "feat(api): add GET /api/config endpoint

Returns the server's startup defaults so the SPA can read default
currency from a single source of truth instead of a separate Vite env
var. Shape mirrors the Go Config struct (defaults.currency) to leave
room for future fields without a breaking re-grouping."
```

- [ ] **Step 2: Verify commit landed**

Run: `git log --oneline -1`
Expected: shows the `feat(api): add GET /api/config endpoint` commit at HEAD.

---

## Task 5: Add `ServerConfig` type and `getConfig()` API client

Smallest possible SPA-side change: extend the types and API surface so subsequent tasks have something to import.

**Files:**
- Modify: `spa/src/lib/types.ts`
- Modify: `spa/src/lib/api.ts`

- [ ] **Step 1: Add the `ServerConfig` interface**

Open `spa/src/lib/types.ts`. Append after the existing `ListResult` interface (after line 18):

```ts
export interface ServerConfig {
  defaults: {
    currency: string;
  };
}
```

- [ ] **Step 2: Add the `getConfig` client function**

Open `spa/src/lib/api.ts`. Replace the existing import line:

```ts
import type { AccountBalance, ListResult } from './types';
```

with:

```ts
import type { AccountBalance, ListResult, ServerConfig } from './types';
```

Then append at the end of the file (after `getBalances`):

```ts
export function getConfig(): Promise<ServerConfig> {
  return apiFetch<ServerConfig>('/api/config');
}
```

- [ ] **Step 3: Verify SPA still type-checks**

Run: `cd spa && npm run check`
Expected: PASS (no Biome errors). `getConfig` is unused at this point, which is fine — Biome does not flag exported symbols as unused.

If `npm run check` reports the function as unused, that means project rules differ from expectation; in that case skip the check until Task 6 introduces the consumer, then re-run.

---

## Task 6: Write failing test for `ServerConfigProvider` / `useServerConfig`

TDD again: the test for the provider drives its API.

**Files:**
- Create: `spa/src/test/server-config.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/server-config.test.tsx`:

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { ServerConfigProvider, useServerConfig } from '@/lib/server-config';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

function CurrencyReadout() {
  const cfg = useServerConfig();
  return <div data-testid="currency">{cfg.defaults.currency}</div>;
}

function renderWithProvider() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ServerConfigProvider fallback={<div data-testid="loading">Loading...</div>}>
        {() => <CurrencyReadout />}
      </ServerConfigProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'TWD' } }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('renders fallback while /api/config is pending, then exposes config to children', async () => {
  renderWithProvider();

  // Fallback visible immediately.
  expect(screen.getByTestId('loading')).toBeInTheDocument();

  // After the query resolves, child reads currency from context.
  expect(await screen.findByTestId('currency')).toHaveTextContent('TWD');
});

test('keeps fallback visible when /api/config fails', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(new Response('{}', { status: 500 }))),
  );
  renderWithProvider();

  // Wait a microtask for the failed query to settle; fallback stays.
  await Promise.resolve();
  expect(screen.getByTestId('loading')).toBeInTheDocument();
  expect(screen.queryByTestId('currency')).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npm run test -- server-config`
Expected: FAIL — Vitest reports that `@/lib/server-config` cannot be resolved.

---

## Task 7: Implement `ServerConfigProvider` and `useServerConfig`

**Files:**
- Create: `spa/src/lib/server-config.tsx`

- [ ] **Step 1: Create the provider and hook**

Create `spa/src/lib/server-config.tsx`:

```tsx
import { useQuery } from '@tanstack/react-query';
import { createContext, type ReactNode, useContext } from 'react';
import { getConfig } from './api';
import type { ServerConfig } from './types';

const ServerConfigContext = createContext<ServerConfig | null>(null);

interface ServerConfigProviderProps {
  fallback: ReactNode;
  children: (cfg: ServerConfig) => ReactNode;
}

export function ServerConfigProvider({ fallback, children }: ServerConfigProviderProps) {
  const query = useQuery({
    queryKey: ['server-config'],
    queryFn: getConfig,
    staleTime: Number.POSITIVE_INFINITY,
  });

  if (query.isPending || query.isError) {
    return <>{fallback}</>;
  }

  return (
    <ServerConfigContext.Provider value={query.data}>
      {children(query.data)}
    </ServerConfigContext.Provider>
  );
}

export function useServerConfig(): ServerConfig {
  const cfg = useContext(ServerConfigContext);
  if (!cfg) {
    throw new Error('useServerConfig must be used inside ServerConfigProvider');
  }
  return cfg;
}
```

- [ ] **Step 2: Run the provider test to verify it passes**

Run: `cd spa && npm run test -- server-config`
Expected: PASS — both subtests pass.

- [ ] **Step 3: Run Biome to catch style issues**

Run: `cd spa && npm run check`
Expected: PASS.

---

## Task 8: Wire `ServerConfigProvider` into `main.tsx`

**Files:**
- Modify: `spa/src/main.tsx`

- [ ] **Step 1: Wrap the router**

Open `spa/src/main.tsx`. Replace the entire file contents with:

```tsx
import './styles/globals.css';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createRouter } from '@tanstack/react-router';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { ServerConfigProvider } from './lib/server-config';
import { routeTree } from './routeTree.gen';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, refetchOnWindowFocus: false },
  },
});

const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

const rootEl = document.getElementById('root');
if (!rootEl) throw new Error('root element not found');

createRoot(rootEl).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ServerConfigProvider
        fallback={
          <div className="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
            Loading…
          </div>
        }
      >
        {() => <RouterProvider router={router} />}
      </ServerConfigProvider>
    </QueryClientProvider>
  </StrictMode>,
);
```

- [ ] **Step 2: Type-check**

Run: `cd spa && npm run check`
Expected: PASS.

(The full integration is verified after Task 11 updates the balances test.)

---

## Task 9: Update `makeTestApp` helper to wrap with `ServerConfigProvider`

`makeTestApp` is consumed by `balances.test.tsx` and will be consumed by any future route tests. It needs to mirror `main.tsx`. To keep the helper flexible we also accept an optional `serverConfig` override so tests can avoid stubbing `fetch` for `/api/config` if they prefer.

**Files:**
- Modify: `spa/src/test/setup.tsx`

- [ ] **Step 1: Replace the file contents**

Open `spa/src/test/setup.tsx`. Replace the entire file with:

```tsx
import '@testing-library/jest-dom/vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
// Re-export a TestApp helper for routing-aware tests.
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';
import { ServerConfigProvider } from '../lib/server-config';
import { routeTree } from '../routeTree.gen';

afterEach(() => {
  cleanup();
});

export function makeTestApp(initialPath: string) {
  const history = createMemoryHistory({ initialEntries: [initialPath] });
  const router = createRouter({ routeTree, history });
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return (
    <QueryClientProvider client={queryClient}>
      <ServerConfigProvider fallback={<div>Loading…</div>}>
        {() => <RouterProvider router={router} />}
      </ServerConfigProvider>
    </QueryClientProvider>
  );
}
```

- [ ] **Step 2: Type-check**

Run: `cd spa && npm run check`
Expected: PASS.

---

## Task 10: Update `balances.test.tsx` fetch stub to be URL-aware

The existing stub returns the balances payload for any URL. With `ServerConfigProvider` in `makeTestApp`, the test now also issues a `/api/config` request — the stub must answer both.

**Files:**
- Modify: `spa/src/test/balances.test.tsx`

- [ ] **Step 1: Replace the `beforeEach` block**

Open `spa/src/test/balances.test.tsx`. Replace the entire `beforeEach(() => { ... });` block (currently lines 11-48) with:

```ts
beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/balances') {
        return Promise.resolve(
          okResponse({
            items: [
              {
                account_id: 1,
                name: 'Assets:Bank',
                type: 'A',
                currency: 'USD',
                amount: 125000,
                is_hidden: false,
              },
              {
                account_id: 2,
                name: 'Assets:Cash',
                type: 'A',
                currency: 'USD',
                amount: 3500,
                is_hidden: false,
              },
              {
                account_id: 3,
                name: 'Liab:Card',
                type: 'L',
                currency: 'USD',
                amount: -42000,
                is_hidden: false,
              },
            ],
            total_count: 3,
            limit: 0,
            offset: 0,
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
```

Note: the existing stub used a non-promise `vi.fn(() => okResponse(...))` form. The browser `fetch` API returns a Promise, and `apiFetch` awaits it, so non-promise returns happen to work — but switching to `Promise.resolve(...)` is more honest about the contract. If the existing test continues to pass with non-promise returns, leave that detail as-is and only change the URL switch.

- [ ] **Step 2: Run the balances test to verify it still passes**

Run: `cd spa && npm run test -- balances`
Expected: PASS — `renders Net Worth headline from a balances response` still produces `$865.00`.

If the test now fails with a routing error (`useServerConfig must be used inside ServerConfigProvider`), it means the provider in `setup.tsx` was missed in Task 9.

---

## Task 11: Replace env-var read in `balances.tsx` with `useServerConfig`

**Files:**
- Modify: `spa/src/routes/balances.tsx`

- [ ] **Step 1: Edit imports and component**

Open `spa/src/routes/balances.tsx`. Make three edits:

1. Add the import at the top alongside the others (after the `getBalances` import on line 7):

```tsx
import { useServerConfig } from '@/lib/server-config';
```

2. Delete the module-level constant on line 12:

```tsx
const DEFAULT_CURRENCY = import.meta.env.VITE_DEFAULT_CURRENCY || 'USD';
```

3. Inside `function BalancesPage()`, add this as the first line of the function body (above `const query = useQuery(...)`):

```tsx
  const { defaults } = useServerConfig();
  const DEFAULT_CURRENCY = defaults.currency;
```

- [ ] **Step 2: Re-run the balances test**

Run: `cd spa && npm run test -- balances`
Expected: PASS — same `$865.00` assertion succeeds because the stubbed `/api/config` returns `USD`.

- [ ] **Step 3: Type-check and lint**

Run: `cd spa && npm run check`
Expected: PASS.

---

## Task 12: Remove `VITE_DEFAULT_CURRENCY` from `vite-env.d.ts`, `.env`, `.env.example`

The env var is no longer consumed by any code. Remove every trace.

**Files:**
- Modify: `spa/src/vite-env.d.ts`
- Modify: `spa/.env`
- Modify: `spa/.env.example`

- [ ] **Step 1: Inspect `vite-env.d.ts` first**

Run: `cat spa/src/vite-env.d.ts`
Expected output (or similar):

```ts
/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_DEFAULT_CURRENCY?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
```

- [ ] **Step 2: Replace `vite-env.d.ts` contents**

If `ImportMetaEnv` would become empty after removing the only field, replace the file with the minimal Vite default:

```ts
/// <reference types="vite/client" />
```

(Vite's built-in client types already declare `ImportMeta.env` with built-ins like `MODE`, `BASE_URL`, etc. The project-specific override is no longer needed.)

If `vite-env.d.ts` has additional fields beyond `VITE_DEFAULT_CURRENCY`, only remove the `readonly VITE_DEFAULT_CURRENCY?: string;` line and leave the rest intact.

- [ ] **Step 3: Inspect `.env` and `.env.example`**

Run: `cat spa/.env spa/.env.example`
Expected output:

```
VITE_DEFAULT_CURRENCY=TWD
# Currency used for the Balances headline. Falls back to USD if unset.
VITE_DEFAULT_CURRENCY=USD
```

- [ ] **Step 4: Delete both files**

Both files contain only the env var (plus an explanatory comment in `.env.example`). With the var gone, they have no remaining content. Remove them:

```bash
rm spa/.env spa/.env.example
```

If a future configuration arrives that needs `.env.example`, it will be recreated then.

- [ ] **Step 5: Verify no references remain**

Run: `grep -rn "VITE_DEFAULT_CURRENCY" spa/ 2>/dev/null`
Expected: no output (no matches anywhere in `spa/`).

- [ ] **Step 6: Type-check**

Run: `cd spa && npm run check`
Expected: PASS.

---

## Task 13: Update `spa/README.md`

Two changes: remove the configuration section about the env var, and remove the "upcoming `/api/config`" bullet from the Status section.

**Files:**
- Modify: `spa/README.md`

- [ ] **Step 1: Remove the `VITE_DEFAULT_CURRENCY` paragraph from "## Configuration"**

Find the line (around line 48):

```markdown
`VITE_DEFAULT_CURRENCY` — currency used for the Net Worth headline. Defaults to `USD` if unset. Copy `.env.example` to `.env` to override.
```

Delete this entire line. The remaining `as_of` paragraph in the Configuration section stays.

- [ ] **Step 2: Remove the "upcoming `/api/config`" bullet from "## Status"**

Find the line (around line 57):

```markdown
- `GET /api/config` endpoint for server-side default currency
```

Delete this entire line.

- [ ] **Step 3: Verify**

Run: `grep -n "VITE_DEFAULT_CURRENCY\|/api/config" spa/README.md`
Expected: no output.

---

## Task 14: Manual end-to-end verification

Confirms the runtime behavior actually changes when `cfg.Defaults.Currency` does, with no `.env` involvement.

- [ ] **Step 1: Set `defaults.currency` to a non-USD value**

If `~/.config/kea/config.yaml` does not have `defaults.currency`, add it:

```yaml
defaults:
  currency: TWD
```

Otherwise change the existing value to `TWD`.

- [ ] **Step 2: Start the Go server**

In one terminal:

```bash
make run -- serve
```

(or `go run ./cmd/kea serve`)

Expected: server listens on `:8080`, no errors.

- [ ] **Step 3: Verify the endpoint manually**

In another terminal:

```bash
curl -s http://localhost:8080/api/config
```

Expected: `{"defaults":{"currency":"TWD"}}`.

- [ ] **Step 4: Start the SPA dev server**

```bash
cd spa && npm run dev
```

Expected: Vite listens on `:5173`, no errors. Open `http://localhost:5173/balances` in a browser. The Net Worth headline and per-type cards should be formatted with the `TWD` currency.

- [ ] **Step 5: Flip the config back to `USD` and restart**

Edit `~/.config/kea/config.yaml` to `defaults.currency: USD`, then `Ctrl-C` and re-run the Go server. Refresh the browser. The dashboard should now format in `USD` without any `.env` change.

- [ ] **Step 6: Confirm no `.env` involvement**

```bash
ls spa/.env spa/.env.example 2>&1
```

Expected: `ls: spa/.env: No such file or directory` (or equivalent for both).

---

## Task 15: Final acceptance check + SPA commit

- [ ] **Step 1: Run the full Go test suite and build**

Run: `go test ./... && go build ./...`
Expected: PASS.

- [ ] **Step 2: Run the full SPA suite**

Run: `cd spa && npm run check && npm run test && npm run build`
Expected: PASS for all three.

- [ ] **Step 3: Inspect `git status` to confirm only the expected SPA files changed**

Run: `git status`
Expected dirty files:
- modified: `spa/README.md`
- modified: `spa/src/lib/api.ts`
- modified: `spa/src/lib/types.ts`
- modified: `spa/src/main.tsx`
- modified: `spa/src/routes/balances.tsx`
- modified: `spa/src/test/balances.test.tsx`
- modified: `spa/src/test/setup.tsx`
- modified: `spa/src/vite-env.d.ts`
- deleted: `spa/.env`
- deleted: `spa/.env.example`
- new file: `spa/src/lib/server-config.tsx`
- new file: `spa/src/test/server-config.test.tsx`

If anything else appears, investigate before committing.

- [ ] **Step 4: Stage and commit**

```bash
git add spa/
git commit -m "feat(spa): bootstrap runtime config from GET /api/config

Fetch the server's default currency at app boot via TanStack Query and
expose it through a ServerConfigProvider + useServerConfig hook. The
root gates child route rendering on a successful config fetch so every
page can rely on the value without per-page fallbacks. Removes the
VITE_DEFAULT_CURRENCY env var (and the .env, .env.example, vite-env.d.ts
type declaration) so the SPA and the Go server agree on a single source
of truth for default currency."
```

- [ ] **Step 5: Verify the two commits are on the branch**

Run: `git log --oneline -2`
Expected: SPA commit at HEAD, Go-side commit at HEAD~1, both authored by the implementer (no `Co-Authored-By` footer).
