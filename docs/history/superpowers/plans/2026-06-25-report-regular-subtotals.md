# Report Regular/Irregular Subtotals Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show regular and irregular subtotals as a sub-line under the existing Total Income / Total Expense KPI cards on all three SPA report pages.

**Architecture:** Compute four new per-currency subtotal maps in `buildReportMaps` during the existing Income/Expense aggregation loop (using `tx.Regular`), return them via `ReportResult` as four optional fields, and render them through the existing `KpiCard.subLine` slot in the SPA. No new endpoints, no migration, additive at every layer.

**Tech Stack:** Go (model + service), React + TypeScript (SPA), Vitest, testify.

**Spec:** [docs/superpowers/specs/2026-06-25-report-regular-subtotals-design.md](../specs/2026-06-25-report-regular-subtotals-design.md)

---

## File Map

**Modify (Go):**
- `internal/model/report.go` — add four optional fields to `ReportResult`
- `internal/service/report_service.go` — extend `buildReportMaps` return signature and per-tx loop; populate fields in `GenerateIncomeStatement`, `GenerateIncomeBreakdown`, `GenerateExpenseBreakdown`
- `internal/service/report_service_test.go` — add subtotal assertions

**Modify (SPA):**
- `spa/src/lib/types.ts` — extend `ReportResult` interface with four optional fields

**Create (SPA):**
- `spa/src/lib/reportSubLine.ts` — `regularSubLine` helper
- `spa/src/test/reportSubLine.test.ts` — unit test for the helper

**Modify (SPA pages):**
- `spa/src/routes/reports.income-statement.tsx` — wire subLine on Income + Expense KPIs
- `spa/src/routes/reports.income-breakdown.tsx` — wire subLine on Total Income KPI
- `spa/src/routes/reports.expense-breakdown.tsx` — wire subLine on Total Expense KPI

**Modify (SPA tests):**
- `spa/src/test/reports.income-statement.test.tsx`
- `spa/src/test/reports.income-breakdown.test.tsx`
- `spa/src/test/reports.expense-breakdown.test.tsx`

**Modify (embedded bundle):**
- `internal/web/dist/index.html` — rebuilt as the final step

---

## Task 1: Extend `ReportResult` model

**Files:**
- Modify: `internal/model/report.go`

- [ ] **Step 1.1: Edit `internal/model/report.go`**

Insert four new fields into `ReportResult`. The final shape:

```go
type ReportResult struct {
    Period                string             `json:"period"`
    TotalIncome           map[string]int64   `json:"total_income"`
    TotalIncomeRegular    map[string]int64   `json:"total_income_regular"`
    TotalIncomeIrregular  map[string]int64   `json:"total_income_irregular"`
    TotalExpense          map[string]int64   `json:"total_expense"`
    TotalExpenseRegular   map[string]int64   `json:"total_expense_regular"`
    TotalExpenseIrregular map[string]int64   `json:"total_expense_irregular"`
    NetAmount             map[string]int64   `json:"net_amount"`
    NetWorth              map[string]int64   `json:"net_worth"`
    PreviousNetWorth      map[string]int64   `json:"previous_net_worth"`
    NetWorthGrowthPct     map[string]float64 `json:"net_worth_growth_pct"`
    IncomeRows            []ReportRow        `json:"income_rows"`
    ExpenseRows           []ReportRow        `json:"expense_rows"`
}
```

(Place the four `*Regular`/`*Irregular` fields immediately after their corresponding totals so the grouping is obvious.)

- [ ] **Step 1.2: Verify build**

Run: `go build ./...`
Expected: PASS. Existing report code paths leave the four new fields as nil maps (Go zero value) which JSON marshals as `null` — backward-compatible with any existing API consumers.

- [ ] **Step 1.3: Commit**

```bash
git add internal/model/report.go
git commit -m "model: add Regular/Irregular subtotal maps to ReportResult"
```

---

## Task 2: Wire subtotals through the report service (TDD)

**Files:**
- Modify: `internal/service/report_service.go`
- Modify: `internal/service/report_service_test.go`

- [ ] **Step 2.1: Write the failing test**

