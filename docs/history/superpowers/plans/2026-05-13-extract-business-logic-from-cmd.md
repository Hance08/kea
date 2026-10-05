# Extract Business Logic from cmd/ Layer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move business logic out of `cmd/` into `internal/service/` and `internal/model/` so future HTTP API handlers can reuse it without importing Cobra/TUI dependencies.

**Architecture:** Four extraction steps executed as independent commits. Items 3 and 4 (model parsers, date parsing) are pure additions with no cmd/ coupling changes required first. Items 1+2 (date range + report enrichment) are the largest change — they move five functions from cmd/ into the service layer and add three new `GenerateFull*` methods.

**Tech Stack:** Go, testify (assert/require), existing mock infrastructure in `internal/service/testhelper_test.go`.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/model/types.go` | Modify | Add `ParseTransactionType`, `ParseTransactionStatus` |
| `internal/model/types_test.go` | Modify | Add tests for the two new parsers |
| `internal/service/report_service.go` | Modify | Add `DateRangeParams`, `ResolveDateRange`, `GenerateFullIncomeStatement`, `GenerateFullIncomeBreakdown`, `GenerateFullExpenseBreakdown`, and move helper functions |
| `internal/service/report_service_test.go` | Modify | Add tests for `ResolveDateRange` and `GenerateFullIncomeStatement` |
| `internal/service/transaction_ops.go` | Modify | Add `ParseTransactionDate` |
| `internal/service/transaction_ops_test.go` | Modify | Add tests for `ParseTransactionDate` |
| `cmd/report_actions.go` | Modify | Remove extracted functions, simplify `run*` methods |
| `cmd/report_types.go` | Modify | Update `ReportProvider` interface for new `GenerateFull*` methods |
| `cmd/add_actions.go` | Modify | Remove `parseTransactionType`, `parseStatus`, `parseDate`; call model/service equivalents |

---

### Task 1: Add `ParseTransactionType` and `ParseTransactionStatus` to model

**Files:**
- Modify: `internal/model/types.go:107` (append after `IsOpeningBalancesAccount`)
- Modify: `internal/model/types_test.go:24` (append after existing tests)

- [ ] **Step 1: Write failing tests for `ParseTransactionType`**

Add to `internal/model/types_test.go`:

```go
func TestParseTransactionType(t *testing.T) {
	tests := []struct {
		input   string
		want    TransactionType
		wantErr bool
	}{
		{"expense", TxTypeExpense, false},
		{"Expense", TxTypeExpense, false},
		{"EXPENSE", TxTypeExpense, false},
		{"  expense  ", TxTypeExpense, false},
		{"income", TxTypeIncome, false},
		{"Income", TxTypeIncome, false},
		{"transfer", TxTypeTransfer, false},
		{"Transfer", TxTypeTransfer, false},
		{"unknown", "", true},
		{"", "", true},
		{"opening", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTransactionType(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run TestParseTransactionType -v`
Expected: compilation error — `ParseTransactionType` undefined.

- [ ] **Step 3: Implement `ParseTransactionType`**

Add to `internal/model/types.go` (after `IsOpeningBalancesAccount`):

```go
func ParseTransactionType(s string) (TransactionType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "expense":
		return TxTypeExpense, nil
	case "income":
		return TxTypeIncome, nil
	case "transfer":
		return TxTypeTransfer, nil
	default:
		return "", fmt.Errorf("invalid transaction type %q: must be expense, income, or transfer", s)
	}
}
```

Add `"fmt"` to the imports in `types.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/model/ -run TestParseTransactionType -v`
Expected: PASS

- [ ] **Step 5: Write failing tests for `ParseTransactionStatus`**

Add to `internal/model/types_test.go`:

```go
func TestParseTransactionStatus(t *testing.T) {
	tests := []struct {
		input string
		want  TransactionStatus
	}{
		{"pending", StatusPending},
		{"Pending", StatusPending},
		{"PENDING", StatusPending},
		{"cleared", StatusCleared},
		{"Cleared", StatusCleared},
		{"", StatusCleared},
		{"anything", StatusCleared},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseTransactionStatus(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/model/ -run TestParseTransactionStatus -v`
Expected: compilation error — `ParseTransactionStatus` undefined.

- [ ] **Step 7: Implement `ParseTransactionStatus`**

Add to `internal/model/types.go` (after `ParseTransactionType`):

```go
func ParseTransactionStatus(s string) TransactionStatus {
	if strings.ToLower(s) == "pending" {
		return StatusPending
	}
	return StatusCleared
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/model/ -run TestParseTransactionStatus -v`
Expected: PASS

- [ ] **Step 9: Update cmd/add_actions.go to use model parsers**

In `cmd/add_actions.go`:

1. Delete the `parseTransactionType` function (lines 214-225).
2. Delete the `parseStatus` function (lines 293-298).
3. Replace all calls to `parseTransactionType(...)` with `model.ParseTransactionType(...)`.
4. Replace all calls to `parseStatus(...)` with `model.ParseTransactionStatus(...)`.

The calls to update:
- Line 49: `txType, err = parseTransactionType(flags.Type)` → `txType, err = model.ParseTransactionType(flags.Type)`
- Line 39: `status := parseStatus(flags.Status)` → `status := model.ParseTransactionStatus(flags.Status)`
- Line 241: `txType, err := parseTransactionType(flags.Type)` → `txType, err := model.ParseTransactionType(flags.Type)`
- Line 254: `status := parseStatus(flags.Status)` → `status := model.ParseTransactionStatus(flags.Status)`

- [ ] **Step 10: Run full test suite**

Run: `go test ./...`
Expected: all tests pass.

- [ ] **Step 11: Commit**

```bash
git add internal/model/types.go internal/model/types_test.go cmd/add_actions.go
git commit -m "refactor: move ParseTransactionType and ParseTransactionStatus to model package

Extract parseTransactionType and parseStatus from cmd/add_actions.go
into model.ParseTransactionType and model.ParseTransactionStatus so
the future HTTP API layer can reuse them without importing cmd/.

Part of #76."
```

---

### Task 2: Add `ParseTransactionDate` to TransactionService

**Files:**
- Modify: `internal/service/transaction_ops.go:1` (add method)
- Modify: `internal/service/transaction_ops_test.go` (add tests)
- Modify: `cmd/add_actions.go` (delete `parseDate`, call service)

- [ ] **Step 1: Write failing tests for `ParseTransactionDate`**

Add to `internal/service/transaction_ops_test.go`:

```go
func TestParseTransactionDate(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	t.Run("empty string returns current time", func(t *testing.T) {
		before := time.Now().Unix()
		got, err := svc.ParseTransactionDate("")
		after := time.Now().Unix()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, got, before)
		assert.LessOrEqual(t, got, after)
	})

	t.Run("valid date string parses correctly", func(t *testing.T) {
		got, err := svc.ParseTransactionDate("2025-06-15")
		require.NoError(t, err)
		parsed := time.Unix(got, 0)
		assert.Equal(t, 2025, parsed.Year())
		assert.Equal(t, time.June, parsed.Month())
		assert.Equal(t, 15, parsed.Day())
	})

	t.Run("invalid format returns error", func(t *testing.T) {
		_, err := svc.ParseTransactionDate("15/06/2025")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "2006-01-02")
	})

	t.Run("partial date returns error", func(t *testing.T) {
		_, err := svc.ParseTransactionDate("2025-06")
		assert.Error(t, err)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestParseTransactionDate -v`
Expected: compilation error — `ParseTransactionDate` undefined.

- [ ] **Step 3: Implement `ParseTransactionDate`**

Add to `internal/service/transaction_ops.go` (after the `IsEditable` method at the end of the file):

```go
func (ts *TransactionService) ParseTransactionDate(dateStr string) (int64, error) {
	if dateStr == "" {
		return time.Now().Unix(), nil
	}
	t, err := time.ParseInLocation(model.DateFormat, dateStr, time.Local)
	if err != nil {
		return 0, fmt.Errorf("invalid date format, use %s: %w", model.DateFormat, err)
	}
	return t.Unix(), nil
}
```

Ensure `"time"` is in the imports (it already is in `transaction_ops.go`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/ -run TestParseTransactionDate -v`
Expected: PASS

- [ ] **Step 5: Update cmd/add_actions.go to use service method**

In `cmd/add_actions.go`:

1. Delete the `parseDate` method on `addRunner` (lines 203-212).
2. The `addRunner` struct needs access to the transaction service. Check that `r.txSvc` is available — looking at the `runInteractive` method, it uses `r.txSvc` already, so this is accessible.
3. Replace calls: `r.parseDate(flags.Timestamp)` → `r.txSvc.ParseTransactionDate(flags.Timestamp)` and `r.parseDate(dateStr)` → `r.txSvc.ParseTransactionDate(dateStr)`.

The calls to update:
- Line 43: `timestamp, err := r.parseDate(flags.Timestamp)` → `timestamp, err := r.txSvc.ParseTransactionDate(flags.Timestamp)`
- Line 158: `timestamp, err := r.parseDate(dateStr)` → `timestamp, err := r.txSvc.ParseTransactionDate(dateStr)`
- Line 253: `timestamp, err := r.parseDate(flags.Timestamp)` → `timestamp, err := r.txSvc.ParseTransactionDate(flags.Timestamp)`

- [ ] **Step 6: Run full test suite**

Run: `go test ./...`
Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go cmd/add_actions.go
git commit -m "refactor: move ParseTransactionDate to TransactionService

Extract parseDate from cmd/add_actions.go into
TransactionService.ParseTransactionDate so the future HTTP API layer
can reuse date parsing with the 'default to now' rule.

Part of #76."
```

---

### Task 3: Move date range resolution helpers to service layer

**Files:**
- Modify: `internal/service/report_service.go` (add `DateRangeParams`, `ResolveDateRange`, move helpers)
- Modify: `internal/service/report_service_test.go` (add tests)
- Modify: `cmd/report_actions.go` (delete moved functions)

- [ ] **Step 1: Write failing tests for `ResolveDateRange`**

Add to `internal/service/report_service_test.go`:

```go
// ──────────────────────────────────────────────
// ResolveDateRange
// ──────────────────────────────────────────────

func TestResolveDateRange(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	t.Run("month format parses correctly", func(t *testing.T) {
		start, end, period, err := svc.ResolveDateRange(DateRangeParams{Month: "2025-03"})
		require.NoError(t, err)

		startT := time.Unix(start, 0)
		endT := time.Unix(end, 0)
		assert.Equal(t, 2025, startT.Year())
		assert.Equal(t, time.March, startT.Month())
		assert.Equal(t, 1, startT.Day())
		assert.Equal(t, 2025, endT.Year())
		assert.Equal(t, time.March, endT.Month())
		assert.Equal(t, 31, endT.Day())
		assert.Equal(t, "March 2025", period)
	})

	t.Run("invalid month format returns error", func(t *testing.T) {
		_, _, _, err := svc.ResolveDateRange(DateRangeParams{Month: "2025/03"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "YYYY-MM")
	})

	t.Run("from and to date range", func(t *testing.T) {
		start, end, period, err := svc.ResolveDateRange(DateRangeParams{
			From: "2025-01-15",
			To:   "2025-02-15",
		})
		require.NoError(t, err)
		assert.Greater(t, end, start)
		assert.Contains(t, period, "2025-01-15")
		assert.Contains(t, period, "2025-02-15")
	})

	t.Run("from only defaults to epoch start", func(t *testing.T) {
		_, _, _, err := svc.ResolveDateRange(DateRangeParams{From: "2025-01-15"})
		require.NoError(t, err)
	})

	t.Run("to only defaults from epoch", func(t *testing.T) {
		start, _, _, err := svc.ResolveDateRange(DateRangeParams{To: "2025-06-15"})
		require.NoError(t, err)
		assert.Equal(t, int64(0), start)
	})

	t.Run("to before from returns error", func(t *testing.T) {
		_, _, _, err := svc.ResolveDateRange(DateRangeParams{
			From: "2025-06-15",
			To:   "2025-01-01",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "on or after")
	})

	t.Run("invalid from format returns error", func(t *testing.T) {
		_, _, _, err := svc.ResolveDateRange(DateRangeParams{From: "not-a-date"})
		assert.Error(t, err)
	})

	t.Run("invalid to format returns error", func(t *testing.T) {
		_, _, _, err := svc.ResolveDateRange(DateRangeParams{To: "not-a-date"})
		assert.Error(t, err)
	})

	t.Run("empty params defaults to current month", func(t *testing.T) {
		start, end, period, err := svc.ResolveDateRange(DateRangeParams{})
		require.NoError(t, err)

		now := time.Now()
		startT := time.Unix(start, 0)
		assert.Equal(t, now.Year(), startT.Year())
		assert.Equal(t, now.Month(), startT.Month())
		assert.Equal(t, 1, startT.Day())
		assert.Greater(t, end, start)
		assert.NotEmpty(t, period)
	})

	t.Run("month takes priority over from/to", func(t *testing.T) {
		start, _, period, err := svc.ResolveDateRange(DateRangeParams{
			Month: "2025-06",
			From:  "2024-01-01",
			To:    "2024-12-31",
		})
		require.NoError(t, err)
		startT := time.Unix(start, 0)
		assert.Equal(t, 2025, startT.Year())
		assert.Equal(t, time.June, startT.Month())
		assert.Equal(t, "June 2025", period)
	})
}
```

- [ ] **Step 2: Write failing tests for `previousPeriodRange` and `computeNetWorthGrowthPctMap`**

Add to `internal/service/report_service_test.go`:

```go
// ──────────────────────────────────────────────
// previousPeriodRange (white-box)
// ──────────────────────────────────────────────

func TestPreviousPeriodRange(t *testing.T) {
	t.Run("30-day period", func(t *testing.T) {
		start := int64(1000)
		end := int64(1000 + 30*86400 - 1)
		prevStart, prevEnd := previousPeriodRange(start, end)
		assert.Equal(t, start-1, prevEnd)
		assert.Equal(t, end-start, prevEnd-prevStart)
	})

	t.Run("single-day period", func(t *testing.T) {
		start := int64(86400)
		end := int64(86400)
		prevStart, prevEnd := previousPeriodRange(start, end)
		assert.Equal(t, start-1, prevEnd)
		assert.Equal(t, prevStart, prevEnd)
	})
}

// ──────────────────────────────────────────────
// computeNetWorthGrowthPctMap (white-box)
// ──────────────────────────────────────────────

func TestComputeNetWorthGrowthPctMap(t *testing.T) {
	t.Run("basic growth", func(t *testing.T) {
		current := map[string]int64{"USD": 11000}
		previous := map[string]int64{"USD": 10000}
		result := computeNetWorthGrowthPctMap(current, previous)
		assert.InDelta(t, 10.0, result["USD"], 0.01)
	})

	t.Run("zero previous is omitted", func(t *testing.T) {
		current := map[string]int64{"USD": 5000}
		previous := map[string]int64{"USD": 0}
		result := computeNetWorthGrowthPctMap(current, previous)
		_, exists := result["USD"]
		assert.False(t, exists)
	})

	t.Run("negative previous uses absolute value", func(t *testing.T) {
		current := map[string]int64{"USD": -500}
		previous := map[string]int64{"USD": -1000}
		result := computeNetWorthGrowthPctMap(current, previous)
		assert.InDelta(t, 50.0, result["USD"], 0.01)
	})

	t.Run("multi-currency", func(t *testing.T) {
		current := map[string]int64{"USD": 12000, "TWD": 60000}
		previous := map[string]int64{"USD": 10000, "TWD": 50000}
		result := computeNetWorthGrowthPctMap(current, previous)
		assert.InDelta(t, 20.0, result["USD"], 0.01)
		assert.InDelta(t, 20.0, result["TWD"], 0.01)
	})

	t.Run("currency only in current is omitted", func(t *testing.T) {
		current := map[string]int64{"USD": 5000, "TWD": 10000}
		previous := map[string]int64{"USD": 4000}
		result := computeNetWorthGrowthPctMap(current, previous)
		assert.InDelta(t, 25.0, result["USD"], 0.01)
		_, exists := result["TWD"]
		assert.False(t, exists)
	})
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestResolveDateRange|TestPreviousPeriodRange|TestComputeNetWorthGrowthPctMap" -v`
Expected: compilation errors — `DateRangeParams`, `ResolveDateRange`, `previousPeriodRange`, `computeNetWorthGrowthPctMap` undefined in the service package.

- [ ] **Step 4: Move helper functions and add `DateRangeParams` + `ResolveDateRange`**

Add to `internal/service/report_service.go` (before `GetNetWorthAt`):

```go
type DateRangeParams struct {
	Month string
	From  string
	To    string
}

func (ts *TransactionService) ResolveDateRange(params DateRangeParams) (startTime, endTime int64, period string, err error) {
	switch {
	case params.Month != "":
		return parseMonth(params.Month)
	case params.From != "" || params.To != "":
		return parseDateRange(params.From, params.To)
	default:
		now := time.Now()
		return parseMonth(now.Format("2006-01"))
	}
}
```

Add these unexported functions to `internal/service/report_service.go` (at the bottom, after `rowsFromMap`):

```go
func parseMonth(month string) (startTime, endTime int64, period string, err error) {
	loc := time.Local
	t, parseErr := time.ParseInLocation("2006-01", month, loc)
	if parseErr != nil {
		err = fmt.Errorf("invalid --month format %q, expected YYYY-MM", month)
		return
	}

	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0).Add(-time.Second)

	startTime = start.Unix()
	endTime = end.Unix()
	period = start.Format("January 2006")
	return
}

func parseDateRange(from, to string) (startTime, endTime int64, period string, err error) {
	loc := time.Local

	var startDate, endDate time.Time

	if from == "" {
		startDate = time.Unix(0, 0)
	} else {
		startDate, err = time.ParseInLocation(model.DateFormat, from, loc)
		if err != nil {
			err = fmt.Errorf("invalid --from format %q, expected YYYY-MM-DD", from)
			return
		}
	}

	if to == "" {
		endDate = time.Now()
	} else {
		endDate, err = time.ParseInLocation(model.DateFormat, to, loc)
		if err != nil {
			err = fmt.Errorf("invalid --to format %q, expected YYYY-MM-DD", to)
			return
		}
		endDate = endDate.Add(24*time.Hour - time.Second)
	}

	if endDate.Before(startDate) {
		err = fmt.Errorf("--to date must be on or after --from date")
		return
	}

	startTime = startDate.Unix()
	endTime = endDate.Unix()
	period = fmt.Sprintf("%s – %s", startDate.Format(model.DateFormat), endDate.Format(model.DateFormat))
	return
}

func previousPeriodRange(startTime, endTime int64) (prevStart, prevEnd int64) {
	duration := endTime - startTime + 1
	prevEnd = startTime - 1
	prevStart = prevEnd - duration + 1
	return
}

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

Add `"time"` to the import block in `report_service.go` (alongside the existing `"context"`, `"sort"`). Add `"fmt"` as well.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/service/ -run "TestResolveDateRange|TestPreviousPeriodRange|TestComputeNetWorthGrowthPctMap" -v`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `go test ./...`
Expected: all tests pass (cmd/ still has its own copies which compile fine).

- [ ] **Step 7: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go
git commit -m "refactor: add DateRangeParams, ResolveDateRange, and date helpers to service

Move parseMonth, parseDateRange, previousPeriodRange, and
computeNetWorthGrowthPctMap from cmd/report_actions.go into the
service layer. cmd/ still has its copies — next commit will remove them.

Part of #76."
```

---

### Task 4: Add `GenerateFull*` methods and update cmd/ report layer

**Files:**
- Modify: `internal/service/report_service.go` (add three `GenerateFull*` methods)
- Modify: `internal/service/report_service_test.go` (add test for `GenerateFullIncomeStatement`)
- Modify: `cmd/report_actions.go` (simplify to use new methods, delete old functions)
- Modify: `cmd/report_types.go` (update `ReportProvider` interface)

- [ ] **Step 1: Write failing test for `GenerateFullIncomeStatement`**

Add to `internal/service/report_service_test.go`:

```go
// ──────────────────────────────────────────────
// GenerateFullIncomeStatement
// ──────────────────────────────────────────────

func TestGenerateFullIncomeStatement(t *testing.T) {
	t.Run("populates period, net worth, and growth", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Revenue:Salary", model.AccountTypeRevenue, -3000),
			split("Assets:Bank", model.AccountTypeAsset, 3000),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateFullIncomeStatement(context.Background(), DateRangeParams{Month: "2025-03"})
		require.NoError(t, err)
		assert.Equal(t, "March 2025", result.Period)
		assert.NotNil(t, result.NetWorth)
		assert.NotNil(t, result.PreviousNetWorth)
		assert.NotNil(t, result.NetWorthGrowthPct)
		assert.Equal(t, int64(3000), result.TotalIncome["USD"])
	})

	t.Run("invalid date params returns error", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.GenerateFullIncomeStatement(context.Background(), DateRangeParams{Month: "bad"})
		assert.Error(t, err)
	})

	t.Run("income statement error propagates", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.splitsRangeErr = assert.AnError
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		_, err := svc.GenerateFullIncomeStatement(context.Background(), DateRangeParams{Month: "2025-03"})
		assert.Error(t, err)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestGenerateFullIncomeStatement -v`
Expected: compilation error — `GenerateFullIncomeStatement` undefined.

- [ ] **Step 3: Implement three `GenerateFull*` methods**

Add to `internal/service/report_service.go` (after `ResolveDateRange`):

```go
func (ts *TransactionService) GenerateFullIncomeStatement(ctx context.Context, params DateRangeParams) (*model.ReportResult, error) {
	start, end, period, err := ts.ResolveDateRange(params)
	if err != nil {
		return nil, err
	}

	result, err := ts.GenerateIncomeStatement(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to generate income statement: %w", err)
	}

	result.Period = period

	currentNetWorth, err := ts.GetNetWorthAt(ctx, end)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch net worth for current period: %w", err)
	}
	result.NetWorth = currentNetWorth

	_, prevEnd := previousPeriodRange(start, end)
	previousNetWorth, err := ts.GetNetWorthAt(ctx, prevEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch net worth for previous period: %w", err)
	}
	result.PreviousNetWorth = previousNetWorth
	result.NetWorthGrowthPct = computeNetWorthGrowthPctMap(currentNetWorth, previousNetWorth)

	return result, nil
}

func (ts *TransactionService) GenerateFullIncomeBreakdown(ctx context.Context, params DateRangeParams) (*model.ReportResult, error) {
	start, end, period, err := ts.ResolveDateRange(params)
	if err != nil {
		return nil, err
	}

	result, err := ts.GenerateIncomeBreakdown(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to generate income breakdown: %w", err)
	}

	result.Period = period
	return result, nil
}

func (ts *TransactionService) GenerateFullExpenseBreakdown(ctx context.Context, params DateRangeParams) (*model.ReportResult, error) {
	start, end, period, err := ts.ResolveDateRange(params)
	if err != nil {
		return nil, err
	}

	result, err := ts.GenerateExpenseBreakdown(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to generate expense breakdown: %w", err)
	}

	result.Period = period
	return result, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/ -run TestGenerateFullIncomeStatement -v`
Expected: PASS

- [ ] **Step 5: Update `ReportProvider` interface in cmd/report_types.go**

Replace the `ReportProvider` interface in `cmd/report_types.go` with:

```go
type ReportProvider interface {
	GenerateFullIncomeStatement(ctx context.Context, params service.DateRangeParams) (*model.ReportResult, error)
	GenerateFullIncomeBreakdown(ctx context.Context, params service.DateRangeParams) (*model.ReportResult, error)
	GenerateFullExpenseBreakdown(ctx context.Context, params service.DateRangeParams) (*model.ReportResult, error)
	GenerateBalanceSheet(ctx context.Context, asOf int64) (*model.BalanceSheetResult, error)
	ResolveDateRange(params service.DateRangeParams) (start, end int64, period string, err error)
}
```

Add `"github.com/hance08/kea/internal/service"` to the imports.

- [ ] **Step 6: Simplify cmd/report_actions.go**

Replace the entire content of `cmd/report_actions.go` with:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/service"
)

func (r *reportRunner) run(ctx context.Context) error {
	reportType := r.flags.Type
	if reportType == "" {
		reportType = "is"
	}

	switch reportType {
	case "is":
		return r.runIncomeStatement(ctx)
	case "ib":
		return r.runIncomeBreakdown(ctx)
	case "eb":
		return r.runExpenseBreakdown(ctx)
	case "bs":
		return r.runBalanceSheet(ctx)
	default:
		return fmt.Errorf("unknown report type %q — use: is, ib, eb, bs", reportType)
	}
}

func (r *reportRunner) dateRangeParams() service.DateRangeParams {
	return service.DateRangeParams{
		Month: r.flags.Month,
		From:  r.flags.From,
		To:    r.flags.To,
	}
}

func (r *reportRunner) runIncomeStatement(ctx context.Context) error {
	result, err := r.provider.GenerateFullIncomeStatement(ctx, r.dateRangeParams())
	if err != nil {
		return err
	}
	return r.view.RenderIncomeStatement(result)
}

func (r *reportRunner) runExpenseBreakdown(ctx context.Context) error {
	result, err := r.provider.GenerateFullExpenseBreakdown(ctx, r.dateRangeParams())
	if err != nil {
		return err
	}
	return r.view.RenderExpenseBreakdown(result)
}

func (r *reportRunner) runIncomeBreakdown(ctx context.Context) error {
	result, err := r.provider.GenerateFullIncomeBreakdown(ctx, r.dateRangeParams())
	if err != nil {
		return err
	}
	return r.view.RenderIncomeBreakdown(result)
}

func (r *reportRunner) runBalanceSheet(ctx context.Context) error {
	_, end, _, err := r.provider.ResolveDateRange(r.dateRangeParams())
	if err != nil {
		return err
	}

	result, err := r.provider.GenerateBalanceSheet(ctx, end)
	if err != nil {
		return fmt.Errorf("failed to generate balance sheet: %w", err)
	}

	return r.view.RenderBalanceSheet(result)
}
```

This deletes `resolveDateRange`, `parseMonth`, `parseDateRange`, `previousPeriodRange`, and `computeNetWorthGrowthPctMap` from cmd/.

- [ ] **Step 7: Run full test suite**

Run: `go test ./...`
Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go cmd/report_actions.go cmd/report_types.go
git commit -m "refactor: add GenerateFull* report methods and simplify cmd/ report layer

GenerateFullIncomeStatement, GenerateFullIncomeBreakdown, and
GenerateFullExpenseBreakdown now handle date resolution, net worth
fetching, and growth computation internally. cmd/report_actions.go
is now a thin shell that builds DateRangeParams and delegates.

Completes #76."
```

---

### Task 5: Final verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: all tests pass.

- [ ] **Step 2: Build the binary**

Run: `make build`
Expected: builds successfully.

- [ ] **Step 3: Verify no business logic remains in cmd/**

Run: `grep -n 'parseMonth\|parseDateRange\|previousPeriodRange\|computeNetWorthGrowthPctMap\|parseTransactionType\|parseStatus\b' cmd/*.go cmd/**/*.go`
Expected: no matches (only `determineMode` and `model.Parse*` calls remain).

- [ ] **Step 4: Verify service layer exports are usable**

Run: `grep -rn 'func.*TransactionService.*ResolveDateRange\|func.*TransactionService.*GenerateFull\|func.*TransactionService.*ParseTransactionDate\|func ParseTransaction' internal/`
Expected: shows all four new exported functions in `internal/service/` and `internal/model/`.
