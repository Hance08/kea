# Monthly Budget — Design

**Date:** 2026-10-05
**Status:** Approved (brainstorming)
**Next step:** Implementation plan via `writing-plans` skill

## Problem

kea records what was spent but has no way to say what *should* be spent. The user wants to set a monthly spending limit per expense category and, at any point in the month, see how much of it is used, how much is left, and whether a category is over budget.

The user ranked three budgeting needs: (A) monthly per-category limits, (C) monthly savings targets, (B) yearly budgets. This spec covers **A only**. C and B are separate future specs; the data model below does not block either.

## Goal

Per-ledger monthly budgets on Expense accounts, with a "budget vs actual" view available from the CLI, the HTTP API, a SPA report page and a dashboard card.

Success: on any day of the month the user can see, per budgeted category, budget / actual / remaining / percent used, with regular and irregular spending separated, and spot over-budget categories at a glance.

## Non-goals (v1)

- Savings targets (C) and yearly or custom-period budgets (B).
- Rollover of unused (or overspent) amounts to the next month.
- Per-month overrides on top of a versioned amount.
- Budgets on Revenue, Asset or Liability accounts.
- Counting Expense splits inside non-`Expense` transactions (fees in `Transfer` or `Investment`).
- Changing the existing reports. They take `AbsInt64` of each split, so refunds inflate expense totals; that is tracked as a separate task. Budgets use signed sums from day one.

## Decisions

| # | Question | Decision |
|---|---|---|
| 1 | Period | Monthly only. Amounts are versioned: a budget row applies from its `effective_month` until a later row for the same account replaces it. |
| 2 | Account level | Any Expense (`E`) account, leaf or parent. A parent's actual includes all descendants **in the same currency**. A parent and its children may each have independent budgets; no consistency check between them. |
| 3 | Currency | A budget's currency is its account's currency (empty means `config.Defaults.Currency`). Not stored on the budget; account currency cannot change after creation. Descendant splits in other currencies are excluded and reported. |
| 4 | Rollover | None in v1. Each month stands alone. |
| 5a | Transaction scope | Same as the expense report: only Expense splits of `Expense`-typed transactions. `Investment`, `Transfer`, `Opening` etc. never count. |
| 5b | Refunds | Signed sum. A negative Expense split (refund) reduces actual. Actual may be negative. |
| 5c | Regular | Both regular and irregular count; actual is also split into `ActualRegular` / `ActualIrregular`. |
| 6 | Interfaces | CLI, HTTP API, SPA settings page, SPA report page and dashboard card, all in v1. |
| 7 | Storage | New table in each ledger's DB, migration `0012`. |

## Data model

### Migration `0012_create_budgets`

Up:

```sql
CREATE TABLE budgets (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id      INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    effective_month TEXT    NOT NULL CHECK (effective_month GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'),
    amount          INTEGER NOT NULL CHECK (amount >= 0),
    stopped         INTEGER NOT NULL DEFAULT 0 CHECK (stopped IN (0, 1)),
    CHECK (stopped = 0 OR amount = 0),
    UNIQUE (account_id, effective_month)
);

CREATE INDEX IF NOT EXISTS idx_budgets_account_month ON budgets (account_id, effective_month);
```

Down: `DROP INDEX` and `DROP TABLE budgets`.

The GLOB only guards the shape; the service validates the month fully (`01`–`12`).

### Semantics

- A row is a version: "from `effective_month` on, this account's monthly budget is `amount`".
- The budget for account X in month M is the row with the greatest `effective_month <= M` for X. If that row has `stopped = 1`, or no such row exists, X has no budget in M.
- Setting a budget for an (account, month) that already has a row **replaces** that row (upsert).
- `amount = 0` is a real budget ("spend nothing"), distinct from no budget.
- Budgets reference `account_id`, so renames are transparent. Deleting an account cascades to its budgets (an account with transactions cannot be deleted anyway).
- Hidden accounts keep their budgets and still appear in reports.
- The service rejects non-Expense accounts; SQLite cannot check that across tables.