Append to `internal/service/report_service_test.go`. The file is white-box `package service` per CLAUDE.md.

The fixture sets up: two Income transactions (one regular salary, one irregular bonus) and two Expense transactions (one regular rent, one irregular vacation), all in the same period, same currency. Use the existing test harness (`newTestTransactionService`, `accRepo.addAccount`, `txRepo.addTransaction`).

```go
func TestGenerateIncomeStatement_RegularSubtotals(t *testing.T) {
    accRepo := newMockAccountRepo()
    txRepo := newMockTransactionRepo()
    svc := newTestTransactionService(accRepo, txRepo)
    ctx := context.Background()

    // Accounts.
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"})
    accRepo.addAccount(&model.Account{ID: 2, Name: "Revenue:Salary", Type: model.AccountTypeRevenue, Currency: "USD"})
    accRepo.addAccount(&model.Account{ID: 3, Name: "Revenue:Bonus", Type: model.AccountTypeRevenue, Currency: "USD"})
    accRepo.addAccount(&model.Account{ID: 4, Name: "Expenses:Rent", Type: model.AccountTypeExpense, Currency: "USD"})
    accRepo.addAccount(&model.Account{ID: 5, Name: "Expenses:Vacation", Type: model.AccountTypeExpense, Currency: "USD"})

    boolPtr := func(b bool) *bool { return &b }

    // Two Income transactions.
    txRepo.addTransaction(
        &model.Transaction{ID: 10, Timestamp: 1700000100, Type: model.TxTypeIncome, Status: model.StatusCleared, Regular: boolPtr(true)},
        []*model.Split{
            {ID: 100, TransactionID: 10, AccountID: 1, Amount: 500_000, Currency: "USD"},  // bank +5000.00
            {ID: 101, TransactionID: 10, AccountID: 2, Amount: -500_000, Currency: "USD"}, // salary -5000.00 (credit-normal)
        },
    )
    txRepo.addTransaction(
        &model.Transaction{ID: 11, Timestamp: 1700000200, Type: model.TxTypeIncome, Status: model.StatusCleared, Regular: boolPtr(false)},
        []*model.Split{
            {ID: 110, TransactionID: 11, AccountID: 1, Amount: 30_000, Currency: "USD"},
            {ID: 111, TransactionID: 11, AccountID: 3, Amount: -30_000, Currency: "USD"},
        },
    )

    // Two Expense transactions.
    txRepo.addTransaction(
        &model.Transaction{ID: 12, Timestamp: 1700000300, Type: model.TxTypeExpense, Status: model.StatusCleared, Regular: boolPtr(true)},
        []*model.Split{
            {ID: 120, TransactionID: 12, AccountID: 4, Amount: 180_000, Currency: "USD"},  // rent +1800.00
            {ID: 121, TransactionID: 12, AccountID: 1, Amount: -180_000, Currency: "USD"},
        },
    )
    txRepo.addTransaction(
        &model.Transaction{ID: 13, Timestamp: 1700000400, Type: model.TxTypeExpense, Status: model.StatusCleared, Regular: boolPtr(false)},
        []*model.Split{
            {ID: 130, TransactionID: 13, AccountID: 5, Amount: 48_000, Currency: "USD"},
            {ID: 131, TransactionID: 13, AccountID: 1, Amount: -48_000, Currency: "USD"},
        },
    )

    // Also seed the splits-with-accounts projection used by the service.
    seedSplitsWithAccts(txRepo, accRepo)

    result, err := svc.GenerateIncomeStatement(ctx, 1700000000, 1700000500)
    if err != nil {
        t.Fatalf("GenerateIncomeStatement returned error: %v", err)
    }

    // Total Income = 5000 + 300 = 5300; Regular = 5000; Irregular = 300.
    if got := result.TotalIncome["USD"]; got != 530_000 {
        t.Errorf("TotalIncome USD = %d, want 530000 (5300.00)", got)
    }
    if got := result.TotalIncomeRegular["USD"]; got != 500_000 {
        t.Errorf("TotalIncomeRegular USD = %d, want 500000 (5000.00)", got)
    }
    if got := result.TotalIncomeIrregular["USD"]; got != 30_000 {
        t.Errorf("TotalIncomeIrregular USD = %d, want 30000 (300.00)", got)
    }

    // Total Expense = 1800 + 480 = 2280; Regular = 1800; Irregular = 480.
    if got := result.TotalExpense["USD"]; got != 228_000 {
        t.Errorf("TotalExpense USD = %d, want 228000 (2280.00)", got)
    }
    if got := result.TotalExpenseRegular["USD"]; got != 180_000 {
        t.Errorf("TotalExpenseRegular USD = %d, want 180000 (1800.00)", got)
    }
    if got := result.TotalExpenseIrregular["USD"]; got != 48_000 {
        t.Errorf("TotalExpenseIrregular USD = %d, want 48000 (480.00)", got)
    }

    // Invariant: Regular + Irregular == Total per currency.
    for ccy := range result.TotalIncome {
        if result.TotalIncomeRegular[ccy]+result.TotalIncomeIrregular[ccy] != result.TotalIncome[ccy] {
            t.Errorf("invariant broken: TotalIncomeRegular+Irregular != TotalIncome for %s", ccy)
        }
    }
    for ccy := range result.TotalExpense {
        if result.TotalExpenseRegular[ccy]+result.TotalExpenseIrregular[ccy] != result.TotalExpense[ccy] {
            t.Errorf("invariant broken: TotalExpenseRegular+Irregular != TotalExpense for %s", ccy)
        }
    }
}

// seedSplitsWithAccts populates the mock's splitsWithAccts field so that
// buildReportMaps' GetSplitsWithAccountsByDateRange call sees the right
// data. The mock's CreateTransactionWithSplits stores splits but does NOT
// populate the projection — it must be primed explicitly for tests that
// exercise report code paths.
func seedSplitsWithAccts(txRepo *mockTransactionRepo, accRepo *mockAccountRepo) {
    for txID, splits := range txRepo.splits {
        details := make([]model.SplitDetail, 0, len(splits))
        for _, s := range splits {
            acc := accRepo.accountsByID[s.AccountID]
            details = append(details, model.SplitDetail{
                ID:          s.ID,
                AccountID:   s.AccountID,
                AccountName: acc.Name,
                AccountType: acc.Type,
                Amount:      s.Amount,
                Currency:    s.Currency,
                Memo:        s.Memo,
            })
        }
        txRepo.splitsWithAccts[txID] = details
    }
}
```

