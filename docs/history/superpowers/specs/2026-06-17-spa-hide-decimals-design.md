# SPA "Hide Decimals" Amount Display Option

**Date:** 2026-06-17
**Status:** Approved
**Scope:** One PR with two commits (`feat(api):` + `feat(spa):`)

## Problem

The SPA always renders monetary amounts with two fractional digits (e.g., `$2,600.00`, `-$420.00`). For users whose ledgers contain mostly whole-unit amounts, the trailing `.00` is visual noise that consumes column width and makes balance sheets harder to scan at a glance.

The Go CLI/TUI already trims trailing zeros via `utils.FormatAmount` (`260000` cents → `"2,600"`), so the behavior between interfaces is also inconsistent.

The fix is to add a server-side config option that the SPA honors when formatting amounts: when on, amounts are rounded to the nearest whole unit and rendered without a fractional component (`$2,600`, `$2,601` for `$2,600.50`, `-$420` for `-$420.00`).

## Non-Goals

- CLI/TUI behavior. `utils.FormatAmount` is unchanged; trailing-zero trimming there is sufficient.
- A UI toggle in the SPA. The setting is loaded from `config.yaml` at server startup, like every other Kea config option. A future iteration could add a UI control.
- Hot reload. Changing `display.hide_decimals` requires a server restart, matching the rest of `internal/config`.
- Per-route or per-component overrides. The setting is global within the SPA session.
- Editing-aid precision. `SplitsEditor.tsx`'s balance hint (`(bal / 100).toFixed(2)`) remains two-decimal — it shows the user the exact remainder while they balance a transaction, which is not the same concern as display.
- Non-amount numeric formatting. Percentages in `KpiCard.tsx` and growth indicators in route components are not amounts and are unaffected.

## Architecture

A single boolean config field `display.hide_decimals` is added to the Go `Config` struct, exposed verbatim via the existing `/api/config` endpoint, and consumed by the SPA through `useServerConfig`. The two SPA money formatters (`formatCents`, `formatBalanceAbs`) take an optional `hideDecimals` flag; a thin `useAmountFormat()` hook in `lib/server-config.tsx` wires the current config into pre-bound versions of those functions for component callsites. Default is `false`, preserving current behavior for every existing user.

### Layers touched

- `internal/config/`: new `DisplayConfig` struct + `Display` field on `Config`; defaulting in `NewDefault()`; test coverage in `config_test.go`.
- `internal/api/config.go`: extend `configResponse` with `Display configDisplay` and propagate the value.
- `internal/api/config_test.go`: cover both values of `hide_decimals`.
- `internal/api/testhelper_test.go`: extend the existing currency-parameterized helper if needed so tests can pass a non-zero `Display` value.
- `spa/src/lib/types.ts`: extend `ServerConfig` with `display: { hide_decimals: boolean }`.
- `spa/src/lib/format.ts`: add optional `AmountFormatOptions` parameter to `formatCents` and `formatBalanceAbs`.
- `spa/src/lib/format.test.ts`: add `hideDecimals: true` cases (whole, half-up, half-down, negative half, currency fallback path).
- `spa/src/lib/server-config.tsx`: add `useAmountFormat()` hook that returns config-bound formatters.
- SPA callsites that import `formatCents` or `formatBalanceAbs` directly: migrate to `useAmountFormat()`.
- SPA test files that stub `/api/config`: extend the stubbed payload with the new `display` field.

## Go Side

### `internal/config/config.go`

```go
type Config struct {
    Database   DatabaseConfig `mapstructure:"database"`
    Defaults   DefaultsConfig `mapstructure:"defaults"`
    Display    DisplayConfig  `mapstructure:"display"`
    Server     ServerConfig   `mapstructure:"server"`
    ConfigPath string         `mapstructure:"-"`
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

YAML form:

```yaml
display:
  hide_decimals: true
```

When the key is omitted, viper unmarshal yields the zero value (`false`), which matches `NewDefault()` and preserves current behavior.

### `internal/api/config.go`

```go
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

The JSON key `hide_decimals` uses snake_case to match the existing convention on this endpoint (`defaults.currency`).

## SPA Side

### Type extension

`spa/src/lib/types.ts`:

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

### Formatters

`spa/src/lib/format.ts`:

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
    return `${value.toFixed(digits)} ${currency}`;
  }
}

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

**Rounding semantics:** `Intl.NumberFormat` rounds half-away-from-zero by default in modern JS engines, so `260050` cents (`$2,600.50`) becomes `$2,601`, and `-260050` becomes `-$2,601`. `260049` becomes `$2,600`. This matches the user-confirmed expectation of "round to nearest".

