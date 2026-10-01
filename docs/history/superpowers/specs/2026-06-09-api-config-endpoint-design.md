# `GET /api/config` Endpoint and SPA Runtime Config Bootstrap

**Date:** 2026-06-09
**Status:** Approved
**Scope:** One PR with two commits (`feat(api):` + `feat(spa):`)

## Problem

The SPA's `/balances` route formats amounts in a "default currency" that must match the server's `cfg.Defaults.Currency`. If they disagree, accounts with empty `Currency` get normalized server-side to one value and excluded client-side by another, producing a visually broken dashboard with a large `excludedByCurrency.length`.

Today the SPA reads its default from `import.meta.env.VITE_DEFAULT_CURRENCY` at Vite build/dev startup, with `'USD'` as a fallback. This has two issues:

1. **Drift risk.** A user setting `cfg.Defaults.Currency = "TWD"` (via `kea` first-run or by editing `~/.config/kea/config.yaml`) silently leaves the SPA on USD until they also edit `spa/.env`.
2. **Out-of-band configuration.** Every user must know about both the Go-side config and the Vite env var. The SPA `README.md` documents both, but it remains a footgun.

The fix is for the server to expose its loaded config to the SPA at runtime via a new `GET /api/config` endpoint, and for the SPA to consume that single source of truth.

## Non-Goals

- Hot-reloading config without a server restart. The endpoint reflects config at the moment of the request; in practice it is loaded once at startup and not mutated by request handlers.
- HTTP-level caching headers (`Cache-Control`, ETag). The SPA caches the result in TanStack Query for the session; there is no benefit to adding browser-cache headers for a localhost SPA on a small payload.
- Including mutable runtime state (e.g., `ActiveLedger`, current ledger path). Mutable state must not be served from `/api/config`; `/api/ledgers/active` already covers `ActiveLedger`.
- Adding fields beyond `defaults.currency`. Future fields (locale, date format, server version) can be added when a consumer needs them.

## Architecture

A read-only endpoint `GET /api/config` returns a snapshot of the server's startup configuration. The SPA fetches it once at app boot via TanStack Query, gates child route rendering on success, and exposes the result through a small React context. Routes that need the value read it via a `useServerConfig()` hook.

### Layers touched

- `internal/api/`: new `config.go` handler + route registration in `router.go` + test in `config_test.go` + test helper extension in `testhelper_test.go`.
- `spa/src/lib/api.ts`: new `getConfig()` function.
- `spa/src/lib/types.ts`: new `ServerConfig` interface.
- `spa/src/lib/server-config.tsx` (new file): `ServerConfigProvider` component and `useServerConfig` hook.
- `spa/src/main.tsx`: wrap `RouterProvider` with `ServerConfigProvider`.
- `spa/src/routes/balances.tsx`: replace `import.meta.env.VITE_DEFAULT_CURRENCY` read with `useServerConfig().defaults.currency`.
- `spa/src/test/setup.tsx`: wrap `makeTestApp` with `ServerConfigProvider` so existing and future tests get the provider for free.
- `spa/src/test/balances.test.tsx`: convert the fetch stub to be URL-aware so `/api/config` and `/api/balances` both resolve.
- `spa/src/test/server-config.test.tsx` (new file): minimal provider/hook test covering the "config not yet loaded" → "config loaded" transition.
- `spa/README.md`: remove the `VITE_DEFAULT_CURRENCY` section.
- `spa/.env.example`: remove the entry if it exists (verify during implementation).

## Endpoint Contract

**Request:** `GET /api/config` — no params, no auth (matches every other Kea endpoint).

**Response:** `200 OK`, `Content-Type: application/json`

```json
{
  "defaults": {
    "currency": "USD"
  }
}
```

**Go types** (in `internal/api/config.go`):

```go
type configResponse struct {
    Defaults configDefaults `json:"defaults"`
}
type configDefaults struct {
    Currency string `json:"currency"`
}
```

**Handler:** reads from `s.svc.Config().Defaults.Currency`. No DB calls. No expected error paths beyond the panic recoverer. Mirrors `handleVersion` in simplicity.

