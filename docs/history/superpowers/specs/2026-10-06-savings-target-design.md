# Savings Target — Design

**Date:** 2026-10-06
**Status:** Approved (brainstorming)
**Next step:** Implementation plan via `writing-plans` skill

## Problem

kea can say what was spent and, since the monthly budget feature (PR #223), what *should* be spent. It cannot say whether the user is saving what they intend to. The user keeps a dedicated savings account (`Assets:Banks:Cube_Saving`) and wants to set a monthly amount to move into it, then see each month, and year to date, whether that happened.

This is phase C of the budgeting roadmap (A monthly budgets — done; C savings targets — this spec; B yearly budgets — later). The A spec's Future work described C as "income minus expense"; this design deliberately measures the savings account instead (see Decisions #1).

## Goal

Per-ledger monthly savings targets on Asset accounts, with a "target vs saved" view, month and year to date, available from the CLI, the HTTP API, a SPA report page and a dashboard card.

Success: on any day the user can see, per savings account, the month's target, how much the account has grown this month, how much is still missing (or exceeded), and the same comparison accumulated over the year so far. The numbers reconcile with the account's balance change (apart from excluded `Opening` transactions).

## Non-goals (v1)

- "Income minus expense" or net-worth-change definitions of saving.
- Savings-rate targets (percentage of income), and showing a savings rate.
- Rollover of a shortfall or surplus into the next month's target.
- One target spanning several accounts; each account gets its own target.
- Regular / irregular subtotals (the `Regular` flag only exists on Income/Expense transactions).
- Fixing the income statement's `AbsInt64` refund handling. That is a separate task.
- Yearly budgets (B).

## Decisions

| # | Question | Decision |
|---|---|---|
| 1 | What is "saved" | The **net balance change** of the target account and its descendants in the month: the signed sum of all their splits, whatever the transaction type. Transfers in count +, transfers out −, interest +, payments from the account −. Transfers between the account and its own descendants net to zero. |
| 2 | Excluded transactions | `Opening`-typed transactions never count: an opening balance is existing money, not money saved that month. Every other type counts. In a month with an `Opening` transaction, Saved therefore differs from the balance change by that amount. |
| 3 | Refunds / signs | Signed sums (consequence of #1). The income-statement refund issue is unrelated and out of scope. |
| 4 | Binding | One target per account. Any Asset (`A`) account, leaf or parent. Several savings accounts means several targets. |
| 5 | Form | A fixed monthly amount in cents, `>= 0`. `0` is a real target ("do not draw the account down"). |
| 6 | Versioning | Same as budgets: a row applies from its `effective_month` until a later row for the same account; upsert per (account, month); stop writes `stopped = 1, amount = 0`. |
| 7 | Currency | The account's currency (empty means `config.Defaults.Currency`). Descendant splits in another currency are excluded and reported. Totals are per currency, never summed across currencies. |
| 8 | Progress | Each month is judged on its own (no rollover). The report also gives year-to-date figures and a per-month breakdown, counting only months in which the target was active. |
| 9 | Remaining | `Remaining = Target - Saved`, signed. Positive means still missing; negative means exceeded. No separate shortfall/achieved fields. |
| 10 | Storage / structure | New table `savings_targets` (migration `0013`), new `SavingsTargetRepository`, new `SavingsService`. Version-selection helpers are extracted from `BudgetService` into generic helpers shared by both. |
| 11 | Interfaces | All in v1: CLI `kea savings`, HTTP API, SPA settings page, SPA report tab, dashboard card. |

## Data model

### Migration `0013_create_savings_targets`

```sql
-- Monthly savings targets on Asset accounts. Each row is a version that
-- applies from effective_month (YYYY-MM) until a later row for the same account.
CREATE TABLE IF NOT EXISTS savings_targets (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id      INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    effective_month TEXT    NOT NULL CHECK (effective_month GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'),
    amount          INTEGER NOT NULL CHECK (amount >= 0),
    stopped         INTEGER NOT NULL DEFAULT 0 CHECK (stopped IN (0, 1)),
    CHECK (stopped = 0 OR amount = 0),
    UNIQUE (account_id, effective_month)
);

CREATE INDEX IF NOT EXISTS idx_savings_targets_account_month ON savings_targets (account_id, effective_month);
```

`0013_create_savings_targets.down.sql` drops the index and the table.

### Semantics

- The target for account X in month M is the row with the greatest `effective_month <= M` for X. If that row is stopped, or none exists, X has no target in M.
- Targets reference `account_id`, so renames are transparent; deleting an account cascades.
- Hidden accounts keep their targets and still appear in reports.
- The service rejects non-Asset accounts; SQLite cannot check that across tables.

### Model (`internal/model/savings.go`)

```go
// SavingsTarget is one version of a monthly savings target on an Asset account.
type SavingsTarget struct {
	ID             int64  `json:"id"`
	AccountID      int64  `json:"account_id"`
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
	Amount         int64  `json:"amount"` // cents
	Stopped        bool   `json:"stopped"`
}

type SetSavingsTargetInput struct {
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
	Amount         int64  `json:"amount"` // cents, >= 0
}

type StopSavingsTargetInput struct {
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
}

// SavingsReport compares each target active in Month with what was saved,
// for the month and year to date. Totals are per currency over rows with no
// targeted ancestor in the same currency.
type SavingsReport struct {
	Month          string             `json:"month"`
	Rows           []SavingsReportRow `json:"rows"`
	TotalTarget    map[string]int64   `json:"total_target"`
	TotalSaved     map[string]int64   `json:"total_saved"`
	TotalYTDTarget map[string]int64   `json:"total_ytd_target"`
	TotalYTDSaved  map[string]int64   `json:"total_ytd_saved"`
}

type SavingsReportRow struct {
	AccountID        int64          `json:"account_id"`
	AccountName      string         `json:"account_name"`
	Currency         string         `json:"currency"`
	EffectiveMonth   string         `json:"effective_month"`
	Target           int64          `json:"target"`
	Saved            int64          `json:"saved"`
	Remaining        int64          `json:"remaining"` // Target - Saved; negative = exceeded
	YTDTarget        int64          `json:"ytd_target"`
	YTDSaved         int64          `json:"ytd_saved"`
	YTDRemaining     int64          `json:"ytd_remaining"`
	Months           []SavingsMonth `json:"months"` // active months, Jan..Month, ascending
	ExcludedAccounts []string       `json:"excluded_accounts"`
}

type SavingsMonth struct {
	Month  string `json:"month"`
	Target int64  `json:"target"`
	Saved  int64  `json:"saved"`
}
```

Slices and maps are never `nil` in responses.

## Repository (`internal/repository/interfaces.go`)

```go
// SavingsTargetRepository stores savings target versions. Selecting the
// version that applies to a month is service logic.
type SavingsTargetRepository interface {
	UpsertSavingsTarget(ctx context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error)
	ListSavingsTargets(ctx context.Context) ([]model.SavingsTarget, error) // joined with account name, ordered by name then month
	DeleteSavingsTarget(ctx context.Context, id int64) error               // missing id wraps ErrNotFound
}
```

Embedded in `repository.Repository` so it is usable inside `ExecTx`. Implemented in `internal/store/sqlite_savings_target.go`.

## Service

### Shared version helpers (`internal/service/versions.go`)

`latestVersions`, `activeBudgets`, `hasActiveBudget` and `hasBudgetedAncestorInCurrency` are currently typed on `model.Budget` / `model.BudgetReportRow`. They become generic helpers over a small accessor interface (account id, effective month, stopped, id; and account name + currency for the ancestor check), used by both services. `BudgetService` behavior does not change; its existing tests must pass unmodified.

### `SavingsService` (`internal/service/savings_service.go`, `savings_report.go`)

Exposed as `svc.Savings()`. Dependencies: `AccountRepository`, `TransactionRepository`, `SavingsTargetRepository`, `TransactionManager`, `*config.Config`.

| Method | Behavior |
|---|---|
| `SetSavingsTarget(ctx, model.SetSavingsTargetInput)` | Blank name -> `ValidationError{account_name}`; month via `ValidateBudgetMonth` (`effective_month`); `amount < 0` -> `ValidationError{amount}`. Inside `ExecTx`: unknown account -> `ErrNotFound`; type not `A` -> `ValidationError{account_name}`; upsert. |
| `StopSavingsTarget(ctx, model.StopSavingsTargetInput)` | Same validation. Inside `ExecTx`: no active target in that month -> `ValidationError{effective_month}`; else upsert stopped. |
| `ListSavingsTargets(ctx)` | All versions, never `nil`. |
| `DeleteSavingsTarget(ctx, id)` | `repository.ErrNotFound` -> `ErrNotFound`. |
| `GenerateSavingsReport(ctx, month)` | See below. |

No new service sentinel; `mapError` is unchanged. If implementation needs one, it gets a `mapError` case.

### `GenerateSavingsReport`

1. Empty `month` means the current month in local time. Invalid -> `ValidationError{month}`.
2. Load all targets; rows are the targets active in `month`. No active target -> empty report.
3. Load all accounts. Load splits (`GetSplitsWithAccountsByDateRange`) and transactions (`GetTransactionsByDateRange`) once, from local Jan 1 of `month`'s year to the end of `month`. Drop `Opening`-typed transactions.
4. Bucket each split by its transaction's local `YYYY-MM`.
5. For each row account N with currency C, per month bucket, over splits whose account is N or a descendant (`isSelfOrDescendant`): currency C -> add the signed amount; other currency -> record the account in `ExcludedAccounts` (only for splits within the selected month, deduplicated, sorted).
6. `Target`, `Saved`, `Remaining` for `month`. YTD window: Jan of `month`'s year through `month`; for each month in it where the account's target is active (per version selection), append a `SavingsMonth{month, target, saved}` and add to `YTDTarget` / `YTDSaved`. `YTDRemaining = YTDTarget - YTDSaved`.
7. Rows sorted by account name. Totals per currency over rows with no targeted ancestor in the same currency (shared helper).

## HTTP API (`internal/api/savings.go`)

| Method | Path | Request | Response |
|---|---|---|---|
| `GET` | `/api/savings-targets` | — | `200 {"items": [SavingsTarget]}` |
| `PUT` | `/api/savings-targets` | `SetSavingsTargetInput` | `200 SavingsTarget` (idempotent upsert) |
| `POST` | `/api/savings-targets/stop` | `StopSavingsTargetInput` | `200 SavingsTarget` |
| `DELETE` | `/api/savings-targets/{id}` | — | `200 {"deleted": true, "id": <id>}` |
| `GET` | `/api/reports/savings` | `?month=YYYY-MM` (default current local month) | `200 SavingsReport` |

Bodies use `decodeJSON` (unknown fields rejected). Errors: `ValidationError` -> 400 with `field` (`account_name`, `effective_month`, `amount`, `month`, `body`); `ErrNotFound` -> 404. Amounts are integer cents.

## CLI (`cmd/savings/`)

New group `NewSavingsCmd`, registered next to `kea budget`. Each command depends on a narrow provider interface. Flag patterns follow `cmd/budget/`.

| Command | Notes |
|---|---|
| `kea savings set [<account> <amount>] [--month YYYY-MM] [--json]` | With args: flag mode. No args: huh wizard (Asset account picker, amount, month defaulting to current). Amount via `utils.ParseAmount`. |
| `kea savings stop <account> [--month] [--json]` | Stop from a month (default current). |
| `kea savings list [--account <name>] [--json]` | All versions: ID, account, month, amount or `stopped`. |
| `kea savings delete <id> [--yes] [--json]` | Confirms without `--yes`; `--json` implies `--yes`. |
| `kea savings report [--month YYYY-MM] [--json]` | Target vs saved, month and YTD. |

### Report view (`ui/views/savings.go`, tablewriter)

```
Savings report  2026-10
Account                     Target      Saved   Remaining   YTD target   YTD saved   YTD remaining
Assets:Banks:Cube_Saving  15,000.00  12,000.00    3,000.00   150,000.00  156,500.00      -6,500.00
Total (TWD)               15,000.00  12,000.00    3,000.00   150,000.00  156,500.00      -6,500.00
! Assets:Banks:Cube_Saving: 1 sub-account excluded (different currency): Assets:Banks:Cube_Saving:USD
```

- Positive Remaining is yellow (still missing), zero or negative is green; no color with `--no-color`.
- Children indented by depth, as in the budget view. One total line per currency.
- JSON DTOs in `ui/views/json_types.go` (`JSONSavingsTarget`, `JSONSavingsReport`, `JSONSavingsReportRow`, `JSONSavingsMonth`) convert cents with `CentsToUnit`.

## SPA

API client `spa/src/lib/api/savings.ts`, hooks `spa/src/lib/hooks/useSavings.ts`, helpers `spa/src/lib/savings.ts`, types in `spa/src/lib/types.ts`. "Current month" is never computed from UTC: pages omit `month` and let the server resolve it, or compute it in local time.

### Settings page `/savings`

New nav item "Savings" after "Budgets". Same layout as `/budgets`: month picker, targets effective in that month with Edit (new version from a chosen month) and Stop, "Add target" form with an Asset-only account picker, per-account version history with delete, 400 errors shown next to the named field.

### Report tab `/reports/savings`

New "Savings" tab in the Reports `TabNav`; month from the URL via `parseMonthSearch`, remembered per ledger (`filter-memory` key `reports/savings`).

- Per target: account, target, saved, remaining text ("3,000 to go" / "6,500 over target"), and a `SavingsProgressBar`.
- YTD line under each target (target, saved, remaining) and a compact per-month table or bar list from `months`.
- Excluded-accounts warning icon with tooltip; per-currency total rows; empty state linking to `/savings`.

### `SavingsProgressBar` (`spa/src/components/savings/`)

A new component; `BudgetProgressBar` is not changed. Semantics are inverted from budgets: Saved >= Target is success; for the current month, Saved / Target below the elapsed fraction of the month is warning; Saved < 0 is danger. Target 0 with Saved >= 0 is success.

### Dashboard card `savings-progress`

Title "Savings This Month". Fetches the report without `month` (server decides). One line per active target: account, mini `SavingsProgressBar`, saved / target, YTD remaining. Clicking goes to `/reports/savings`. Empty state links to `/savings`. No config form in v1. Registered in `spa/src/lib/dashboard/registry.tsx`, added to `WidgetId` and the default layout next to `budget-progress` (`x: 1, y: 5, w: 1, h: 2`); saved layouts get it via `reconcileWithRegistry`.

## Testing

| Layer | Tests |
|---|---|
| Migration | `internal/store/migration_0013_test.go`: CHECKs, UNIQUE, CASCADE, down migration. |
| Store | `internal/store/sqlite_savings_target_test.go` with `setupTestDB`: upsert insert/replace, list join and order, delete and not-found. |
| Service | `versions_test.go` for the generic helpers; existing budget tests unchanged and green. `savings_service_test.go`: validation, non-Asset account, not found, zero target, stop without active target. `savings_report_test.go`: signed in/out, interest, descendants, intra-subtree transfer nets to 0, `Opening` excluded, other-currency exclusion, YTD only over active months (stop then restart), target starting after January, same-currency ancestor totals, local-time month boundaries, empty report. |
| API | `internal/api/savings_test.go`: each endpoint's success path, 400 with `field`, 404, unknown fields, report default month. |
| CLI | Runner tests with mock providers per command (flag mode, error propagation, `--json`); view tests for colors, indentation, totals, exclusion notes. |
| SPA | Vitest: API client; settings page; report tab (remaining wording, YTD, exclusion tooltip, empty state); `SavingsProgressBar` states; dashboard card (no month param, empty state); registry includes the widget. |

## Documentation

Updated in the same commits as the code, then `scripts/check-docs.sh`:

- `docs/domain.md`: new "Savings targets" section (definition, `Opening` exclusion, versioning, currency, YTD, no rollover).
- `docs/http-api.md`: Savings endpoints and `/reports/savings`.
- `docs/architecture.md`: `SavingsService`, `SavingsTargetRepository`, `cmd/savings`, shared version helpers.
- `docs/decisions.md`: "Savings are measured as the target account's balance change", "Opening transactions do not count as saving", "Savings targets have their own table".
- `SKILL.md`: `kea savings` commands.
- `spa/README.md`: new routes.

## Implementation notes

- After adding SPA routes, run `npx vite build` to regenerate `spa/src/routeTree.gen.ts` before `npm run build` (which runs `tsc` first).
- Builds modify `internal/web/dist/index.html`; do not commit it.
- Six `spa/src/test/balances*.test.tsx` tests failed on master before this work; if still failing, treat them as the baseline.
- Rebuild the `kea` binary: an old binary refuses to start on a ledger with migration `0013`. Remind the user to re-run the install script on other hosts.

## Future work

- Savings-rate display or targets (saved / income).
- Targets spanning several accounts.
- Optional rollover of shortfalls.
- Yearly budgets (B).