If `seedSplitsWithAccts` is already present in the test file (search for it first — `grep -n seedSplitsWithAccts internal/service/report_service_test.go`), reuse the existing helper instead of redefining it.

- [ ] **Step 2.2: Run the test to verify it fails**

Run: `go test ./internal/service/ -run TestGenerateIncomeStatement_RegularSubtotals -v`
Expected: FAIL — the four `Total*Regular`/`Total*Irregular` fields are nil, so the map reads return 0 and assertions for 500000 / 30000 / 180000 / 48000 all fail.

- [ ] **Step 2.3: Refactor `buildReportMaps` to also return subtotal maps**

Open `internal/service/report_service.go` and find `buildReportMaps` (around line 162). Change its signature and body:

```go
// buildReportMaps fetches all splits in the date range and aggregates them into
// income/expense row maps plus per-currency regular/irregular subtotals.
// Pass includeIncome=false to skip income classification (and vice versa) to
// avoid unnecessary work for breakdown-only queries.
func (ts *TransactionService) buildReportMaps(
    ctx context.Context,
    startTime, endTime int64,
    includeIncome, includeExpense bool,
) (
    incomeByAccount, expenseByAccount map[string]*model.ReportRow,
    incomeRegular, incomeIrregular, expenseRegular, expenseIrregular map[string]int64,
    err error,
) {
    txSplitsMap, err := ts.txRepo.GetSplitsWithAccountsByDateRange(ctx, startTime, endTime)
    if err != nil {
        return nil, nil, nil, nil, nil, nil, err
    }

    txs, err := ts.txRepo.GetTransactionsByDateRange(ctx, startTime, endTime)
    if err != nil {
        return nil, nil, nil, nil, nil, nil, err
    }
    txTypeMap := make(map[int64]model.TransactionType, len(txs))
    txRegularMap := make(map[int64]*bool, len(txs))
    for _, tx := range txs {
        txTypeMap[tx.ID] = tx.Type
        txRegularMap[tx.ID] = tx.Regular
    }

    incomeByAccount = map[string]*model.ReportRow{}
    expenseByAccount = map[string]*model.ReportRow{}
    incomeRegular = map[string]int64{}
    incomeIrregular = map[string]int64{}
    expenseRegular = map[string]int64{}
    expenseIrregular = map[string]int64{}

    for txID, details := range txSplitsMap {
        txType := txTypeMap[txID]
        reg := txRegularMap[txID]
        isRegular := reg != nil && *reg

        if includeIncome && txType == model.TxTypeIncome {
            offset := offsetAccountName(details, model.AccountTypeRevenue)
            for _, split := range details {
                if split.AccountType == model.AccountTypeRevenue {
                    key := split.AccountName + "|" + offset + "|" + split.Currency
                    row := getOrCreateRowWithOffset(incomeByAccount, key, split.AccountName, offset, split.Currency)
                    amt := utils.AbsInt64(split.Amount)
                    row.Amount += amt
                    row.TxCount++
                    if isRegular {
                        incomeRegular[split.Currency] += amt
                    } else {
                        incomeIrregular[split.Currency] += amt
                    }
                }
            }
        }

        if includeExpense && txType == model.TxTypeExpense {
            offset := offsetAccountName(details, model.AccountTypeExpense)
            for _, split := range details {
                if split.AccountType == model.AccountTypeExpense {
                    key := split.AccountName + "|" + offset + "|" + split.Currency
                    row := getOrCreateRowWithOffset(expenseByAccount, key, split.AccountName, offset, split.Currency)
                    amt := utils.AbsInt64(split.Amount)
                    row.Amount += amt
                    row.TxCount++
                    if isRegular {
                        expenseRegular[split.Currency] += amt
                    } else {
                        expenseIrregular[split.Currency] += amt
                    }
                }
            }
        }
    }

    return incomeByAccount, expenseByAccount, incomeRegular, incomeIrregular, expenseRegular, expenseIrregular, nil
}
```