**Empty-value semantics:** if `cfg.Defaults.Currency` is unset (zero value `""`), the endpoint returns `"currency": ""`. The SPA decides how to handle that (see SPA section). The server does not substitute a default for a default — doing so would obscure misconfiguration.

**Snapshot semantics:** the endpoint reflects config at the moment of the request. No stability guarantee is documented, but in practice these fields are loaded once at startup and the SPA may cache the response for the session.

### Why this shape

The response mirrors the Go `Config` struct layout (`cfg.Defaults.Currency`) rather than a flat `{"default_currency": "USD"}`. Rationale:

- Future fields (`defaults.locale`, `defaults.date_format`) extend naturally without colliding with a flat namespace.
- Re-grouping a flat field later would be a breaking change to a public endpoint.
- The minor cost — one extra level of indirection today — is paid once in the SPA's `getConfig` consumer.

### What is NOT included

- `ActiveLedger` — mutable, covered by `/api/ledgers/active`.
- Server version — covered by `/api/version`.
- CORS origins — server-internal.
- Build SHA — not yet tracked anywhere.

## SPA Bootstrap

### `ServerConfigProvider` and `useServerConfig`

New file `spa/src/lib/server-config.tsx`:

```tsx
import { createContext, useContext, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getConfig } from './api';
import type { ServerConfig } from './types';

const ServerConfigContext = createContext<ServerConfig | null>(null);

export function ServerConfigProvider({
  children,
  fallback,
}: {
  children: (cfg: ServerConfig) => ReactNode;
  fallback: ReactNode;
}) {
  const query = useQuery({
    queryKey: ['server-config'],
    queryFn: getConfig,
    staleTime: Infinity,
  });
  if (query.isPending || query.isError) return <>{fallback}</>;
  return (
    <ServerConfigContext.Provider value={query.data}>
      {children(query.data)}
    </ServerConfigContext.Provider>
  );
}

export function useServerConfig(): ServerConfig {
  const cfg = useContext(ServerConfigContext);
  if (!cfg) throw new Error('useServerConfig must be used inside ServerConfigProvider');
  return cfg;
}
```

The provider is a separate file from `lib/api.ts` because it composes a context, a query, and a render gate; `lib/api.ts` stays a thin fetch module.

### Type additions in `spa/src/lib/types.ts`

```ts
export interface ServerConfig {
  defaults: { currency: string };
}
```

### `main.tsx` wiring

Wrap `<RouterProvider>` inside `<ServerConfigProvider>` using the children-as-function pattern so the router only mounts once the config has loaded. The fallback is a minimal top-level skeleton — a centered spinner is sufficient because this is a localhost SPA and the wait is on the order of milliseconds.

### Loading and error semantics

- **Loading:** the provider renders `fallback` while the query is pending. The router (and therefore every route) only mounts after `/api/config` resolves successfully.
- **Error:** the provider keeps `fallback` on screen when the query errors. This is the same UX as any other API failure on this SPA, and it correctly signals "the backend is not reachable" rather than rendering a dashboard with stale or guessed values. A "Retry" button mirroring `balances.tsx`'s error state is optional and may be added later if needed.
- **Empty currency from server:** if `defaults.currency === ''`, the SPA passes `''` to `summarizeBalances`, which still works (empty string becomes an exclusion key). The dashboard correctly displays "everything excluded" — the correct signal that the user has not configured a default currency. The SPA does not fall back to a client-side `'USD'`.

### `balances.tsx` change

Replace the module-level env read:

```tsx
// before
const DEFAULT_CURRENCY = import.meta.env.VITE_DEFAULT_CURRENCY || 'USD';
```

with a component-scope hook read:

```tsx
// after
function BalancesPage() {
  const { defaults } = useServerConfig();
  const DEFAULT_CURRENCY = defaults.currency;
  // ... rest of component unchanged
}
```

## Testing

### Go side — `internal/api/config_test.go`

A table-driven test covering populated and empty currency cases:

