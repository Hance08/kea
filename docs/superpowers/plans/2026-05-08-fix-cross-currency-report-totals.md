# Fix Cross-Currency Report Totals — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop reports from summing amounts across different currencies into a single misleading total. Group totals by currency so multi-currency ledgers produce financially valid output.

**Architecture:** Replace scalar total fields (`TotalIncome int64`, etc.) with `map[string]int64` keyed by currency code. Each report method aggregates per-currency. The UI and JSON rendering layers iterate the map to display one total line per currency. `GetNetWorthAt` returns a map too. The single `Currency string` field on result structs becomes unnecessary for totals (kept only for backward compat in JSON where needed, or removed).

**Tech Stack:** Go, existing mock-based white-box tests in `internal/service`, pterm/tablewriter UI in `ui/views`.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/model/report.go` | Modify | Change total fields from `int64` → `map[string]int64`; remove top-level `Currency` field from result structs |
| `internal/service/report_service.go` | Modify | Aggregate totals per currency in all report methods + `GetNetWorthAt` |
| `internal/service/report_service_test.go` | Modify | Add multi-currency test cases; update existing assertions for new types |
| `cmd/report_types.go` | Modify | Update `ReportProvider` interface for new `GetNetWorthAt` return type |
| `cmd/report_actions.go` | Modify | Adapt net-worth growth calculation for per-currency maps |
| `ui/views/report.go` | Modify | Render per-currency total lines in tables |
| `ui/views/report_json.go` | Modify | Update JSON DTOs to expose per-currency totals |

---

### Task 1: Update Model Structs

**Files:**
- Modify: `internal/model/report.go`

- [ ] **Step 1: Change `ReportResult` total fields to per-currency maps and remove `Currency`**

```go
type ReportResult struct {
	Period            string            `json:"period"`
	TotalIncome       map[string]int64  `json:"total_income"`
	TotalExpense      map[string]int64  `json:"total_expense"`
	NetAmount         map[string]int64  `json:"net_amount"`
	NetWorth          map[string]int64  `json:"net_worth"`
	PreviousNetWorth  map[string]int64  `json:"previous_net_worth"`
	NetWorthGrowthPct map[string]float64 `json:"net_worth_growth_pct"`
	IncomeRows        []ReportRow       `json:"income_rows"`
	ExpenseRows       []ReportRow       `json:"expense_rows"`
}
```

- [ ] **Step 2: Change `BalanceSheetResult` total fields to per-currency maps and remove `Currency`**

```go
type BalanceSheetResult struct {
	Assets           []ReportRow      `json:"assets"`
	Liabilities      []ReportRow      `json:"liabilities"`
	Equity           []ReportRow      `json:"equity"`
	TotalAssets      map[string]int64 `json:"total_assets"`
	TotalLiabilities map[string]int64 `json:"total_liabilities"`
	TotalEquity      map[string]int64 `json:"total_equity"`
	NetWorth         map[string]int64 `json:"net_worth"`
	AsOf             int64            `json:"as_of"`
}
```

- [ ] **Step 3: Verify the project does NOT compile (expected — dependents reference old types)**

Run: `go build ./... 2>&1 | head -30`
Expected: compile errors in `report_service.go`, `report_actions.go`, `ui/views/report.go`, `ui/views/report_json.go`, tests.

- [ ] **Step 4: Commit**

```bash
git add internal/model/report.go
git commit -m "refactor: change report total fields to per-currency maps"
```

---

### Task 2: Update Service — `GetNetWorthAt`

**Files:**
- Modify: `internal/service/report_service.go:15-34`
- Test: `internal/service/report_service_test.go`

- [ ] **Step 1: Write failing multi-currency test for `GetNetWorthAt`**

Add this test to `report_service_test.go` inside `TestGetNetWorthAt`:

```go
t.Run("multi-currency keeps totals separate", func(t *testing.T) {
	txRepo := newMockTransactionRepo()
	addTxSplits(txRepo.splitsWithAccts, 1,
		model.SplitDetail{AccountName: "Assets:USD_Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
		model.SplitDetail{AccountName: "Equity:Opening_USD", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
	)
	addTxSplits(txRepo.splitsWithAccts, 2,
		model.SplitDetail{AccountName: "Assets:TWD_Bank", AccountType: model.AccountTypeAsset, Amount: 50000, Currency: "TWD"},
		model.SplitDetail{AccountName: "Equity:Opening_TWD", AccountType: model.AccountTypeEquity, Amount: -50000, Currency: "TWD"},
	)
	addTxSplits(txRepo.splitsWithAccts, 3,
		model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: 2000, Currency: "USD"},
		model.SplitDetail{AccountName: "Assets:USD_Bank", AccountType: model.AccountTypeAsset, Amount: -2000, Currency: "USD"},
	)
	svc := newTestTransactionService(newMockAccountRepo(), txRepo)

	nw, err := svc.GetNetWorthAt(context.Background(), 0)
	require.NoError(t, err)
	// USD: assets 10000-2000=8000, liabilities 2000 → net 6000
	assert.Equal(t, int64(6000), nw["USD"])
	// TWD: assets 50000, liabilities 0 → net 50000
	assert.Equal(t, int64(50000), nw["TWD"])
	assert.Len(t, nw, 2)
})
```

- [ ] **Step 2: Update the return type of `GetNetWorthAt` to `map[string]int64`**

```go
func (ts *TransactionService) GetNetWorthAt(ctx context.Context, endTime int64) (map[string]int64, error) {
	txSplitsMap, err := ts.txRepo.GetSplitsWithAccountsByDateRange(ctx, 0, endTime)
	if err != nil {
		return nil, err
	}

	assets := map[string]int64{}
	liabilities := map[string]int64{}
	for _, splits := range txSplitsMap {
		for _, split := range splits {
			switch split.AccountType {
			case model.AccountTypeAsset:
				assets[split.Currency] += split.Amount
			case model.AccountTypeLiability:
				liabilities[split.Currency] += split.Amount
			}
		}
	}

	nw := map[string]int64{}
	for ccy, amt := range assets {
		nw[ccy] = amt - liabilities[ccy]
	}
	for ccy, amt := range liabilities {
		if _, ok := nw[ccy]; !ok {
			nw[ccy] = -amt
		}
	}
	return nw, nil
}
```

- [ ] **Step 3: Update existing `GetNetWorthAt` tests to use the new return type**

All existing tests that do `worth, err := svc.GetNetWorthAt(...)` need to change. The single-currency tests used splits without a `Currency` field, so they'll aggregate under `""`. Update them to set `Currency: "USD"` on every `SplitDetail` and assert `nw["USD"]` instead of a bare `int64`.

For the "assets only" test:
```go
t.Run("assets only", func(t *testing.T) {
	txRepo := newMockTransactionRepo()
	addTxSplits(txRepo.splitsWithAccts, 1,
		model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
		model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
	)
	svc := newTestTransactionService(newMockAccountRepo(), txRepo)

	nw, err := svc.GetNetWorthAt(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), nw["USD"])
})
```

Apply the same pattern for "assets minus liabilities", "no transactions returns zero net worth" (returns empty map), and "only income/expense splits are excluded from net worth" (returns empty map). The "repo failure" test just checks `err != nil` — no change needed beyond the return variable name.

- [ ] **Step 4: Run `GetNetWorthAt` tests**

Run: `go test ./internal/service/ -run TestGetNetWorthAt -v`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go
git commit -m "refactor: GetNetWorthAt returns per-currency net worth map"
```