- [ ] **Step 2.4: Update the three callers**

In the same file:

**`GenerateIncomeStatement`** (around line 212):

```go
func (ts *TransactionService) GenerateIncomeStatement(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
    incomeByAccount, expenseByAccount, incomeRegular, incomeIrregular, expenseRegular, expenseIrregular, err :=
        ts.buildReportMaps(ctx, startTime, endTime, true, true)
    if err != nil {
        return nil, err
    }

    result := &model.ReportResult{
        IncomeRows:            rowsFromMap(incomeByAccount),
        ExpenseRows:           rowsFromMap(expenseByAccount),
        TotalIncome:           map[string]int64{},
        TotalIncomeRegular:    incomeRegular,
        TotalIncomeIrregular:  incomeIrregular,
        TotalExpense:          map[string]int64{},
        TotalExpenseRegular:   expenseRegular,
        TotalExpenseIrregular: expenseIrregular,
        NetAmount:             map[string]int64{},
    }
    for _, r := range result.IncomeRows {
        result.TotalIncome[r.Currency] += r.Amount
    }
    for _, r := range result.ExpenseRows {
        result.TotalExpense[r.Currency] += r.Amount
    }
    for ccy, inc := range result.TotalIncome {
        result.NetAmount[ccy] = inc - result.TotalExpense[ccy]
    }
    for ccy, exp := range result.TotalExpense {
        if _, ok := result.NetAmount[ccy]; !ok {
            result.NetAmount[ccy] = -exp
        }
    }

    return result, nil
}
```

**`GenerateIncomeBreakdown`** (around line 244):