```go
func TestGetConfig(t *testing.T) {
    tests := []struct {
        name     string
        currency string
        want     string // expected JSON body
    }{
        {"populated", "USD", `{"defaults":{"currency":"USD"}}`},
        {"empty",     "",    `{"defaults":{"currency":""}}`},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ts, _, _ := newServerForWriteWithCurrency(t, tt.currency)
            // GET ts.URL + "/api/config"
            // assert 200, Content-Type application/json, body == tt.want
        })
    }
}
```

The empty-currency case requires a harness variant because `newServerForWrite` hardcodes `"USD"`. Add a thin `newServerForWriteWithCurrency(t, currency string)` helper to `testhelper_test.go` that mirrors the existing factory with a parameterized currency. This matches the existing "factory per shape" pattern (`newServerWithStore`, `newServerForWrite`, `newTestServerWithLedger`).

### SPA side — `spa/src/test/balances.test.tsx`

Convert the current URL-agnostic `fetch` stub to a URL-aware switch:

```ts
beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn((url: string) => {
    if (url === '/api/config') {
      return okResponse({ defaults: { currency: 'USD' } });
    }
    if (url === '/api/balances') {
      return okResponse({ items: [/* unchanged */], total_count: 3, limit: 0, offset: 0 });
    }
    throw new Error(`unexpected fetch: ${url}`);
  }));
});
```

### SPA side — `spa/src/test/setup.tsx`

`makeTestApp` must wrap the router in `<ServerConfigProvider>` exactly as `main.tsx` does. Without this, any component calling `useServerConfig` throws. Update once in `setup.tsx` so every existing and future test inherits the provider.

### SPA side — `spa/src/test/server-config.test.tsx` (new)

A minimal test mounting a component that calls `useServerConfig` inside `<ServerConfigProvider>`. Stubs `fetch` for `/api/config`, asserts that:

1. The fallback renders before the query resolves.
2. After the query resolves, the consuming component reads the expected `defaults.currency`.

This covers the loading → loaded transition that nothing else exercises.

## File and Commit Plan

Single PR with two commits so each scope is reviewable in isolation.

### Commit 1 — `feat(api): add GET /api/config endpoint`

- `internal/api/config.go` (new) — handler + response types
- `internal/api/router.go` — register `GET /api/config`
- `internal/api/config_test.go` (new) — table test for populated + empty currency
- `internal/api/testhelper_test.go` — new `newServerForWriteWithCurrency` helper

### Commit 2 — `feat(spa): bootstrap runtime config from GET /api/config`

- `spa/src/lib/types.ts` — add `ServerConfig` interface
- `spa/src/lib/api.ts` — add `getConfig()`
- `spa/src/lib/server-config.tsx` (new) — provider + `useServerConfig` hook
- `spa/src/main.tsx` — wrap router with `ServerConfigProvider`
- `spa/src/routes/balances.tsx` — replace env var with hook
- `spa/src/test/setup.tsx` — wrap `makeTestApp` with provider
- `spa/src/test/balances.test.tsx` — URL-aware fetch stub
- `spa/src/test/server-config.test.tsx` (new) — provider/hook test
- `spa/README.md` — remove `VITE_DEFAULT_CURRENCY` section
- `spa/.env.example` — remove the entry if present (verify during implementation)

## Acceptance Criteria

- `go test ./...` and `go build ./...` green.
- `cd spa && npm run check && npm run test && npm run build` green.
- Manual verification: run `kea serve` and `cd spa && npm run dev`, change `cfg.Defaults.Currency` between `USD` and a non-USD value (via `config.yaml` plus a server restart), and confirm that `/balances` reflects the new currency without touching `spa/.env`.
- `VITE_DEFAULT_CURRENCY` is gone from the codebase: no references in `spa/src/**`, no documentation in `spa/README.md`, no entry in `spa/.env.example`.

## Conventions

- Go: SPDX header on every new `.go` file (`// SPDX-License-Identifier: GPL-3.0-or-later` / `// Copyright (C) 2026  Hance Chin`).
- TypeScript: no SPDX header (matching existing SPA files).
- Conventional commit format. Scopes: `(api)` for Go-side commit, `(spa)` for SPA-side commit.
- Table-driven Go tests using stdlib + `testify` + `httptest` patterns already in `internal/api/`.
- Vitest + RTL on the SPA side.
- No `Co-Authored-By` footer on commits.