### Model (`internal/model/budget.go`)

```go
type Budget struct {
    ID             int64  `json:"id"`
    AccountID      int64  `json:"account_id"`
    AccountName    string `json:"account_name"`
    EffectiveMonth string `json:"effective_month"` // YYYY-MM
    Amount         int64  `json:"amount"`          // cents
    Stopped        bool   `json:"stopped"`
}

type SetBudgetInput struct {
    AccountName    string `json:"account_name"`
    EffectiveMonth string `json:"effective_month"`
    Amount         int64  `json:"amount"`
}

type StopBudgetInput struct {
    AccountName    string `json:"account_name"`
    EffectiveMonth string `json:"effective_month"`
}

type BudgetReport struct {
    Month       string            `json:"month"`
    Rows        []BudgetReportRow `json:"rows"`
    TotalBudget map[string]int64  `json:"total_budget"` // per currency
    TotalActual map[string]int64  `json:"total_actual"` // per currency
}

type BudgetReportRow struct {
    AccountID        int64    `json:"account_id"`
    AccountName      string   `json:"account_name"`
    Currency         string   `json:"currency"`
    EffectiveMonth   string   `json:"effective_month"` // version that applied
    Budget           int64    `json:"budget"`
    Actual           int64    `json:"actual"`
    ActualRegular    int64    `json:"actual_regular"`
    ActualIrregular  int64    `json:"actual_irregular"`
    Remaining        int64    `json:"remaining"` // Budget - Actual, may be negative
    ExcludedAccounts []string `json:"excluded_accounts"`
}
```

`Rows` and `ExcludedAccounts` are never `nil` (serialize as `[]`). Percent used is not part of the model; CLI and SPA compute it for display.

## Repository (`internal/repository/interfaces.go`)

```go
type BudgetRepository interface {
    UpsertBudget(ctx context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error)
    ListBudgets(ctx context.Context) ([]model.Budget, error) // all versions, joined with accounts for AccountName
    DeleteBudget(ctx context.Context, id int64) error        // missing id -> repository.ErrNotFound
}
```

`BudgetRepository` is added to the composite `Repository` passed to `ExecTx`. Selecting the effective version is a business rule and lives in the service; the full budget list is small.

Store implementation: `internal/store/sqlite_budget.go`, using `ExecContext` / `QueryContext` only. `UpsertBudget` uses `INSERT ... ON CONFLICT(account_id, effective_month) DO UPDATE` and returns the row id.

## Service (`internal/service/budget_service.go`, `internal/service/budget_report.go`)

New `BudgetService`, exposed as `svc.Budget()` on the facade. Dependencies: `AccountRepository`, `TransactionRepository`, `BudgetRepository`, `TransactionManager`, `*config.Config`. `NewService` and `app.NewApp` pass the same `*store.Store` for the new repository.

| Method | Behavior |
|---|---|
| `SetBudget(ctx, model.SetBudgetInput) (*model.Budget, error)` | Validates month (`YYYY-MM`, month 01–12) and `amount >= 0` (`ValidationError`). Inside `ExecTx`: account not found -> `ErrNotFound`; account type not `E` -> `ValidationError{Field: "account_name"}`; then upsert. |
| `StopBudget(ctx, model.StopBudgetInput) (*model.Budget, error)` | Same validation. Inside `ExecTx`: if the account has no active budget in that month -> `ValidationError`; otherwise upsert `stopped = 1, amount = 0`. |
| `ListBudgets(ctx) ([]model.Budget, error)` | All versions, sorted by account name, then month. |
| `DeleteBudget(ctx, id int64) error` | `repository.ErrNotFound` -> `ErrNotFound`. |
| `GenerateBudgetReport(ctx, month string) (*model.BudgetReport, error)` | See below. |

Repository errors are translated to service errors. No new service sentinel is needed, so `mapError` is unchanged; if implementation turns one up, it gets a `mapError` case.

### `GenerateBudgetReport`