---

### Task 3: Update Service — Income Statement and Breakdowns

**Files:**
- Modify: `internal/service/report_service.go:110-175`
- Test: `internal/service/report_service_test.go`

- [ ] **Step 1: Write failing multi-currency test for `GenerateIncomeStatement`**

Add to `TestGenerateIncomeStatement`:

```go
t.Run("multi-currency totals are grouped by currency", func(t *testing.T) {
	txRepo := newMockTransactionRepo()
	addTxSplits(txRepo.splitsWithAccts, 1,
		model.SplitDetail{AccountName: "Revenue:Salary", AccountType: model.AccountTypeRevenue, Amount: -3000, Currency: "USD"},
		model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 3000, Currency: "USD"},
	)
	addTxSplits(txRepo.splitsWithAccts, 2,
		model.SplitDetail{AccountName: "Revenue:Freelance", AccountType: model.AccountTypeRevenue, Amount: -90000, Currency: "TWD"},
		model.SplitDetail{AccountName: "Assets:TWD_Bank", AccountType: model.AccountTypeAsset, Amount: 90000, Currency: "TWD"},
	)
	addTxSplits(txRepo.splitsWithAccts, 3,
		model.SplitDetail{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
		model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: -500, Currency: "USD"},
	)
	txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
	txRepo.addTransaction(&model.Transaction{ID: 2, Type: model.TxTypeIncome}, nil)
	txRepo.addTransaction(&model.Transaction{ID: 3, Type: model.TxTypeExpense}, nil)
	svc := newTestTransactionService(newMockAccountRepo(), txRepo)

	result, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3000), result.TotalIncome["USD"])
	assert.Equal(t, int64(90000), result.TotalIncome["TWD"])
	assert.Equal(t, int64(500), result.TotalExpense["USD"])
	assert.Equal(t, int64(2500), result.NetAmount["USD"])   // 3000 - 500
	assert.Equal(t, int64(90000), result.NetAmount["TWD"])   // 90000 - 0
})
```