**Backward compatibility:** the new `options` parameter is optional and defaults to two-decimal behavior, so any callsite not yet migrated still compiles and produces unchanged output.

### `useAmountFormat` hook

`spa/src/lib/server-config.tsx` gains:

```ts
import { formatBalanceAbs, formatCents } from './format';

export function useAmountFormat() {
  const cfg = useServerConfig();
  const opts = { hideDecimals: cfg.display.hide_decimals };
  return {
    formatCents: (cents: number, currency: string) =>
      formatCents(cents, currency, opts),
    formatBalanceAbs: (cents: number) => formatBalanceAbs(cents, opts),
  };
}
```

The hook returns fresh closures on every render. That is acceptable here: the formatters are called inline during render, no memoization-sensitive consumer relies on referential equality, and `useServerConfig()` is already stable for the lifetime of the SPA session.

### Callsite migration

Every component currently importing `formatCents` from `@/lib/format` or `formatBalanceAbs` from `@/lib/format` migrates to the hook. Based on a grep of the SPA tree at the time of writing, the following files import these helpers directly (full list to be re-verified during implementation):

- `spa/src/components/NetWorthCard.tsx`
- `spa/src/components/accounts/ChildAccountsCard.tsx`
- `spa/src/components/accounts/AccountDetailHeader.tsx`
- `spa/src/components/accounts/AccountSearchResults.tsx`
- `spa/src/components/accounts/RecentTransactionsCard.tsx`
- `spa/src/components/transactions/TransactionRow.tsx`
- `spa/src/components/reports/KpiCard.tsx`
- `spa/src/components/reports/CurrencyFooter.tsx`
- `spa/src/components/reports/ProportionBar.tsx`
- `spa/src/components/reports/ReportRowTable.tsx`
- `spa/src/routes/reports.balance-sheet.tsx`
- `spa/src/routes/reports.net-worth.tsx`
- `spa/src/routes/transactions.$id.index.tsx`

Each callsite changes from a module-level import call to a hook read inside the component body:

```tsx
// before
import { formatCents } from '@/lib/format';
// …
{formatCents(amount, currency)}

// after
import { useAmountFormat } from '@/lib/server-config';
// …
const { formatCents } = useAmountFormat();
{formatCents(amount, currency)}
```

`spa/src/lib/format.ts` remains exported and callable directly — it stays the implementation, and the hook is only sugar for components that need the config-bound variant. Tests that test the formatter in isolation (`format.test.ts`) continue to import from `lib/format`.

## Testing

### Go side

**`internal/config/config_test.go`** — add a case asserting `NewDefault().Display.HideDecimals == false` and (if the existing test reads YAML through viper) a case where `display.hide_decimals: true` unmarshals correctly into `Config.Display.HideDecimals`.

**`internal/api/config_test.go`** — extend the existing table to cover both display states:

```go
tests := []struct {
    name         string
    currency     string
    hideDecimals bool
    want         string
}{
    {"defaults",      "USD", false, `{"defaults":{"currency":"USD"},"display":{"hide_decimals":false}}`},
    {"hide decimals", "USD", true,  `{"defaults":{"currency":"USD"},"display":{"hide_decimals":true}}`},
}
```

The harness helper that builds the test server must accept a `display.hide_decimals` argument (extend the existing `newServerForWriteWithCurrency` from the api-config-endpoint design, or add a parallel helper if the existing one is shared across many tests and harder to widen safely).

### SPA side — `spa/src/lib/format.test.ts`

Add cases covering the new option:

```ts
describe('formatCents with hideDecimals', () => {
  it('drops decimals for whole values', () => {
    expect(formatCents(260000, 'USD', { hideDecimals: true })).toBe('$2,600');
  });
  it('rounds half up to nearest', () => {
    expect(formatCents(260050, 'USD', { hideDecimals: true })).toBe('$2,601');
  });
  it('rounds half away from zero for negatives', () => {
    expect(formatCents(-260050, 'USD', { hideDecimals: true })).toBe('-$2,601');
  });
  it('does not round below half', () => {
    expect(formatCents(260049, 'USD', { hideDecimals: true })).toBe('$2,600');
  });
  it('falls back for unknown currency', () => {
    expect(formatCents(260050, 'ZZZ', { hideDecimals: true })).toBe('2601 ZZZ');
  });
});

describe('formatBalanceAbs with hideDecimals', () => {
  it('drops decimals for whole values', () => {
    expect(formatBalanceAbs(260000, { hideDecimals: true })).toBe('$2,600');
  });
  it('rounds and ignores sign', () => {
    expect(formatBalanceAbs(-260050, { hideDecimals: true })).toBe('$2,601');
  });
});
```