```go
func (ts *TransactionService) GenerateIncomeBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
    incomeByAccount, _, incomeRegular, incomeIrregular, _, _, err :=
        ts.buildReportMaps(ctx, startTime, endTime, true, false)
    if err != nil {
        return nil, err
    }

    rows := rowsFromMap(incomeByAccount)
    sort.Slice(rows, func(i, j int) bool { return rows[i].Amount > rows[j].Amount })

    total := map[string]int64{}
    for _, r := range rows {
        total[r.Currency] += r.Amount
    }

    return &model.ReportResult{
        IncomeRows:           rows,
        ExpenseRows:          []model.ReportRow{},
        TotalIncome:          total,
        TotalIncomeRegular:   incomeRegular,
        TotalIncomeIrregular: incomeIrregular,
    }, nil
}
```

**`GenerateExpenseBreakdown`** (around line 266):

```go
func (ts *TransactionService) GenerateExpenseBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
    _, expenseByAccount, _, _, expenseRegular, expenseIrregular, err :=
        ts.buildReportMaps(ctx, startTime, endTime, false, true)
    if err != nil {
        return nil, err
    }

    rows := rowsFromMap(expenseByAccount)
    sort.Slice(rows, func(i, j int) bool { return rows[i].Amount > rows[j].Amount })

    total := map[string]int64{}
    for _, r := range rows {
        total[r.Currency] += r.Amount
    }

    return &model.ReportResult{
        IncomeRows:            []model.ReportRow{},
        ExpenseRows:           rows,
        TotalExpense:          total,
        TotalExpenseRegular:   expenseRegular,
        TotalExpenseIrregular: expenseIrregular,
    }, nil
}
```

- [ ] **Step 2.5: Run the failing test — it should pass now**

Run: `go test ./internal/service/ -run TestGenerateIncomeStatement_RegularSubtotals -v`
Expected: PASS.

- [ ] **Step 2.6: Run full service package**

Run: `go test ./internal/service/...`
Expected: PASS. Existing report tests don't read the new fields, so they remain green.

- [ ] **Step 2.7: Run the full project**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 2.8: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go
git commit -m "service: compute regular/irregular subtotals for income/expense reports"
```

---

## Task 3: Extend SPA `ReportResult` types

**Files:**
- Modify: `spa/src/lib/types.ts`

- [ ] **Step 3.1: Add the four optional fields**

Find the `ReportResult` interface and add the new fields immediately after the corresponding totals. The final shape (only the relevant slice — keep all other fields):

```ts
export interface ReportResult {
  // …existing fields…
  total_income: Record<string, number>;
  total_income_regular?: Record<string, number>;
  total_income_irregular?: Record<string, number>;
  total_expense: Record<string, number>;
  total_expense_regular?: Record<string, number>;
  total_expense_irregular?: Record<string, number>;
  // …existing fields…
}
```

The `?:` is important: when the API doesn't populate them (older deployments, mocks), TypeScript treats reads as `undefined` and the pages use `?? 0`.

- [ ] **Step 3.2: Verify the SPA still type-checks**

Run from `/Users/hance/programming/kea/spa`:
```bash
npx tsc --noEmit
```
Expected: PASS.

- [ ] **Step 3.3: Commit**

```bash
git add spa/src/lib/types.ts
git commit -m "spa: type Regular/Irregular subtotals on ReportResult"
```

---

## Task 4: Create the `regularSubLine` helper (TDD)

**Files:**
- Create: `spa/src/lib/reportSubLine.ts`
- Create: `spa/src/test/reportSubLine.test.ts`

- [ ] **Step 4.1: Write the failing test**

Create `spa/src/test/reportSubLine.test.ts`:

```ts
import { describe, test, expect } from 'vitest';
import { regularSubLine } from '@/lib/reportSubLine';

const fakeFormat = (cents: number, currency: string) =>
  `${currency} ${(cents / 100).toFixed(2)}`;