- [ ] **Step 2: Update `GenerateIncomeStatement` to aggregate per currency**

```go
func (ts *TransactionService) GenerateIncomeStatement(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
	incomeByAccount, expenseByAccount, err := ts.buildReportMaps(ctx, startTime, endTime, true, true)
	if err != nil {
		return nil, err
	}

	result := &model.ReportResult{
		TotalIncome:  map[string]int64{},
		TotalExpense: map[string]int64{},
		NetAmount:    map[string]int64{},
		IncomeRows:   rowsFromMap(incomeByAccount),
		ExpenseRows:  rowsFromMap(expenseByAccount),
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

- [ ] **Step 3: Update `GenerateIncomeBreakdown` and `GenerateExpenseBreakdown`**

`GenerateIncomeBreakdown`:
```go
func (ts *TransactionService) GenerateIncomeBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
	incomeByAccount, _, err := ts.buildReportMaps(ctx, startTime, endTime, true, false)
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
		IncomeRows:  rows,
		TotalIncome: total,
	}, nil
}
```

`GenerateExpenseBreakdown`:
```go
func (ts *TransactionService) GenerateExpenseBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
	_, expenseByAccount, err := ts.buildReportMaps(ctx, startTime, endTime, false, true)
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
		ExpenseRows:  rows,
		TotalExpense: total,
	}, nil
}
```

- [ ] **Step 4: Update all existing income/expense tests for new map types**

Every assertion like `assert.Equal(t, int64(2000), result.TotalIncome)` must change to `assert.Equal(t, int64(2000), result.TotalIncome["USD"])`. Set `Currency: "USD"` on every `SplitDetail` in existing tests (use `model.SplitDetail{...}` struct literal form — not the `split()` helper if it doesn't support currency). The "empty data" test should assert `assert.Empty(t, result.TotalIncome)`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/service/ -run "TestGenerateIncomeStatement|TestGenerateIncomeBreakdown|TestGenerateExpenseBreakdown" -v`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go
git commit -m "refactor: income/expense reports aggregate totals per currency"
```

---

### Task 4: Update Service — Balance Sheet

**Files:**
- Modify: `internal/service/report_service.go:177-238`
- Test: `internal/service/report_service_test.go`

- [ ] **Step 1: Write failing multi-currency test for `GenerateBalanceSheet`**

Add to `TestGenerateBalanceSheet`:

```go
t.Run("multi-currency totals are grouped by currency", func(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:USD_Bank", Type: model.AccountTypeAsset, Currency: "USD"})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:TWD_Bank", Type: model.AccountTypeAsset, Currency: "TWD"})
	accRepo.addAccount(&model.Account{ID: 3, Name: "Liabilities:Card", Type: model.AccountTypeLiability, Currency: "USD"})
	accRepo.balances[1] = 10000
	accRepo.balances[2] = 50000
	accRepo.balances[3] = 3000
	svc := newTestTransactionService(accRepo, newMockTransactionRepo())

	result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), result.TotalAssets["USD"])
	assert.Equal(t, int64(50000), result.TotalAssets["TWD"])
	assert.Equal(t, int64(3000), result.TotalLiabilities["USD"])
	assert.Equal(t, int64(7000), result.NetWorth["USD"])   // 10000 - 3000
	assert.Equal(t, int64(50000), result.NetWorth["TWD"])  // 50000 - 0
})
```

- [ ] **Step 2: Update `GenerateBalanceSheet` to aggregate per currency**

```go
func (ts *TransactionService) GenerateBalanceSheet(ctx context.Context, asOf int64) (*model.BalanceSheetResult, error) {
	allAccounts, err := ts.accRepo.GetAllAccounts(ctx)
	if err != nil {
		return nil, err
	}

	balances, err := ts.accRepo.GetAllAccountBalances(ctx, asOf)
	if err != nil {
		return nil, err
	}

	result := &model.BalanceSheetResult{
		AsOf:             asOf,
		TotalAssets:      map[string]int64{},
		TotalLiabilities: map[string]int64{},
		TotalEquity:      map[string]int64{},
		NetWorth:         map[string]int64{},
	}

	for _, acc := range allAccounts {
		balance := balances[acc.ID]
		if balance == 0 {
			continue
		}

		currency := acc.Currency
		if currency == "" {
			currency = ts.config.Defaults.Currency
		}

		row := model.ReportRow{
			AccountName: acc.Name,
			Amount:      balance,
			Currency:    currency,
		}

		switch acc.Type {
		case model.AccountTypeAsset:
			result.Assets = append(result.Assets, row)
			result.TotalAssets[currency] += balance
		case model.AccountTypeLiability:
			result.Liabilities = append(result.Liabilities, row)
			result.TotalLiabilities[currency] += balance
		case model.AccountTypeEquity:
			result.Equity = append(result.Equity, row)
			result.TotalEquity[currency] += balance
		}
	}

	for ccy, assets := range result.TotalAssets {
		result.NetWorth[ccy] = assets - result.TotalLiabilities[ccy]
	}
	for ccy, liab := range result.TotalLiabilities {
		if _, ok := result.NetWorth[ccy]; !ok {
			result.NetWorth[ccy] = -liab
		}
	}

	sort.Slice(result.Assets, func(i, j int) bool {
		return result.Assets[i].Amount > result.Assets[j].Amount
	})
	sort.Slice(result.Liabilities, func(i, j int) bool {
		return result.Liabilities[i].Amount > result.Liabilities[j].Amount
	})
	sort.Slice(result.Equity, func(i, j int) bool {
		return result.Equity[i].Amount > result.Equity[j].Amount
	})

	return result, nil
}
```

- [ ] **Step 3: Update existing balance sheet tests for new map types**

Change `assert.Equal(t, int64(10000), result.TotalAssets)` → `assert.Equal(t, int64(10000), result.TotalAssets["USD"])`, etc. For accounts without explicit `Currency`, the service falls back to config default `"USD"`, so assertions key on `"USD"`. The `Currency` assertion test (`"account with custom currency uses account currency"`) now only checks the row-level `Currency` field (unchanged). Remove any assertion on `result.Currency` since the field no longer exists.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/service/ -run TestGenerateBalanceSheet -v`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go
git commit -m "refactor: balance sheet aggregates totals per currency"
```

---

### Task 5: Update `ReportProvider` Interface and `report_actions.go`

**Files:**
- Modify: `cmd/report_types.go:22-28`
- Modify: `cmd/report_actions.go:36-64, 195-202`

- [ ] **Step 1: Update `ReportProvider` interface**

Change `GetNetWorthAt` return type:

```go
type ReportProvider interface {
	GenerateIncomeStatement(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error)
	GenerateIncomeBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error)
	GenerateExpenseBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error)
	GenerateBalanceSheet(ctx context.Context, asOf int64) (*model.BalanceSheetResult, error)
	GetNetWorthAt(ctx context.Context, endTime int64) (map[string]int64, error)
}
```

- [ ] **Step 2: Update `runIncomeStatement` to use per-currency net worth**

The `result.NetWorth`, `result.PreviousNetWorth`, and `result.NetWorthGrowthPct` are now maps. Update `runIncomeStatement`:

```go
func (r *reportRunner) runIncomeStatement(ctx context.Context) error {
	start, end, period, err := r.resolveDateRange()
	if err != nil {
		return err
	}

	result, err := r.provider.GenerateIncomeStatement(ctx, start, end)
	if err != nil {
		return fmt.Errorf("failed to generate income statement: %w", err)
	}

	result.Period = period

	currentNetWorth, err := r.provider.GetNetWorthAt(ctx, end)
	if err != nil {
		return fmt.Errorf("failed to fetch net worth for current period: %w", err)
	}
	result.NetWorth = currentNetWorth

	_, prevEnd := previousPeriodRange(start, end)
	previousNetWorth, err := r.provider.GetNetWorthAt(ctx, prevEnd)
	if err != nil {
		return fmt.Errorf("failed to fetch net worth for previous period: %w", err)
	}
	result.PreviousNetWorth = previousNetWorth
	result.NetWorthGrowthPct = computeNetWorthGrowthPctMap(currentNetWorth, previousNetWorth)

	return r.view.RenderIncomeStatement(result)
}
```

- [ ] **Step 3: Replace `computeNetWorthGrowthPct` with `computeNetWorthGrowthPctMap`**

```go
func computeNetWorthGrowthPctMap(current, previous map[string]int64) map[string]float64 {
	result := map[string]float64{}
	allCurrencies := map[string]struct{}{}
	for ccy := range current {
		allCurrencies[ccy] = struct{}{}
	}
	for ccy := range previous {
		allCurrencies[ccy] = struct{}{}
	}
	for ccy := range allCurrencies {
		prev := previous[ccy]
		if prev == 0 {
			continue
		}
		cur := current[ccy]
		result[ccy] = (float64(cur-prev) / float64(utils.AbsInt64(prev))) * 100
	}
	return result
}
```

Remove the old `computeNetWorthGrowthPct` function.

- [ ] **Step 4: Verify project compiles (excluding UI)**

Run: `go build ./cmd/... 2>&1 | head -20`
Expected: compile errors only from `ui/views/` (the render functions still reference old types). The `cmd` package should compile after this step — if not, fix remaining references.

- [ ] **Step 5: Commit**

```bash
git add cmd/report_types.go cmd/report_actions.go
git commit -m "refactor: update ReportProvider and report_actions for per-currency totals"
```

---

### Task 6: Update UI — Table Rendering

**Files:**
- Modify: `ui/views/report.go`

- [ ] **Step 1: Add a helper to render per-currency summary lines**

Add this helper at the bottom of `report.go`:

```go
func (v *ReportView) renderPerCurrencyTotals(label string, totals map[string]int64, colorFn func(string) string) {
	currencies := sortedKeys(totals)
	for _, ccy := range currencies {
		amt := totals[ccy]
		v.renderSummaryLine(label, colorFn(utils.FormatAmount(amt)), ccy)
	}
}

