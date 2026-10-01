# Fix ToJSONTxListItem ParseFloat Comma Bug Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `ToJSONTxListItem` so amounts >= 1,000 produce correct float64 values instead of 0.

**Architecture:** The root cause is in `ToJSONTxListItem` (`ui/views/json_types.go:112`) which calls `strconv.ParseFloat` on a comma-formatted string like `"1,234.56"`. The best fix is to strip commas before parsing. Additionally, the `TransactionListItem.Amount` field is populated via `fmt.Sprintf("%.2f", amountFloat)` in `cmd/transaction/list.go:181` — this produces `"1234.56"` (no commas) for the current code path, but the converter function should be robust regardless of input format. We also add a `model.TransactionListItem.Amount` field (raw `int64` cents) path analysis to confirm no other callers pass comma-formatted strings.

**Tech Stack:** Go, strconv, strings

---

### Task 1: Add failing test for comma-formatted amounts

**Files:**
- Modify: `ui/views/json_test.go:86-97`

- [ ] **Step 1: Write failing test for comma-formatted amount**

Add a new test case in `json_test.go` after the existing `TestToJSONTxListItem`:

```go
func TestToJSONTxListItem_commaAmount(t *testing.T) {
	item := TransactionListItem{
		ID: 10, Date: "2024-03-24", Type: "Income",
		Account: "Assets:Bank", Offset: "Revenue:Salary",
		Description: "salary", Amount: "1,234.56", Currency: "TWD", Status: "Cleared",
	}
	got := ToJSONTxListItem(item)
	assert.Equal(t, 1234.56, got.Amount)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ui/views/ -run TestToJSONTxListItem_commaAmount -v`
Expected: FAIL — `got 0, want 1234.56` because `strconv.ParseFloat("1,234.56", 64)` returns 0.

- [ ] **Step 3: Commit failing test**

```bash
git add ui/views/json_test.go
git commit -m "test: add failing test for comma-formatted amount in ToJSONTxListItem (#133)"
```

### Task 2: Fix ToJSONTxListItem to strip commas before parsing

**Files:**
- Modify: `ui/views/json_types.go:6-8` (imports)
- Modify: `ui/views/json_types.go:111-124` (ToJSONTxListItem function)

- [ ] **Step 1: Add `strings` to imports in `json_types.go`**

The current imports at `json_types.go:6-8` are:

```go
import (
	"strconv"
	"time"

	"github.com/hance08/kea/internal/model"
)
```

Change to:

```go
import (
	"strconv"
	"strings"
	"time"

	"github.com/hance08/kea/internal/model"
)
```

- [ ] **Step 2: Fix the ParseFloat call to strip commas**

The current `ToJSONTxListItem` at line 112:

```go
amount, _ := strconv.ParseFloat(item.Amount, 64)
```

Change to:

```go
amount, _ := strconv.ParseFloat(strings.ReplaceAll(item.Amount, ",", ""), 64)
```

- [ ] **Step 3: Run the failing test to verify it passes**

Run: `go test ./ui/views/ -run TestToJSONTxListItem_commaAmount -v`
Expected: PASS

- [ ] **Step 4: Run the full existing test to verify no regression**

Run: `go test ./ui/views/ -run TestToJSONTxListItem -v`
Expected: Both `TestToJSONTxListItem` and `TestToJSONTxListItem_commaAmount` PASS.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 6: Commit the fix**

```bash
git add ui/views/json_types.go
git commit -m "fix: strip commas from amount before ParseFloat in ToJSONTxListItem (#133)"
```

### Task 3: Add edge-case tests for negative and large comma-formatted amounts

**Files:**
- Modify: `ui/views/json_test.go`

- [ ] **Step 1: Add edge-case tests**

Add after the `TestToJSONTxListItem_commaAmount` test:

```go
func TestToJSONTxListItem_negativeCommaAmount(t *testing.T) {
	item := TransactionListItem{
		ID: 11, Date: "2024-03-24", Type: "Expense",
		Account: "Assets:Bank", Offset: "Expenses:Rent",
		Description: "rent", Amount: "-1,234.56", Currency: "TWD", Status: "Cleared",
	}
	got := ToJSONTxListItem(item)
	assert.Equal(t, -1234.56, got.Amount)
}

func TestToJSONTxListItem_largeCommaAmount(t *testing.T) {
	item := TransactionListItem{
		ID: 12, Date: "2024-03-24", Type: "Income",
		Account: "Assets:Bank", Offset: "Revenue:Salary",
		Description: "bonus", Amount: "1,234,567.89", Currency: "TWD", Status: "Cleared",
	}
	got := ToJSONTxListItem(item)
	assert.Equal(t, 1234567.89, got.Amount)
}
```

- [ ] **Step 2: Run the new tests to verify they pass**

Run: `go test ./ui/views/ -run "TestToJSONTxListItem_(negativeComma|largeComma)" -v`
Expected: PASS for both.

- [ ] **Step 3: Run all tests**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 4: Commit edge-case tests**

```bash
git add ui/views/json_test.go
git commit -m "test: add edge-case tests for negative and large comma amounts (#133)"
```
