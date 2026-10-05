# Design: Per-Currency Opening Balances

**Date:** 2026-04-01
**Status:** Draft

---

## Problem

`createOpeningBalance` in `internal/service/account_ops.go` always uses
`config.Defaults.Currency` for both splits, regardless of the new account's
own currency. If a user creates a TWD account with an opening balance while
the system default is USD, both splits are recorded as USD — silently wrong
data.

Additionally, the single `Equity:OpeningBalances` account cannot hold
multi-currency splits and still display a meaningful balance, because
`GetAccountBalance` does a plain `SUM(amount)` with no currency awareness.

---

## Goals

1. Opening balance splits are recorded in the account's actual currency.
2. `Equity:OpeningBalances_*` accounts are always single-currency, so their
   balances remain meaningful.
3. Existing users' data is migrated automatically with no manual steps.

---

## Non-Goals

- Full multi-currency transaction support (FX rates, currency conversion).
- Changing how `ValidateSplitsBalance` works (currency-aware balance
  validation is a separate concern).

---

## Design

### Naming Convention

Replace the single `Equity:OpeningBalances` system account with
per-currency sibling accounts under `Equity`:

```
Equity:OpeningBalances_USD
Equity:OpeningBalances_TWD
Equity:OpeningBalances_JPY
...
```

The underscore keeps these as leaf accounts (not parent-child), avoiding
complications with the rule that parent accounts cannot hold transactions.

The helper for deriving the name:

```go
func OpeningBalancesAccountName(currency string) string {
    return "Equity:OpeningBalances_" + strings.ToUpper(currency)
}
```

---

### 1. initWizard

When the init wizard creates the initial data set, it creates
`Equity:OpeningBalances_<default_currency>` (e.g. `Equity:OpeningBalances_USD`)
instead of the old `Equity:OpeningBalances`.

The `SystemAccountOpeningBalance` constant in `internal/model/types.go` is
removed and replaced with the helper function above.

---

### 2. createOpeningBalance

The function resolves the equity counterpart account by currency:

```
currency = account.Currency
if currency == "" {
    currency = config.Defaults.Currency
}
equityAccountName = OpeningBalancesAccountName(currency)
```

Look up `equityAccountName`. If it does not exist, create it automatically
(type `C`, same currency, no description, no parent prompt to the user).
Then create the two splits both in `currency`.

The user sees no extra prompt — account creation is transparent.

---

### 3. Protection Logic

The current hard-coded check against `"Equity:OpeningBalances"` is replaced
with a prefix check:

```go
func IsOpeningBalancesAccount(name string) bool {
    return strings.HasPrefix(name, "Equity:OpeningBalances_")
}
```

`DeleteAccountByName` and any other guard that references the old constant
switches to this helper. All `Equity:OpeningBalances_*` accounts become
undeletable system accounts.

---

### 4. Startup Migration (existing users)

On app startup, after DB migrations run, execute a one-time Go-level
migration in `internal/app/app.go`:

```
if account "Equity:OpeningBalances" exists
   AND account "Equity:OpeningBalances_<default_currency>" does NOT exist:
     UPDATE accounts SET name = "Equity:OpeningBalances_<default_currency>"
     WHERE name = "Equity:OpeningBalances"
```

**Why this is safe:** the old code always used `config.Defaults.Currency`
for every split in `Equity:OpeningBalances`, so all existing splits are
already in the default currency. The rename is a no-op from a data
correctness standpoint.

This logic runs once and becomes a no-op on subsequent startups once the
old name no longer exists.

---

## Affected Files

| File | Change |
|------|--------|
| `internal/model/types.go` | Remove `SystemAccountOpeningBalance` constant; add `OpeningBalancesAccountName` helper and `IsOpeningBalancesAccount` helper |
| `internal/service/account_ops.go` | Update `createOpeningBalance` to use per-currency account; update `DeleteAccountByName` guard |
| `internal/app/app.go` | Add startup migration: rename legacy account |
| `ui/` (initWizard) | Create `Equity:OpeningBalances_<currency>` instead of `Equity:OpeningBalances` |
| `internal/service/account_ops_test.go` | Update tests to use new account name pattern |

---

## Test Cases to Add / Update

- `createOpeningBalance` for account whose currency matches default → uses
  existing `Equity:OpeningBalances_<default>`.
- `createOpeningBalance` for account whose currency differs from default →
  auto-creates `Equity:OpeningBalances_TWD`, uses it.
- `createOpeningBalance` for account with empty currency → falls back to
  default currency account.
- `DeleteAccountByName` rejects deletion of any `Equity:OpeningBalances_*`
  account.
- Startup migration: old `Equity:OpeningBalances` is renamed correctly when
  no per-currency account exists yet.
- Startup migration: no-op when per-currency account already exists.

---

## Open Questions

None — all design decisions resolved in discussion.