1. Empty `month` means the current month. Resolve `[start, end]` with the existing `parseMonth`.
2. Load all budgets and pick each account's effective version for the month; drop stopped ones.
3. Load all accounts (for name, type and currency of descendants).
4. Load the month's splits and transactions with the repository methods `buildReportMaps` uses. Keep only Expense-account splits of `Expense`-typed transactions.
5. For each budgeted account N with currency C, over the kept splits whose account name is N or starts with `N + ":"`:
   - currency C: add the **signed** amount to `Actual`, and to `ActualRegular` or `ActualIrregular` by the transaction's `Regular` flag;
   - other currency: add the account name to `ExcludedAccounts` (deduplicated, sorted). Only accounts that actually had such splits that month are listed.
6. `Remaining = Budget - Actual`. Rows sorted by account name.
7. Totals per currency sum only **top-level budgeted rows**: rows with no budgeted ancestor in the same report. This prevents double counting when a parent and its child both have budgets.

## HTTP API (`internal/api/budgets.go`)

| Method | Path | Request | Response |
|---|---|---|---|
| `GET` | `/api/budgets` | — | `200 {"items": [Budget]}` |
| `PUT` | `/api/budgets` | `SetBudgetInput` | `200 Budget` (idempotent upsert) |
| `POST` | `/api/budgets/stop` | `StopBudgetInput` | `200 Budget` |
| `DELETE` | `/api/budgets/{id}` | — | `200 {"deleted": true, "id": <id>}` |
| `GET` | `/api/reports/budget` | `?month=YYYY-MM` (default current month) | `200 BudgetReport` |

- DELETE matches the other DELETE endpoints; `apiFetch` in the SPA always parses a JSON body.
- Bodies use `decodeJSON` (unknown fields rejected).
- Errors: `ValidationError` -> 400 with `field`; `ErrNotFound` -> 404.
- Amounts are integer cents, like every other endpoint.

## CLI (`cmd/budget/`)

New group `NewBudgetCmd`, registered in `cmd/root.go` after `app.NewApp`. Each command depends on a narrow provider interface.

| Command | Notes | Flag pattern |
|---|---|---|
| `kea budget set [<account> <amount>] [--month YYYY-MM] [--json]` | With args: flag mode. No args: huh wizard (Expense account picker, amount, month defaulting to current). Amount via `utils.ParseAmount`. | C |
| `kea budget stop <account> [--month] [--json]` | Stop from a month (default current). | B |
| `kea budget list [--account <name>] [--json]` | All versions: ID, account, month, amount or `stopped`. | B |
| `kea budget delete <id> [--yes] [--json]` | Confirms without `--yes`; `--json` implies `--yes`. | B |
| `kea budget report [--month YYYY-MM] [--json]` | Budget vs actual. | B |

`--month` defaults to the current month everywhere.

### Report view (`ui/views/budget_report.go`, tablewriter)

```
Budget report  2026-10
Account                  Budget     Actual    Regular  Irregular  Remaining  Used
Expenses:Food          10,000.00   7,320.50      0.00   7,320.50   2,679.50   73%
  Expenses:Food:Dining  6,000.00   6,410.00      0.00   6,410.00    -410.00  107%
Expenses:Housing       20,000.00  18,500.00  18,000.00     500.00   1,500.00   93%
Total (TWD)            30,000.00  25,820.50  18,000.00   7,820.50   4,179.50   86%
! Expenses:Food: 1 sub-account excluded (different currency): Expenses:Food:Japan
```

- Children are indented by depth relative to the shallowest budgeted ancestor shown.
- Used >= 80% is yellow, > 100% red; no color with `--no-color`. Used is blank when Budget is 0 and Actual is 0; it shows `over` when Budget is 0 and Actual > 0.
- One total line per currency (top-level rows only).

### JSON

DTOs in `ui/views/json_types.go` (`JSONBudget`, `JSONBudgetReport`, `JSONBudgetReportRow`) convert cents with `CentsToUnit`, following the existing CLI JSON decision.

