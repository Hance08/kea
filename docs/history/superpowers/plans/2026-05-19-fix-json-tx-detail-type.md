# Fix JSONTxDetail Missing Type Field — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the missing `Type` field to `JSONTxDetail` so `kea transaction show --json` includes the transaction type, matching the contract already established by `JSONTxListItem`.

**Architecture:** Add `Type string` JSON field to the `JSONTxDetail` struct and populate it in `ToJSONTxDetail` using `string(d.Type)` — the same pattern used by `ToJSONTxListItem`. Update the existing test to cover the new field.

**Tech Stack:** Go, testify

---

### Task 1: Add Type field to JSONTxDetail and its converter

**Files:**
- Modify: `ui/views/json_types.go:38-44` (struct) and `ui/views/json_types.go:95-107` (converter)
- Modify: `ui/views/json_test.go:47-65` (existing test)

- [ ] **Step 1: Update the existing test to assert on the Type field**

In `ui/views/json_test.go`, the `TestToJSONTxDetail` function constructs a `model.TransactionDetail` but does not set `Type`. Add `Type: model.TxTypeExpense` to the test input and assert `got.Type == "Expense"`.

```go
// In TestToJSONTxDetail, update the detail construction (line 48-57):
detail := &model.TransactionDetail{
    ID:          42,
    Timestamp:   1711238400, // 2024-03-24
    Description: "Buy coffee",
    Status:      model.StatusCleared,
    Type:        model.TxTypeExpense,
    Splits: []model.SplitDetail{
        {ID: 1, AccountID: 10, AccountName: "Assets:Cash", AccountType: model.AccountTypeAsset, Amount: -500, Currency: "TWD", Memo: ""},
        {ID: 2, AccountID: 20, AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500, Currency: "TWD", Memo: "lunch"},
    },
}
```

Add this assertion after line 64:

```go
assert.Equal(t, "Expense", got.Type)
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./ui/views/ -run TestToJSONTxDetail -v`
Expected: FAIL — `got.Type` is `""` because `JSONTxDetail` has no `Type` field yet.

- [ ] **Step 3: Add Type field to JSONTxDetail struct**

In `ui/views/json_types.go`, add `Type` to the struct (between `Description` and `Status` to match `JSONTxListItem` ordering):

```go
type JSONTxDetail struct {
	ID          int64             `json:"id"`
	Date        string            `json:"date"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Status      string            `json:"status"`
	Splits      []JSONSplitDetail `json:"splits"`
}
```

- [ ] **Step 4: Populate Type in ToJSONTxDetail converter**

In `ui/views/json_types.go`, update the `ToJSONTxDetail` return (line 100-106):

```go
return JSONTxDetail{
    ID:          d.ID,
    Date:        time.Unix(d.Timestamp, 0).Format(model.DateFormat),
    Description: d.Description,
    Type:        string(d.Type),
    Status:      d.Status.String(),
    Splits:      splits,
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./ui/views/ -run TestToJSONTxDetail -v`
Expected: PASS

- [ ] **Step 6: Run the full test suite**

Run: `go test ./...`
Expected: All tests pass, no regressions.

- [ ] **Step 7: Commit**

```bash
git add ui/views/json_types.go ui/views/json_test.go
git commit -m "fix: add missing Type field to JSONTxDetail (#121)"
```