describe('regularSubLine', () => {
  test('returns formatted line with both values', () => {
    expect(regularSubLine(500_000, 30_000, 'USD', fakeFormat)).toBe(
      'Regular USD 5000.00 · Irregular USD 300.00',
    );
  });

  test('returns the line when only regular is non-zero', () => {
    expect(regularSubLine(500_000, 0, 'USD', fakeFormat)).toBe(
      'Regular USD 5000.00 · Irregular USD 0.00',
    );
  });

  test('returns the line when only irregular is non-zero', () => {
    expect(regularSubLine(0, 30_000, 'USD', fakeFormat)).toBe(
      'Regular USD 0.00 · Irregular USD 300.00',
    );
  });

  test('returns undefined when both are zero', () => {
    expect(regularSubLine(0, 0, 'USD', fakeFormat)).toBeUndefined();
  });
});
```

- [ ] **Step 4.2: Run to confirm it fails**

```bash
cd spa && npm test -- --run reportSubLine
```
Expected: FAIL — `Cannot find module '@/lib/reportSubLine'`.

- [ ] **Step 4.3: Implement the helper**

Create `spa/src/lib/reportSubLine.ts`:

```ts
/**
 * Build the muted sub-line shown under Total Income / Total Expense KPI cards.
 * Returns undefined when both subtotals are zero, so the caller can skip
 * passing a sub-line altogether (KpiCard renders nothing for an undefined
 * sub-line).
 */
export function regularSubLine(
  regular: number,
  irregular: number,
  currency: string,
  formatCents: (cents: number, currency: string) => string,
): string | undefined {
  if (regular === 0 && irregular === 0) return undefined;
  return `Regular ${formatCents(regular, currency)} · Irregular ${formatCents(irregular, currency)}`;
}
```

- [ ] **Step 4.4: Run to confirm it passes**

```bash
cd spa && npm test -- --run reportSubLine
```
Expected: PASS (4/4 tests).

- [ ] **Step 4.5: Commit**

```bash
git add spa/src/lib/reportSubLine.ts spa/src/test/reportSubLine.test.ts
git commit -m "spa: add regularSubLine helper for KPI subtotals"
```

---

## Task 5: Wire subLine into the Income Statement page

**Files:**
- Modify: `spa/src/routes/reports.income-statement.tsx`

- [ ] **Step 5.1: Add the helper import and compute subtotals**

In `spa/src/routes/reports.income-statement.tsx`, add the import next to the existing reports imports (top of file):

```ts
import { regularSubLine } from '@/lib/reportSubLine';
import { useAmountFormat } from '@/lib/server-config';
```

Note: `useAmountFormat` is already exported from `@/lib/server-config` — `KpiCard` uses it internally. The page-level call is new but cheap (it's a hook returning a memoized formatter).

Inside the body, after the existing `const net = result.net_amount[currency] ?? 0;` line, add:

```tsx
const incomeRegular = result.total_income_regular?.[currency] ?? 0;
const incomeIrregular = result.total_income_irregular?.[currency] ?? 0;
const expenseRegular = result.total_expense_regular?.[currency] ?? 0;
const expenseIrregular = result.total_expense_irregular?.[currency] ?? 0;
const { formatCents } = useAmountFormat();
```

- [ ] **Step 5.2: Pass `subLine` to both KpiCards**

Update the Income and Expense `KpiCard` instances to include `subLine`. The Income card's final shape:

```tsx
<KpiCard
  label="Income"
  amount={income}
  currency={currency}
  variant="green"
  subLine={regularSubLine(incomeRegular, incomeIrregular, currency, formatCents)}
  diff={
    previousQuery.isSuccess
      ? {
          delta: income - (previousQuery.data.total_income[currency] ?? 0),
          prevAmount: previousQuery.data.total_income[currency] ?? 0,
          goodWhen: 'up',
        }
      : undefined
  }
