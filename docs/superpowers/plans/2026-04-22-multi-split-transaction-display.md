# Multi-Split Transaction Display Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `DetermineType` so that multi-split transactions where Asset/Liability flow dominates a minor Expense split are correctly classified as Transfer instead of Expense.

**Architecture:** Add a `totalPositiveAssetLiabAmount` accumulator to the existing loop in `DetermineType`, then split the single `hasExpense && assetOrLiabCnt >= 1` condition into two paths — one for `assetOrLiabCnt >= 2` (applies dominance check) and one for `assetOrLiabCnt == 1` (unchanged Expense behaviour). No other functions need changes.

**Tech Stack:** Go, testify (`github.com/stretchr/testify`)

---

### Task 1: Add failing tests for the new classification behaviour

**Files:**
- Modify: `internal/service/transaction_classifier_test.go`

- [ ] **Step 1: Add three new cases to `TestDetermineType`**

Open `internal/service/transaction_classifier_test.go` and append the following three entries inside the `tests` slice in `TestDetermineType` (after the existing `"unclassifiable → Other"` case, before the closing `}`):

```go
{
    name: "transfer with fee: A/L dominates E → Transfer",
    splits: []model.SplitDetail{
        split("Assets:Investments:00878", model.AccountTypeAsset, 5160),
        split("Expenses:Fees:Stocks", model.AccountTypeExpense, 7),
        split("Assets:Bank:DAWHO", model.AccountTypeAsset, -5167),
    },
    want: model.TxTypeTransfer,
},
{
    name: "expense split bill: E ties A/L → Expense",
    splits: []model.SplitDetail{
        split("Expenses:Food:Drink", model.AccountTypeExpense, 60),
        split("Assets:Receivable:Friends", model.AccountTypeAsset, 60),
        split("Assets:Ewallet:LinePayMoney", model.AccountTypeAsset, -120),
    },
    want: model.TxTypeExpense,
},
{
    name: "expense dominant: E > A/L → Expense",
    splits: []model.SplitDetail{
        split("Expenses:Food", model.AccountTypeExpense, 100),
        split("Assets:Receivable:Friends", model.AccountTypeAsset, 40),
        split("Assets:Bank", model.AccountTypeAsset, -140),
    },
    want: model.TxTypeExpense,
},
```

- [ ] **Step 2: Run the new tests to confirm they fail**

```bash
go test ./internal/service/ -run TestDetermineType -v
```

Expected: the three new cases FAIL. Existing cases still PASS. You should see output like:
```
--- FAIL: TestDetermineType/transfer_with_fee:_A/L_dominates_E_→_Transfer
--- FAIL: TestDetermineType/expense_split_bill:_E_ties_A/L_→_Expense
--- FAIL: TestDetermineType/expense_dominant:_E_>_A/L_→_Expense
```

---

### Task 2: Fix `DetermineType` to apply dominance check

**Files:**
- Modify: `internal/service/transaction_classifier.go`

- [ ] **Step 1: Add `totalPositiveAssetLiabAmount` variable declaration**

In `DetermineType`, find the `var (` block that declares `hasExpense`, `hasRevenue`, etc. (around line 17). Add the new variable alongside `totalExpenseAmount`:

```go
var (
    hasExpense      bool
    hasRevenue      bool
    hasEquity       bool
    assetOrLiabCnt  int
    isOpening       bool
    isAssetIncrease bool
)

var totalRevenueAmount int64
var totalExpenseAmount int64
var totalPositiveAssetLiabAmount int64
```

- [ ] **Step 2: Accumulate `totalPositiveAssetLiabAmount` in the loop**

Find the `case "A":` and `case "L":` handling in the `switch accType` block. Replace it with:

```go
case "A":
    assetOrLiabCnt++
    if split.Amount > 0 {
        isAssetIncrease = true
        totalPositiveAssetLiabAmount += split.Amount
    }
case "L":
    assetOrLiabCnt++
    if split.Amount > 0 {
        totalPositiveAssetLiabAmount += split.Amount
    }
```

- [ ] **Step 3: Replace the single `hasExpense` condition with two paths**

Find this block (around line 70):

```go
if hasExpense && assetOrLiabCnt >= 1 {
    return model.TxTypeExpense, nil
}
```

Replace it with:

```go
if hasExpense && assetOrLiabCnt >= 2 {
    if totalPositiveAssetLiabAmount > totalExpenseAmount {
        return model.TxTypeTransfer, nil
    }
    return model.TxTypeExpense, nil
}
if hasExpense && assetOrLiabCnt == 1 {
    return model.TxTypeExpense, nil
}
```

- [ ] **Step 4: Run all classifier tests to confirm they pass**

```bash
go test ./internal/service/ -run TestDetermineType -v
```

Expected: ALL cases PASS including the three new ones.

- [ ] **Step 5: Run the full test suite to check for regressions**

```bash
go test ./...
```

Expected: all tests PASS, no failures.

- [ ] **Step 6: Commit**

```bash
git add internal/service/transaction_classifier.go internal/service/transaction_classifier_test.go
git commit -m "fix: classify multi-split A/L-dominant transactions as Transfer

When a transaction has 2+ Asset/Liability splits alongside an Expense
split, compare positive A/L flow against total Expense amount. If A/L
dominates, classify as Transfer. Ties and E-dominant cases remain Expense."
```
