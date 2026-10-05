# Report Regular/Irregular Subtotals — Design

**Date:** 2026-06-25
**Status:** Approved (brainstorming)
**Builds on:** [Regular Transaction Attribute spec](2026-06-25-regular-transaction-attribute-design.md)

## Problem

The three SPA report pages — Income Statement, Income Breakdown, Expense Breakdown — show `Total Income` and `Total Expense` KPI cards for the selected period. Users now want to see how much of that total is *regular* (habitual income/expense) vs *irregular* (one-off) at a glance, without applying a filter and re-loading the report.

## Goal

For every report page that shows a Total Income or Total Expense KPI card, show a sub-line under the figure with the regular and irregular subtotals for the selected period and currency. Example:

```
Total Income
$5,300
Regular $5,000 · Irregular $300
```

## Non-goals

- No filter UI — this is a passive breakdown, not an additional filter.
- No per-row regular/irregular split in the breakdown tables.
- No CLI change (`kea report --regular` is a separate, already-deferred follow-up).
- No new API endpoints; piggyback on the existing report responses.

## Data model

`internal/model/report.go` — extend `ReportResult` with four optional, per-currency subtotal maps (additive, non-breaking; existing `Total*` fields untouched):

```go
type ReportResult struct {
    // …existing fields…
    TotalIncome           map[string]int64 `json:"total_income"`
    TotalIncomeRegular    map[string]int64 `json:"total_income_regular"`
    TotalIncomeIrregular  map[string]int64 `json:"total_income_irregular"`
    TotalExpense          map[string]int64 `json:"total_expense"`
    TotalExpenseRegular   map[string]int64 `json:"total_expense_regular"`
    TotalExpenseIrregular map[string]int64 `json:"total_expense_irregular"`
    // …
}
```

Each map keys currency to cents. Old reports without these fields populated marshal them as `null` (Go zero-value map), which JSON decoders treat as absent and the SPA tolerates as "no data" → no sub-line rendered.

## Service layer

`internal/service/report_service.go` `buildReportMaps` already loads:
- `txSplitsMap` (transaction id → splits with account types)
- `txs` (used to build `txTypeMap` of `txID → TransactionType`)

Extend this to build a parallel `txRegularMap` of `txID → *bool`, then accumulate four per-currency subtotals during the same loop that classifies Income/Expense splits:

```go
txRegularMap := make(map[int64]*bool, len(txs))
for _, tx := range txs {
    txRegularMap[tx.ID] = tx.Regular
}

incomeRegular := map[string]int64{}
incomeIrregular := map[string]int64{}
expenseRegular := map[string]int64{}
expenseIrregular := map[string]int64{}

for txID, details := range txSplitsMap {
    txType := txTypeMap[txID]
    regular := txRegularMap[txID]
    isRegular := regular != nil && *regular

    if includeIncome && txType == model.TxTypeIncome {
        // …existing per-row aggregation…
        for _, split := range details {
            if split.AccountType == model.AccountTypeRevenue {
                amt := utils.AbsInt64(split.Amount)
                if isRegular {
                    incomeRegular[split.Currency] += amt
                } else {
                    incomeIrregular[split.Currency] += amt
                }
            }
        }
    }

    if includeExpense && txType == model.TxTypeExpense {
        // mirror for Expense splits
    }
}
```

`buildReportMaps` returns these four maps alongside the existing two row maps. The three `Generate*` callers copy them into `ReportResult`.

**Invariant cross-check:** for each currency,
`TotalIncomeRegular[ccy] + TotalIncomeIrregular[ccy] == TotalIncome[ccy]`
and the same for Expense. A service test asserts this directly.

**Edge cases:**
- A transaction with `Regular == nil` should not appear here — service-layer validation already guarantees Income/Expense always have `Regular` set. Defensive: a `nil` regular falls into the Irregular bucket (treated as not-regular). The cross-check invariant still holds.
- Zero values: when there are no Income/Expense transactions in the period, the subtotal maps are empty (`map[string]int64{}`), not nil — matching the existing `Total*` pattern. The SPA reads `.total_income_regular[currency] ?? 0`.

## API

No changes. The new fields ship via JSON marshaling of `*model.ReportResult` for free.

## SPA

### Types

`spa/src/lib/types.ts` — extend `ReportResult`:

```ts
export interface ReportResult {
  // …existing fields…
  total_income?: Record<string, number>;
  total_income_regular?: Record<string, number>;
  total_income_irregular?: Record<string, number>;
  total_expense?: Record<string, number>;
  total_expense_regular?: Record<string, number>;
  total_expense_irregular?: Record<string, number>;
  // …
}
```

(`?:` because reports that don't populate them — older API responses, mocks — still parse.)

### Pages

In `spa/src/routes/reports.income-statement.tsx`, `reports.income-breakdown.tsx`, `reports.expense-breakdown.tsx`, build a `subLine` and pass to `KpiCard`:

```tsx
import { useAmountFormat } from '@/lib/server-config';

function regularSubLine(
  regular: number,
  irregular: number,
  currency: string,
  formatCents: (cents: number, ccy: string) => string,
): string | undefined {
  if (regular === 0 && irregular === 0) return undefined;
  return `Regular ${formatCents(regular, currency)} · Irregular ${formatCents(irregular, currency)}`;
}
```

Pull `formatCents` from the existing `useAmountFormat()` hook (already used by `KpiCard` itself; the page-level call is fine because `KpiCard` accepts a pre-formatted string in its `subLine` prop).

Wire to the existing `<KpiCard label="Total Income" amount={income} currency={currency} variant="green" />`:

```tsx
const incomeRegular = result.total_income_regular?.[currency] ?? 0;
const incomeIrregular = result.total_income_irregular?.[currency] ?? 0;
const { formatCents } = useAmountFormat();
<KpiCard
  label="Total Income"
  amount={income}
  currency={currency}
  variant="green"
  subLine={regularSubLine(incomeRegular, incomeIrregular, currency, formatCents)}
/>
```

Same for Expense.

The `regularSubLine` helper lives in `spa/src/lib/reportSubLine.ts` (small file, easy to test in isolation, shared by the three pages).

### Visual

The KpiCard already renders `subLine` as a small muted line beneath the figure. No CSS changes needed. The middle-dot separator (`·`) matches existing conventions (e.g., `CurrencyFooter`).

## Testing

### Backend

- `internal/service/report_service_test.go` — add `TestGenerateIncomeStatement_RegularSubtotals`. Seed: two Income transactions (one regular $5,000, one irregular $300) and two Expense transactions (one regular $1,800, one irregular $480). Assert all six totals (Income/Expense × Total/Regular/Irregular) per currency, and the invariant `Regular + Irregular == Total`.
- Extend `TestGenerateIncomeBreakdown` and `TestGenerateExpenseBreakdown` (or add focused tests) with a fixture that exercises the subtotal maps in those code paths too.

### SPA

- `spa/src/test/reportSubLine.test.ts` — unit test the helper: positive values render with `·` separator; both-zero returns undefined; currency symbol matches.
- `spa/src/test/reports.income-statement.test.tsx` — extend with a fixture whose mocked report includes `total_income_regular`/`total_income_irregular`/`total_expense_regular`/`total_expense_irregular`; assert the subline text appears under each KPI.
- Similarly extend `reports.income-breakdown.test.tsx` and `reports.expense-breakdown.test.tsx`.

## Rollout

- Additive across model, API, SPA — no breaking changes.
- No new migration; reuses Task 0011's `regular` column.
- After implementation, rebuild the embedded SPA bundle (`internal/web/dist/index.html`) as the final commit, matching the existing workflow.