func sortedKeys(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
```

Add `"sort"` to the imports.

- [ ] **Step 2: Update `RenderIncomeStatement`**

Replace the summary section (after the expense table) with:

```go
// Summary
v.renderPerCurrencyTotals("Total Income", result.TotalIncome, pterm.Green)
v.renderPerCurrencyTotals("Total Expenses", result.TotalExpense, pterm.Red)

for _, ccy := range sortedKeys(result.NetAmount) {
	net := result.NetAmount[ccy]
	netStr := utils.FormatAmount(utils.AbsInt64(net))
	if net >= 0 {
		v.renderSummaryLine("Net", pterm.Green(netStr), ccy)
	} else {
		v.renderSummaryLine("Net", pterm.Red("-"+netStr), ccy)
	}
}

for _, ccy := range sortedKeys(result.NetWorth) {
	nw := result.NetWorth[ccy]
	nwStr := utils.FormatAmount(utils.AbsInt64(nw))
	if nw >= 0 {
		v.renderSummaryLine("Net Worth", pterm.Green(nwStr), ccy)
	} else {
		v.renderSummaryLine("Net Worth", pterm.Red("-"+nwStr), ccy)
	}
}

growthCurrencies := make([]string, 0, len(result.NetWorthGrowthPct))
for ccy := range result.NetWorthGrowthPct {
	growthCurrencies = append(growthCurrencies, ccy)
}
sort.Strings(growthCurrencies)

if len(growthCurrencies) == 0 {
	v.renderSummaryLineNoCurrency("Net Worth Growth", "N/A")
} else {
	for _, ccy := range growthCurrencies {
		pct := result.NetWorthGrowthPct[ccy]
		pctText := fmt.Sprintf("%+.2f%%", pct)
		switch {
		case pct > 0:
			v.renderSummaryLine("Net Worth Growth", pterm.Green(pctText), ccy)
		case pct < 0:
			v.renderSummaryLine("Net Worth Growth", pterm.Red(pctText), ccy)
		default:
			v.renderSummaryLine("Net Worth Growth", pctText, ccy)
		}
	}
}
```

- [ ] **Step 3: Update `RenderExpenseBreakdown` summary**

Replace the summary line:
```go
v.renderPerCurrencyTotals("Total Expenses", result.TotalExpense, pterm.Red)
```

- [ ] **Step 4: Update `RenderIncomeBreakdown` summary**

Replace the summary line:
```go
v.renderPerCurrencyTotals("Total Income", result.TotalIncome, pterm.Green)
```

- [ ] **Step 5: Update `RenderBalanceSheet` summary**

Replace the three summary lines at the bottom with:

```go
v.renderPerCurrencyTotals("Total Assets", result.TotalAssets, pterm.Green)
v.renderPerCurrencyTotals("Total Liabilities", result.TotalLiabilities, pterm.Red)

for _, ccy := range sortedKeys(result.NetWorth) {
	nw := result.NetWorth[ccy]
	nwStr := utils.FormatAmount(utils.AbsInt64(nw))
	if nw >= 0 {
		v.renderSummaryLine("Net Worth", pterm.Green(nwStr), ccy)
	} else {
		v.renderSummaryLine("Net Worth", pterm.Red("-"+nwStr), ccy)
	}
}
```

- [ ] **Step 6: Commit**

```bash
git add ui/views/report.go
git commit -m "feat: render per-currency totals in report tables"
```

---

### Task 7: Update JSON Rendering

**Files:**
- Modify: `ui/views/report_json.go`

- [ ] **Step 1: Update JSON DTOs**

Replace the scalar total fields with per-currency maps:

```go
type jsonReportResult struct {
	Period            string             `json:"period"`
	TotalIncome       map[string]float64 `json:"total_income"`
	TotalExpense      map[string]float64 `json:"total_expense"`
	NetAmount         map[string]float64 `json:"net_amount"`
	NetWorth          map[string]float64 `json:"net_worth"`
	PreviousNetWorth  map[string]float64 `json:"previous_net_worth"`
	NetWorthGrowthPct map[string]float64 `json:"net_worth_growth_pct"`
	IncomeRows        []jsonReportRow    `json:"income_rows"`
	ExpenseRows       []jsonReportRow    `json:"expense_rows"`
}

type jsonBalanceSheetResult struct {
	Assets           []jsonReportRow    `json:"assets"`
	Liabilities      []jsonReportRow    `json:"liabilities"`
	Equity           []jsonReportRow    `json:"equity"`
	TotalAssets      map[string]float64 `json:"total_assets"`
	TotalLiabilities map[string]float64 `json:"total_liabilities"`
	TotalEquity      map[string]float64 `json:"total_equity"`
	NetWorth         map[string]float64 `json:"net_worth"`
	AsOf             int64              `json:"as_of"`
}
```

- [ ] **Step 2: Add a `centsMapToUnitMap` helper**

```go
func centsMapToUnitMap(m map[string]int64) map[string]float64 {
	if m == nil {
		return nil
	}
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = CentsToUnit(v)
	}
	return out
}
```

- [ ] **Step 3: Update `toJSONReportResult`**

```go
func toJSONReportResult(r *model.ReportResult) jsonReportResult {
	return jsonReportResult{
		Period:            r.Period,
		TotalIncome:       centsMapToUnitMap(r.TotalIncome),
		TotalExpense:      centsMapToUnitMap(r.TotalExpense),
		NetAmount:         centsMapToUnitMap(r.NetAmount),
		NetWorth:          centsMapToUnitMap(r.NetWorth),
		PreviousNetWorth:  centsMapToUnitMap(r.PreviousNetWorth),
		NetWorthGrowthPct: r.NetWorthGrowthPct,
		IncomeRows:        toJSONRows(r.IncomeRows),
		ExpenseRows:       toJSONRows(r.ExpenseRows),
	}
}
```

- [ ] **Step 4: Update `toJSONBalanceSheetResult`**

```go
func toJSONBalanceSheetResult(r *model.BalanceSheetResult) jsonBalanceSheetResult {
	return jsonBalanceSheetResult{
		Assets:           toJSONRows(r.Assets),
		Liabilities:      toJSONRows(r.Liabilities),
		Equity:           toJSONRows(r.Equity),
		TotalAssets:      centsMapToUnitMap(r.TotalAssets),
		TotalLiabilities: centsMapToUnitMap(r.TotalLiabilities),
		TotalEquity:      centsMapToUnitMap(r.TotalEquity),
		NetWorth:         centsMapToUnitMap(r.NetWorth),
		AsOf:             r.AsOf,
	}
}
```

- [ ] **Step 5: Build and verify**

Run: `go build ./...`
Expected: clean compile with no errors.

- [ ] **Step 6: Run all tests**

Run: `go test ./... -v`
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add ui/views/report_json.go
git commit -m "feat: update JSON report output for per-currency totals"
```

---

### Task 8: Update `report_actions_test.go` (if it exists) and Clean Up

**Files:**
- Modify: `cmd/report_actions.go` (remove old `computeNetWorthGrowthPct` if not done in Task 5)

- [ ] **Step 1: Search for any remaining references to old fields**

Run: `grep -rn "result\.Currency\|\.TotalAssets[^[]" --include="*.go" . | grep -v "_test.go | grep -v "\.md"`
Expected: no matches (all code paths use per-currency maps now).

- [ ] **Step 2: Run full test suite**

Run: `go test ./...`
Expected: all pass.

- [ ] **Step 3: Final commit**

```bash
git add -A
git commit -m "chore: clean up remaining old currency field references"
```
