# SPA "Hide Decimals" Amount Display Option — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a server config option `display.hide_decimals` that, when true, makes the SPA render monetary amounts rounded to the nearest whole unit (e.g., `$2,600` instead of `$2,600.00`, and `$2,601` instead of `$2,600.50`).

**Architecture:** A boolean field is added to the Go `Config` struct and exposed via the existing `/api/config` endpoint. The SPA reads it through `useServerConfig`, and a thin `useAmountFormat()` hook returns config-bound versions of `formatCents` and `formatBalanceAbs`. Every SPA component that currently imports those formatters directly migrates to the hook. Default value is `false`, preserving today's two-decimal output.

**Tech Stack:** Go 1.x with `spf13/viper`, `testify`-style stdlib tests, `chi` router; React + TypeScript SPA with TanStack Query/Router, Vitest, React Testing Library.

**Spec:** [docs/superpowers/specs/2026-06-17-spa-hide-decimals-design.md](../specs/2026-06-17-spa-hide-decimals-design.md)

---

## Task 1: Add `DisplayConfig` to Go config struct

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/config/config_test.go`:

```go
func TestNewDefault_DisplayConfig(t *testing.T) {
	cfg := NewDefault()

	if cfg.Display.HideDecimals != false {
		t.Errorf("expected default HideDecimals=false, got %v", cfg.Display.HideDecimals)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestNewDefault_DisplayConfig -v`
Expected: FAIL — `cfg.Display undefined` (compile error)

- [ ] **Step 3: Add the `Display` field and `DisplayConfig` struct**

Replace the entire contents of `internal/config/config.go` with:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package config

type Config struct {
	Database   DatabaseConfig `mapstructure:"database"`
	Defaults   DefaultsConfig `mapstructure:"defaults"`
	Display    DisplayConfig  `mapstructure:"display"`
	Server     ServerConfig   `mapstructure:"server"`
	ConfigPath string         `mapstructure:"-"`
}

type ServerConfig struct {
	Host        string   `mapstructure:"host"`
	Port        int      `mapstructure:"port"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type DefaultsConfig struct {
	Currency string `mapstructure:"currency"`
}

type DisplayConfig struct {
	HideDecimals bool `mapstructure:"hide_decimals"`
}

func NewDefault() *Config {
	return &Config{
		Database: DatabaseConfig{Path: ""},
		Defaults: DefaultsConfig{},
		Display:  DisplayConfig{HideDecimals: false},
		Server: ServerConfig{
			Host:        "localhost",
			Port:        8080,
			CORSOrigins: []string{"http://localhost:5173"},
		},
	}
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS — both `TestNewDefault_ServerConfig` and `TestNewDefault_DisplayConfig`.

- [ ] **Step 5: Verify the rest of the project still builds**

Run: `go build ./...`
Expected: exit 0, no output.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add display.hide_decimals option"
```

---

## Task 2: Expose `display.hide_decimals` via `/api/config`

**Files:**
- Modify: `internal/api/config.go`
- Modify: `internal/api/config_test.go`
- Modify: `internal/api/testhelper_test.go`

- [ ] **Step 1: Add a test helper that lets callers set `Display.HideDecimals`**

Open `internal/api/testhelper_test.go` and insert this function immediately after the existing `newServerForWriteWithCurrency` (around line 90, right before the `newServerForWrite` definition):

```go
// newServerForWriteWithDisplay is a variant of newServerForWriteWithCurrency
// that also lets the caller set cfg.Display.HideDecimals. Used by /api/config
// tests that exercise the display option.
func newServerForWriteWithDisplay(t *testing.T, currency string, hideDecimals bool) (*httptest.Server, *service.Service, *store.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(dbPath, migrations.FS)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := config.NewDefault()
	cfg.Defaults.Currency = currency
	cfg.Display.HideDecimals = hideDecimals

	svc := service.NewService(st, st, st, cfg)
	srv := NewServer(cfg, svc, nil, nil, "", nil, discardLogger())
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

	return ts, svc, st
}
```

- [ ] **Step 2: Replace the existing `/api/config` test with a wider table**

Replace the entire contents of `internal/api/config_test.go` with:

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
		name         string
		currency     string
		hideDecimals bool
		wantBody     string
	}{
		{"populated", "USD", false, `{"defaults":{"currency":"USD"},"display":{"hide_decimals":false}}`},
		{"empty", "", false, `{"defaults":{"currency":""},"display":{"hide_decimals":false}}`},
		{"non_default", "TWD", false, `{"defaults":{"currency":"TWD"},"display":{"hide_decimals":false}}`},
		{"hide_decimals", "USD", true, `{"defaults":{"currency":"USD"},"display":{"hide_decimals":true}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _, _ := newServerForWriteWithDisplay(t, tt.currency, tt.hideDecimals)

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

			var parsed struct {
				Defaults struct {
					Currency string `json:"currency"`
				} `json:"defaults"`
				Display struct {
					HideDecimals bool `json:"hide_decimals"`
				} `json:"display"`
			}
			if err := json.Unmarshal(raw, &parsed); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if parsed.Defaults.Currency != tt.currency {
				t.Errorf("parsed currency: got %q, want %q", parsed.Defaults.Currency, tt.currency)
			}
			if parsed.Display.HideDecimals != tt.hideDecimals {
				t.Errorf("parsed hide_decimals: got %v, want %v", parsed.Display.HideDecimals, tt.hideDecimals)
			}
		})
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/api/ -run TestGetConfig -v`
Expected: FAIL — the response body still lacks the `display` key (the handler hasn't been updated yet).

- [ ] **Step 4: Update the handler and response struct**

Replace the entire contents of `internal/api/config.go` with:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import "net/http"

type configResponse struct {
	Defaults configDefaults `json:"defaults"`
	Display  configDisplay  `json:"display"`
}

type configDefaults struct {
	Currency string `json:"currency"`
}

type configDisplay struct {
	HideDecimals bool `json:"hide_decimals"`
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) error {
	cfg := s.svc.Config()
	return writeJSON(w, http.StatusOK, configResponse{
		Defaults: configDefaults{Currency: cfg.Defaults.Currency},
		Display:  configDisplay{HideDecimals: cfg.Display.HideDecimals},
	})
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/api/ -run TestGetConfig -v`
Expected: PASS — all four sub-tests.

- [ ] **Step 6: Run the full API test suite to check for regressions**

Run: `go test ./internal/api/...`
Expected: PASS.

- [ ] **Step 7: Run the full Go test suite**

Run: `go test ./...`
Expected: PASS across every package.

- [ ] **Step 8: Commit**

```bash
git add internal/api/config.go internal/api/config_test.go internal/api/testhelper_test.go
git commit -m "feat(api): expose display.hide_decimals in /api/config response"
```

---

## Task 3: Extend SPA `ServerConfig` type

**Files:**
- Modify: `spa/src/lib/types.ts`

- [ ] **Step 1: Extend the interface**

Open `spa/src/lib/types.ts`, find the `ServerConfig` interface (around line 20), and replace it with:

```ts
export interface ServerConfig {
  defaults: {
    currency: string;
  };
  display: {
    hide_decimals: boolean;
  };
}
```

- [ ] **Step 2: Run TypeScript type-check**

Run: `cd spa && npm run check`
Expected: type errors in every test file that mocks `/api/config` without the new `display` field. This is expected — we'll fix them in Task 6. Note the error list; it should match the file list in Task 6 Step 2.

- [ ] **Step 3: Do NOT commit yet**

Type-only changes will land together with Task 4–6 in a single SPA commit, after the formatter changes, hook, and callsite migrations are also in place.

---

## Task 4: Add `hideDecimals` option to SPA formatters

**Files:**
- Modify: `spa/src/lib/format.ts`
- Modify: `spa/src/lib/format.test.ts`

- [ ] **Step 1: Write the failing tests**

Append to `spa/src/lib/format.test.ts`:

```ts
import { formatBalanceAbs } from './format';

describe('formatCents with hideDecimals: true', () => {
  test('drops decimals for whole values', () => {
    expect(formatCents(260000, 'USD', { hideDecimals: true })).toBe('$2,600');
  });

  test('rounds half away from zero (positive)', () => {
    expect(formatCents(260050, 'USD', { hideDecimals: true })).toBe('$2,601');
  });

  test('rounds half away from zero (negative)', () => {
    expect(formatCents(-260050, 'USD', { hideDecimals: true })).toBe('-$2,601');
  });

  test('does not round below half', () => {
    expect(formatCents(260049, 'USD', { hideDecimals: true })).toBe('$2,600');
  });

  test('falls back gracefully on unknown currency', () => {
    expect(() => formatCents(260050, 'ZZZ', { hideDecimals: true })).not.toThrow();
  });
});

describe('formatBalanceAbs', () => {
  test('renders two decimals by default', () => {
    expect(formatBalanceAbs(260000)).toBe('$2,600.00');
    expect(formatBalanceAbs(-42050)).toBe('$420.50');
  });

  test('drops decimals when hideDecimals is true', () => {
    expect(formatBalanceAbs(260000, { hideDecimals: true })).toBe('$2,600');
  });

  test('rounds and strips sign when hideDecimals is true', () => {
    expect(formatBalanceAbs(-260050, { hideDecimals: true })).toBe('$2,601');
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd spa && npx vitest run src/lib/format.test.ts`
Expected: FAIL — `formatCents`/`formatBalanceAbs` reject the third / second argument (type error or runtime ignore depending on existing signature).

- [ ] **Step 3: Update the formatters**

Replace the entire contents of `spa/src/lib/format.ts` with:

```ts
export interface AmountFormatOptions {
  hideDecimals?: boolean;
}

export function formatCents(
  cents: number,
  currency: string,
  options: AmountFormatOptions = {},
): string {
  const value = cents / 100;
  const digits = options.hideDecimals ? 0 : 2;
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency,
      minimumFractionDigits: digits,
      maximumFractionDigits: digits,
    }).format(value);
  } catch {
    // Unknown currency code — fall back to plain number with the code.
    return `${value.toFixed(digits)} ${currency}`;
  }
}

// `$X,XXX.XX` with no currency code and no minus sign. Used where the row's
// type/column already conveys account and the surrounding color conveys sign.
export function formatBalanceAbs(
  cents: number,
  options: AmountFormatOptions = {},
): string {
  const abs = Math.abs(cents / 100);
  const digits = options.hideDecimals ? 0 : 2;
  return `$${abs.toLocaleString('en-US', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })}`;
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd spa && npx vitest run src/lib/format.test.ts`
Expected: PASS — every test in the file (including the original three) green.

- [ ] **Step 5: Do NOT commit yet**

Hold the commit until the hook and callsite migration are in place (Task 6).

---

## Task 5: Add `useAmountFormat` hook

**Files:**
- Modify: `spa/src/lib/server-config.tsx`
- Modify: `spa/src/test/server-config.test.tsx`

- [ ] **Step 1: Extend `server-config.tsx` with the hook**

Replace the entire contents of `spa/src/lib/server-config.tsx` with:

```tsx
import { useQuery } from '@tanstack/react-query';
import { type ReactNode, createContext, useContext } from 'react';
import { getConfig } from './api';
import { formatBalanceAbs, formatCents } from './format';
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

export function useAmountFormat() {
  const cfg = useServerConfig();
  const opts = { hideDecimals: cfg.display.hide_decimals };
  return {
    formatCents: (cents: number, currency: string) => formatCents(cents, currency, opts),
    formatBalanceAbs: (cents: number) => formatBalanceAbs(cents, opts),
  };
}
```

- [ ] **Step 2: Update existing `server-config.test.tsx` mocks to include `display`**

Open `spa/src/test/server-config.test.tsx`. Every `/api/config` mock payload — there is one around line 34, search for `defaults: { currency` — must be extended:

```ts
// before
return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));

// after
return Promise.resolve(
  okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
);
```

(If the helper builds the payload via an object literal of a different shape, mirror the new field there; do not change the asserted behavior of the existing tests.)

- [ ] **Step 3: Run the file-level tests to confirm they still pass**

Run: `cd spa && npx vitest run src/test/server-config.test.tsx`
Expected: PASS — existing assertions hold; the new `display` field is just along for the ride.

- [ ] **Step 4: Do NOT commit yet**

Continue to Task 6 — the SPA commit is one batch.

---

## Task 6: Migrate SPA callsites and update test fixtures

**Files (callsite migration — change `import { formatCents } from '@/lib/format'` to use `useAmountFormat` from `@/lib/server-config`):**
- Modify: `spa/src/components/NetWorthCard.tsx`
- Modify: `spa/src/components/accounts/ChildAccountsCard.tsx`
- Modify: `spa/src/components/accounts/AccountDetailHeader.tsx`
- Modify: `spa/src/components/accounts/AccountSearchResults.tsx`
- Modify: `spa/src/components/accounts/RecentTransactionsCard.tsx`
- Modify: `spa/src/components/transactions/TransactionRow.tsx`
- Modify: `spa/src/components/reports/KpiCard.tsx`
- Modify: `spa/src/components/reports/CurrencyFooter.tsx`
- Modify: `spa/src/components/reports/ProportionBar.tsx`
- Modify: `spa/src/components/reports/ReportRowTable.tsx`
- Modify: `spa/src/routes/reports.balance-sheet.tsx`
- Modify: `spa/src/routes/reports.net-worth.tsx`
- Modify: `spa/src/routes/transactions.$id.index.tsx`

**Files (test fixture updates — add `display: { hide_decimals: false }` to `/api/config` mocks):**
- Modify: `spa/src/test/transactions.reconciled.test.tsx`
- Modify: `spa/src/test/reports.tab-nav.test.tsx`
- Modify: `spa/src/test/transactions.list.test.tsx`
- Modify: `spa/src/test/reports.income-breakdown.test.tsx`
- Modify: `spa/src/test/reports.income-statement.test.tsx`
- Modify: `spa/src/test/reports.net-worth.test.tsx`
- Modify: `spa/src/test/balances.test.tsx` (3 occurrences)
- Modify: `spa/src/test/transactions.form.test.tsx`
- Modify: `spa/src/test/reports.expense-breakdown.test.tsx`
- Modify: `spa/src/test/reports.balance-sheet.test.tsx`

- [ ] **Step 1: Verify the callsite list is current**

Run: `cd spa && grep -rn "from '@/lib/format'" src --include='*.ts' --include='*.tsx'`
Expected: exactly the 13 callsite files listed above (plus `src/lib/format.test.ts`, which is allowed). If the list differs (files added/removed since the spec), update the migration target list.

- [ ] **Step 2: Verify the test fixture list is current**

Run: `cd spa && grep -rln "'/api/config'" src/test`
Expected: every file matches the test fixture list above. If new test files import `/api/config`, add them.

- [ ] **Step 3: For each callsite, swap the import and use the hook inside the component body**

For each file in the callsite list, apply this mechanical change. The exact lines vary, but the pattern is identical.

Example (`spa/src/components/NetWorthCard.tsx`):

```tsx
// before
import { formatCents } from '@/lib/format';
// …
export function NetWorthCard({ netWorth, currency }: Props) {
  return <div>{formatCents(netWorth, currency)}</div>;
}

// after
import { useAmountFormat } from '@/lib/server-config';
// …
export function NetWorthCard({ netWorth, currency }: Props) {
  const { formatCents } = useAmountFormat();
  return <div>{formatCents(netWorth, currency)}</div>;
}
```

For components that use `formatBalanceAbs` (`AccountSearchResults.tsx`, `RecentTransactionsCard.tsx`, `TransactionRow.tsx`):

```tsx
// before
import { formatBalanceAbs } from '@/lib/format';
// …
{formatBalanceAbs(amount)}

// after
import { useAmountFormat } from '@/lib/server-config';
// …
const { formatBalanceAbs } = useAmountFormat();
{formatBalanceAbs(amount)}
```

For files that import both (`KpiCard.tsx` only uses `formatCents`, others vary — destructure both from the hook if needed):

```tsx
const { formatCents, formatBalanceAbs } = useAmountFormat();
```

Notes that apply to specific files:

- `AccountSearchResults.tsx`: it currently uses `formatBalanceAbs` at module scope inside an exported helper function. The hook is React-only — convert the surrounding helper into a component, OR call the hook in the consumer and pass the formatter down. The component path is preferred; rename and inline.
- `KpiCard.tsx`: keep the `.toFixed(1)` percentage calls untouched — they are not amounts.
- `transactions.$id.index.tsx`: the route component already runs as a function component, so the hook can be called inside its body alongside other hooks.

After each file edit, do not run the dev server. Type-check / test runs happen in Step 5.

- [ ] **Step 4: For each test fixture, extend the `/api/config` mock payload**

For each file in the test fixture list, find every occurrence of:

```ts
if (url === '/api/config') {
  return Promise.resolve(okResponse({ defaults: { currency: '<X>' } }));
}
```

and rewrite it as:

```ts
if (url === '/api/config') {
  return Promise.resolve(
    okResponse({ defaults: { currency: '<X>' }, display: { hide_decimals: false } }),
  );
}
```

Keep the `<X>` currency value the same as before (most are `'USD'`; some `report.*` tests may use a different value — preserve it). Apply once per fixture occurrence; `balances.test.tsx` has three.

- [ ] **Step 5: Run the TypeScript type-checker**

Run: `cd spa && npm run check`
Expected: zero errors. If any callsite or test fixture is missing the migration / mock update, the type-checker will name the file.

- [ ] **Step 6: Run the SPA test suite**

Run: `cd spa && npm run test`
Expected: PASS — every test green. If a test fails because the component now requires `ServerConfigProvider` and the test renders the component outside `makeTestApp`, wrap the assertion in `makeTestApp` or add a local `ServerConfigProvider` with a stubbed `/api/config` fetch. (This affects any direct `render(<NetWorthCard … />)` style test if one exists. Component-level tests under `src/test/components` may need a wrapper.)

- [ ] **Step 7: Build the SPA to confirm production output**

Run: `cd spa && npm run build`
Expected: build succeeds, no warnings about unused imports.

- [ ] **Step 8: Verify no SPA file imports `formatCents`/`formatBalanceAbs` outside the allowed set**

Run: `cd spa && grep -rn "from '@/lib/format'" src --include='*.ts' --include='*.tsx'`
Expected: exactly two matches — `src/lib/format.test.ts` and `src/lib/server-config.tsx`. Anything else is a missed migration.

- [ ] **Step 9: Commit**

```bash
git add spa/src/lib/types.ts spa/src/lib/format.ts spa/src/lib/format.test.ts \
        spa/src/lib/server-config.tsx \
        spa/src/components/NetWorthCard.tsx \
        spa/src/components/accounts/ChildAccountsCard.tsx \
        spa/src/components/accounts/AccountDetailHeader.tsx \
        spa/src/components/accounts/AccountSearchResults.tsx \
        spa/src/components/accounts/RecentTransactionsCard.tsx \
        spa/src/components/transactions/TransactionRow.tsx \
        spa/src/components/reports/KpiCard.tsx \
        spa/src/components/reports/CurrencyFooter.tsx \
        spa/src/components/reports/ProportionBar.tsx \
        spa/src/components/reports/ReportRowTable.tsx \
        spa/src/routes/reports.balance-sheet.tsx \
        spa/src/routes/reports.net-worth.tsx \
        spa/src/routes/transactions.$id.index.tsx \
        spa/src/test/
git commit -m "feat(spa): honor display.hide_decimals when formatting amounts"
```

---

## Task 7: Manual verification

**Files:** none modified — this task verifies end-to-end behavior.

- [ ] **Step 1: With the default config, confirm nothing visible changed**

Make sure `display:` is absent from `~/.config/kea/config.yaml` (or `display.hide_decimals: false` is present). Then run, in two terminals:

```bash
go run ./cmd/kea serve
```

```bash
cd spa && npm run dev
```

Open the SPA in a browser, navigate to `/balances`, `/transactions`, `/reports/balance-sheet`, `/reports/net-worth`, `/reports/income-statement`. Confirm every amount displays exactly as before this change (`$X,XXX.XX`).

- [ ] **Step 2: Turn on the option**

Edit `~/.config/kea/config.yaml` and add:

```yaml
display:
  hide_decimals: true
```

Stop and restart `kea serve`. Hard-reload the SPA in the browser.

Walk the same routes. Confirm:
- Whole-unit amounts display without a decimal portion (e.g., `$2,600`, not `$2,600.00`).
- Any transaction known to have non-zero cents (seed one, e.g., `$2,600.50`) displays rounded to `$2,601`.
- Negative amounts show `-$2,601` for `-$2,600.50`.
- The "amount" column width in tables visibly shrinks.

- [ ] **Step 3: Confirm CLI behavior is unchanged**

```bash
go run ./cmd/kea transaction list --limit 5
```

The CLI output is unchanged from before this change — `utils.FormatAmount` was not touched. Trailing zeros remain trimmed (e.g., `2,600`, not `2,600.00` and not `2,601` when the value is `2,600.5`).

- [ ] **Step 4: Reset the config**

Either remove the `display:` block or set `display.hide_decimals: false`, then restart the server. Confirm the SPA returns to two-decimal output.

- [ ] **Step 5: (Optional) Commit any test-config or seed-data changes**

If Step 2 required seeding a fractional-cents transaction for verification, do not commit that DB change — it is local fixture data.

---

## Done criteria

- `go test ./...` and `go build ./...` green.
- `cd spa && npm run check && npm run test && npm run build` green.
- With `display.hide_decimals` absent or `false`, the SPA renders amounts identically to before this change.
- With `display.hide_decimals: true`, every SPA money-display callsite renders amounts as whole units, rounded half-away-from-zero (`$2,600.50` → `$2,601`).
- `grep -rn "from '@/lib/format'" spa/src` matches only `lib/format.test.ts` and `lib/server-config.tsx`.
- CLI/TUI behavior unchanged.