The exact rounding behavior at half values is what modern V8 / JavaScriptCore implement for `Intl.NumberFormat` today (half-away-from-zero). The test pins behavior so a future engine change is caught loudly; if a CI environment surfaces a different rounding mode, the implementation will need an explicit `roundingMode: 'halfExpand'` option.

### SPA side — test fixture updates

Every `/api/config` mock in `spa/src/test/**/*.test.tsx` must include the new `display` block. From the current grep:

- `spa/src/test/server-config.test.tsx`
- `spa/src/test/transactions.reconciled.test.tsx`
- `spa/src/test/reports.tab-nav.test.tsx`
- `spa/src/test/transactions.list.test.tsx`
- `spa/src/test/reports.income-breakdown.test.tsx`
- `spa/src/test/reports.income-statement.test.tsx`
- `spa/src/test/reports.net-worth.test.tsx`
- `spa/src/test/balances.test.tsx`
- `spa/src/test/transactions.form.test.tsx`
- `spa/src/test/reports.expense-breakdown.test.tsx`
- `spa/src/test/reports.balance-sheet.test.tsx`

The minimum mock payload becomes `{ defaults: { currency: '<existing>' }, display: { hide_decimals: false } }`. Where the test file uses a shared helper to build this payload (e.g., a builder in `spa/src/test/setup.tsx`), update the helper and the individual files inherit the change.

### SPA side — `spa/src/test/server-config.test.tsx`

Extend the existing provider test (if shape coverage is present) so the asserted `ServerConfig` value includes the new `display` field. No new file required.

### Manual verification

1. With `display.hide_decimals` unset (or `false`), run `kea serve` + `cd spa && npm run dev` and confirm `/transactions`, `/balances`, every `/reports/*` page, and account-detail balances still show two-decimal values exactly as before.
2. Set `display.hide_decimals: true` in `config.yaml`, restart the server, hard-reload the SPA, and confirm the same routes now show whole-unit amounts. Spot-check a transaction with a non-zero fractional cents value (e.g., `$2,600.50`) to confirm rounding to `$2,601`.

## File and Commit Plan

Single PR, two commits.

### Commit 1 — `feat(api): expose display.hide_decimals via /api/config`

- `internal/config/config.go` — add `DisplayConfig` and `Display` field; update `NewDefault`.
- `internal/config/config_test.go` — defaults + YAML unmarshal coverage.
- `internal/api/config.go` — extend `configResponse` and the handler.
- `internal/api/config_test.go` — extend the table.
- `internal/api/testhelper_test.go` — widen / add the test-server helper as needed.

### Commit 2 — `feat(spa): honor display.hide_decimals when formatting amounts`

- `spa/src/lib/types.ts` — extend `ServerConfig`.
- `spa/src/lib/format.ts` — optional `AmountFormatOptions` on both formatters.
- `spa/src/lib/format.test.ts` — new `hideDecimals: true` cases.
- `spa/src/lib/server-config.tsx` — add `useAmountFormat`.
- All SPA callsites listed above migrate to the hook.
- All SPA test fixtures that stub `/api/config` extend the payload with the `display` block.

## Acceptance Criteria

- `go test ./...` and `go build ./...` green.
- `cd spa && npm run check && npm run test && npm run build` green.
- With `display.hide_decimals: false` (default), every SPA page displays amounts identically to before this change — verified by the existing snapshot/string-match tests in `spa/src/test` continuing to pass without changes beyond the `/api/config` stub payload.
- With `display.hide_decimals: true`, the SPA renders `$2,600` for `260000` cents and `$2,601` for `260050` cents on transaction, balance, account-detail, and report pages.
- `useServerConfig` consumers compile against the new `display` field; no SPA file imports `formatCents` or `formatBalanceAbs` from `@/lib/format` outside `lib/format.test.ts` and `lib/server-config.tsx`.

## Conventions

- Go: SPDX header on every new `.go` file (`// SPDX-License-Identifier: GPL-3.0-or-later` / `// Copyright (C) 2026  Hance Chin`). No new Go files are expected for this change; existing files are extended.
- TypeScript: no SPDX header (matching existing SPA files).
- Conventional commit format. Scopes: `(api)` for the Go-side commit, `(spa)` for the SPA-side commit.
- Table-driven Go tests using stdlib + `testify` + `httptest` patterns already in `internal/api/`.
- Vitest + RTL on the SPA side.
- No `Co-Authored-By` footer on commits.