/>
```

Same shape for the Expense card (variant="red", `expenseRegular`/`expenseIrregular`, prevAmount from `total_expense`).

Leave the Net card untouched — its existing `subLine={netSubLine}` shows the net-worth growth percentage.

- [ ] **Step 5.3: Type-check + run**

```bash
cd spa && npx tsc --noEmit
```
Expected: PASS.

- [ ] **Step 5.4: Commit**

```bash
git add spa/src/routes/reports.income-statement.tsx
git commit -m "spa: show regular/irregular subtotals on Income Statement KPIs"
```

---

## Task 6: Wire subLine into the Income Breakdown page

**Files:**
- Modify: `spa/src/routes/reports.income-breakdown.tsx`

- [ ] **Step 6.1: Add imports and compute**

Add at the top:

```ts
import { regularSubLine } from '@/lib/reportSubLine';
import { useAmountFormat } from '@/lib/server-config';
```

Inside the body after the existing `const income = result.total_income[currency] ?? 0;` line, add:

```tsx
const incomeRegular = result.total_income_regular?.[currency] ?? 0;
const incomeIrregular = result.total_income_irregular?.[currency] ?? 0;
const { formatCents } = useAmountFormat();
```

- [ ] **Step 6.2: Update the KpiCard**

Replace:
```tsx
<KpiCard label="Total Income" amount={income} currency={currency} variant="green" />
```
with:
```tsx
<KpiCard
  label="Total Income"
  amount={income}
  currency={currency}
  variant="green"
  subLine={regularSubLine(incomeRegular, incomeIrregular, currency, formatCents)}
/>
```

- [ ] **Step 6.3: Type-check**

```bash
cd spa && npx tsc --noEmit
```
Expected: PASS.

- [ ] **Step 6.4: Commit**

```bash
git add spa/src/routes/reports.income-breakdown.tsx
git commit -m "spa: show regular/irregular subtotal on Income Breakdown KPI"
```

---

## Task 7: Wire subLine into the Expense Breakdown page

**Files:**
- Modify: `spa/src/routes/reports.expense-breakdown.tsx`

- [ ] **Step 7.1: Add imports and compute**

At the top:

```ts
import { regularSubLine } from '@/lib/reportSubLine';
import { useAmountFormat } from '@/lib/server-config';
```

Inside the body after the existing `const expense = result.total_expense[currency] ?? 0;` line:

```tsx
const expenseRegular = result.total_expense_regular?.[currency] ?? 0;
const expenseIrregular = result.total_expense_irregular?.[currency] ?? 0;
const { formatCents } = useAmountFormat();
```

- [ ] **Step 7.2: Update the KpiCard**

Replace:
```tsx
<KpiCard label="Total Expense" amount={expense} currency={currency} variant="red" />
```
with:
```tsx
<KpiCard
  label="Total Expense"
  amount={expense}
  currency={currency}
  variant="red"
  subLine={regularSubLine(expenseRegular, expenseIrregular, currency, formatCents)}
