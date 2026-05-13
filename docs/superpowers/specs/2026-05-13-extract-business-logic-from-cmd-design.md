# Extract Business Logic from cmd/ Layer

**Issue:** [#76](https://github.com/Hance08/kea/issues/76)
**Branch:** `refactor/issue-76-extract-business-logic`
**Date:** 2026-05-13

## Problem

Several pieces of business logic live in `cmd/` instead of `internal/service/` or `internal/model/`. A future HTTP API layer would need to duplicate this logic or import cmd packages (pulling in Cobra and TUI dependencies).

## Scope

Four items to extract, one item dropped:

| # | Item | Destination | Status |
|---|------|-------------|--------|
| 1 | Date range resolution | `TransactionService` | In scope |
| 2 | Report assembly (period, net worth, growth) | `TransactionService` | In scope |
| 3 | Transaction type & status parsing | `model` package | In scope |
| 4 | Date parsing with defaults | `TransactionService` | In scope |
| 5 | Account name manipulation | Dropped | UI orchestration, not business logic |

## Design

### 1. Date Range Resolution & Report Enrichment

**New type** in `internal/service/report_service.go`:

```go
type DateRangeParams struct {
    Month string // "YYYY-MM" — takes priority if set
    From  string // "YYYY-MM-DD"
    To    string // "YYYY-MM-DD"
}
```

**New exported methods** on `TransactionService`:

- `ResolveDateRange(params DateRangeParams) (start, end int64, period string, err error)` — consolidates `parseMonth`, `parseDateRange`, and the "default to current month" fallback. Pure function on params, no DB access.
- `GenerateFullIncomeStatement(ctx, DateRangeParams) (*model.ReportResult, error)` — calls `ResolveDateRange`, then `GenerateIncomeStatement`, then fetches net worth for current and previous periods, computes growth %. Returns a fully populated `ReportResult`.
- `GenerateFullIncomeBreakdown(ctx, DateRangeParams) (*model.ReportResult, error)` — resolves dates then delegates to `GenerateIncomeBreakdown`, sets `Period`. No net worth or growth calculation.
- `GenerateFullExpenseBreakdown(ctx, DateRangeParams) (*model.ReportResult, error)` — resolves dates then delegates to `GenerateExpenseBreakdown`, sets `Period`. No net worth or growth calculation.

**Moved helpers** (unexported, in `report_service.go`):
- `parseMonth(string) (int64, int64, string, error)`
- `parseDateRange(from, to string) (int64, int64, string, error)`
- `previousPeriodRange(start, end int64) (int64, int64)`
- `computeNetWorthGrowthPctMap(current, previous map[string]int64) map[string]float64`

Existing `GenerateIncomeStatement(ctx, start, end)` etc. remain unchanged for callers that already have resolved timestamps.

**cmd/ changes:** `reportRunner.run*` methods collapse to: build `DateRangeParams` from flags, call `GenerateFullX`, render. Delete `resolveDateRange`, `parseMonth`, `parseDateRange`, `previousPeriodRange`, `computeNetWorthGrowthPctMap` from `cmd/report_actions.go`.

### 2. Transaction Type & Status Parsing

**New functions** in `internal/model/types.go`:

```go
func ParseTransactionType(s string) (TransactionType, error)
```
Strict parser — accepts "expense", "income", "transfer" (case-insensitive, trimmed). Returns error for unknown values.

```go
func ParseTransactionStatus(s string) TransactionStatus
```
Accepts "pending" (case-insensitive) → `StatusPending`, everything else → `StatusCleared`.

**cmd/ changes:** Delete `parseTransactionType` and `parseStatus` from `cmd/add_actions.go`. Replace calls with `model.ParseTransactionType` and `model.ParseTransactionStatus`.

`determineMode` (fuzzy matching for TUI prompts) stays in `cmd/` — it is UI-specific.

### 3. Date Parsing with Defaults

**New method** on `TransactionService` in `internal/service/transaction_ops.go`:

```go
func (ts *TransactionService) ParseTransactionDate(dateStr string) (int64, error)
```

Empty string → `time.Now().Unix()`. Otherwise parses `model.DateFormat` ("2006-01-02") in local time.

**cmd/ changes:** Delete `parseDate` from `cmd/add_actions.go` (`addRunner.parseDate`). Replace calls with `svc.Transaction().ParseTransactionDate()`.

## Testing

### New tests

| Location | Tests |
|----------|-------|
| `internal/service/report_service_test.go` | `ResolveDateRange` — month format, date range, defaults, validation errors. `GenerateFullIncomeStatement` — verifies `Period`, `NetWorth`, `NetWorthGrowthPct` are populated. |
| `internal/model/types_test.go` | `ParseTransactionType` — valid inputs, case variations, invalid input error. `ParseTransactionStatus` — "pending", "Pending", default to cleared. |
| `internal/service/transaction_ops_test.go` | `ParseTransactionDate` — empty returns now (within tolerance), valid date, invalid format error. |

### Existing tests

`go test ./...` must pass after each extraction step. No behavioral changes — only code movement.

## Non-goals

- No new `ReportService` struct — report methods stay on `TransactionService`.
- No changes to repository interfaces or store layer.
- No changes to `model.ReportResult` or `model.BalanceSheetResult` struct definitions (they already have the fields).
- `determineMode` (fuzzy type matching) stays in cmd/ — UI concern.
- Account name manipulation (item #5) stays in cmd/ — UI orchestration.