## SPA

API client `spa/src/lib/api/budgets.ts`, types in `spa/src/lib/types.ts`.

### Budgets settings page `/budgets`

New top-level nav item "Budgets".

- Month picker (default current month). Table of budgets **effective in that month**: account (indented by depth), currency, amount, effective month, with Edit and Stop actions.
- Edit asks "effective from" (default: the selected month) and sends `PUT /api/budgets`, creating a new version instead of rewriting history.
- "Add budget" form: Expense-only account picker (reusing the existing account picker), amount, effective month.
- Each account expands into its version history; each version can be deleted after confirmation.
- 400 errors are shown next to the field named in `field`.

### Budget report `/reports/budget`

New "Budget" tab in the Reports `TabNav`.

- Month from the URL, using `reports-search-params` (persisted per ledger in `localStorage`). Only `month`; no from/to.
- Each row: account (indented), budget, actual with a regular/irregular sub-line (`reportSubLine`), remaining, and a progress bar: < 80% normal, 80–100% warning, > 100% danger.
- For the current month, a vertical "time elapsed" marker on each bar at (days elapsed / days in month), computed client-side.
- Rows with `excluded_accounts` show a warning icon; its tooltip lists the excluded accounts.
- Per-currency total rows. Empty state with a link to `/budgets` when the month has no budgets.

### Dashboard card `budget-progress`

Registered in `spa/src/lib/dashboard/registry.ts` and added to the default layout. Existing saved layouts get it automatically: `reconcileWithRegistry` appends unknown widgets as visible.

- Always the current month. Top N budgets by percent used, descending: account name, mini progress bar, percent. Clicking goes to `/reports/budget`.
- `ConfigForm`: N (default 5) and "only show >= 80%" (default off).
- Empty state "No budgets yet" with a link to `/budgets`.

## Testing

| Layer | Tests |
|---|---|
| Migration | `internal/store/migration_0012_test.go`: CHECKs (month shape, amount >= 0, stopped implies amount 0), UNIQUE, CASCADE on account delete, down migration. |
| Store | `internal/store/sqlite_budget_test.go` with `setupTestDB`: upsert insert and replace, list join and order, delete and not-found. |
| Service | `internal/service/budget_service_test.go`, `budget_report_test.go` with in-memory mocks in `testhelper_test.go`: validation, non-Expense account, not found, stop without active budget, version selection (before first version, between versions, after stop, restart after stop), descendant aggregation, currency exclusion, signed refunds, regular split, non-Expense transaction types ignored, top-level-only totals, empty month. |
| API | `internal/api/budgets_test.go`: each endpoint's success path, 400 with `field`, 404, unknown fields rejected, report default month. |
| CLI | Runner tests with mock providers for each command (flag mode, error propagation, `--json`); view tests for indentation, color thresholds, per-currency totals, exclusion notes. |
| SPA | Vitest: API client; report page (thresholds, time marker only in current month, exclusion tooltip, totals, empty state); settings page (add, edit as new version, stop, delete version, field errors); dashboard card (sort, N, >= 80% filter, empty state); registry includes the new widget. |

## Documentation

Updated in the same commits as the code, then `scripts/check-docs.sh`:

- `docs/domain.md`: new "Budgets" section (versioning, account level, currency rule, signed sums, transaction scope, top-level totals).
- `docs/http-api.md`: Budgets endpoints and `/reports/budget`.
- `docs/architecture.md`: `BudgetService` on the facade, `BudgetRepository`, package map rows for `cmd/budget`.
- `docs/decisions.md`: entries for "Budgets are versioned by effective month" and "Budgets use signed sums".
- `SKILL.md`: `kea budget` commands.
- `spa/README.md`: new routes.

## Future work

- C: monthly savings target per currency (income minus expense).
- B: yearly budgets; revisit rollover there.
- Per-month overrides on versioned budgets.
- Optional inclusion of Expense splits from non-`Expense` transactions.