/>
```

- [ ] **Step 7.3: Type-check**

```bash
cd spa && npx tsc --noEmit
```
Expected: PASS.

- [ ] **Step 7.4: Commit**

```bash
git add spa/src/routes/reports.expense-breakdown.tsx
git commit -m "spa: show regular/irregular subtotal on Expense Breakdown KPI"
```

---

## Task 8: SPA page tests for the subLine

**Files:**
- Modify: `spa/src/test/reports.income-statement.test.tsx`
- Modify: `spa/src/test/reports.income-breakdown.test.tsx`
- Modify: `spa/src/test/reports.expense-breakdown.test.tsx`

- [ ] **Step 8.1: Inspect existing fixture pattern**

Read `spa/src/test/reports.income-statement.test.tsx` first. It likely uses a `vi.stubGlobal('fetch', ...)` or a per-test fetch mock returning a fixture report. Note the shape of the fixture (`total_income`, `total_expense`, rows, etc.) and extend it with `total_income_regular`, `total_income_irregular`, `total_expense_regular`, `total_expense_irregular`.

- [ ] **Step 8.2: Add a subLine assertion to `reports.income-statement.test.tsx`**

Append (or merge into the relevant describe block):

```tsx
test('renders Regular/Irregular subLine on Income and Expense KPIs', async () => {
  // Stub fetch with a fixture that includes the new subtotal fields.
  // Use the same currency the page reads ('USD' in the default fixture).
  // Income: total 5300, regular 5000, irregular 300.
  // Expense: total 2280, regular 1800, irregular 480.
  // (Mirror the existing test setup; only the subtotal fields are new.)

  // After render:
  // expect(await screen.findByText(/Regular .* 5000.*Irregular .* 300/)).toBeInTheDocument();
  // expect(screen.getByText(/Regular .* 1800.*Irregular .* 480/)).toBeInTheDocument();
});
```

Fill in the fixture shape from the file's existing helper. If the existing tests use a `makeReport()` factory, extend it with the four new fields; if not, define the fixture inline matching the new `ReportResult` interface from Task 3.

- [ ] **Step 8.3: Same for `reports.income-breakdown.test.tsx`**

```tsx
test('renders Regular/Irregular subLine on the Total Income KPI', async () => {
  // Fixture: income 5300, regular 5000, irregular 300, at least one income_row.
  // expect(await screen.findByText(/Regular .* 5000.*Irregular .* 300/)).toBeInTheDocument();
});
```

- [ ] **Step 8.4: Same for `reports.expense-breakdown.test.tsx`**

```tsx
test('renders Regular/Irregular subLine on the Total Expense KPI', async () => {
  // Fixture: expense 2280, regular 1800, irregular 480, at least one expense_row.
  // expect(await screen.findByText(/Regular .* 1800.*Irregular .* 480/)).toBeInTheDocument();
});
```

- [ ] **Step 8.5: Run**

```bash
cd spa && npm test -- --run reports
```
Expected: all PASS. The 6 pre-existing `balances.*` failures noted in the Regular-attribute branch are tolerated (they're in unrelated test files; `npm test -- --run reports` doesn't run them).

- [ ] **Step 8.6: Commit**

```bash
git add spa/src/test/reports.income-statement.test.tsx spa/src/test/reports.income-breakdown.test.tsx spa/src/test/reports.expense-breakdown.test.tsx
git commit -m "spa: test Regular/Irregular subLine renders on report KPIs"
```

---

## Task 9: Rebuild the embedded SPA bundle

**Files:**
- Modify: `internal/web/dist/index.html`

- [ ] **Step 9.1: Build**

```bash
cd spa && npm run build
```
Expected: succeeds; outputs to `internal/web/dist/`.

- [ ] **Step 9.2: Verify Go embed still builds**

```bash
go build ./...
```
Expected: PASS.

- [ ] **Step 9.3: Commit only the tracked `index.html`**

The `internal/web/dist/assets/*` files are gitignored by project convention; only `index.html` is tracked (and references the new asset hashes).

```bash
git add internal/web/dist/index.html
git commit -m "build(spa): refresh embedded bundle for report subtotals"
```

---

## Task 10: Final full-project verification

- [ ] **Step 10.1: Go tests**

```bash
go test ./...
```
Expected: PASS.

- [ ] **Step 10.2: SPA tests**

```bash
cd spa && npm test -- --run
```
Expected: 283/289 PASS like at the end of the previous feature — the 6 `balances.*` failures pre-date this work and are not regressions. No new failures.

- [ ] **Step 10.3: `go mod tidy`**

```bash
go mod tidy
```
Expected: no changes to `go.mod`/`go.sum` (this plan adds no Go dependencies).

If anything is dirty, commit it:

```bash
git diff --cached --quiet || git commit -m "chore: go mod tidy"
```

---

## Self-Review Notes

- **Spec coverage:** Model fields (Task 1), service computation (Task 2), API piggyback — no separate task because Go's JSON marshal picks up the new fields automatically; SPA types (Task 3), helper (Task 4), three pages (Tasks 5–7), tests (Tasks 2 backend, 4 helper, 8 SPA pages), rebuild (Task 9), final sweep (Task 10). Every section of the spec maps to a task.
- **Invariant test:** Task 2's test asserts `Regular + Irregular == Total` per currency, matching the spec's cross-check.
- **Type consistency:** the Go field names (`TotalIncomeRegular` etc.) and JSON tags (`total_income_regular` etc.) match the TS interface fields in Task 3 and the read sites in Tasks 5–7.
- **Backwards compatibility:** existing report consumers (Go and SPA) ignore the new fields; SPA reads use `?? 0` so missing fields are harmless.
- **Defensive handling:** when `tx.Regular == nil` (shouldn't happen for Income/Expense after Task 4 of the previous feature, but defensive), the loop in Task 2 treats it as irregular — consistent with the spec.
