# Fix Currency Consistency on Transaction Edit — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce single-currency-per-transaction invariant on the update/edit path, matching the create path's existing validation.

**Architecture:** Add a `ValidateSplitDetailsCurrency` helper on `TransactionService` that checks currency consistency across `[]model.SplitDetail`. Call it from both `UpdateTransactionComplete` and `ValidateTransactionEdit`. TDD: write failing tests first, then add the validation.

**Tech Stack:** Go, white-box service tests with hand-written mocks

---

### Task 1: Add currency consistency check to `UpdateTransactionComplete`

**Files:**
- Modify: `internal/service/transaction_validation.go` (add helper)
- Modify: `internal/service/transaction_ops.go:274-281` (call helper)
- Test: `internal/service/transaction_ops_test.go` (add test case)
- Test: `internal/service/transaction_validation_test.go` (add test case)

- [ ] **Step 1: Write failing test for `UpdateTransactionComplete` with mixed currencies**

Add this test case inside `TestUpdateTransactionComplete` in `internal/service/transaction_ops_test.go`, after the `"unbalanced splits rejected"` subtest (around line 570):

```go
t.Run("mixed currency splits rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    txRepo := newMockTransactionRepo()
    setupStandardAccounts(accRepo)
    txRepo.addTransaction(
        &model.Transaction{ID: 5, Status: model.StatusPending},
        makeExistingSplits(10, 11),
    )
    svc := newTestTransactionService(accRepo, txRepo)

    splits := []model.SplitDetail{
        {ID: 10, AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -1000, Currency: "USD"},
        {ID: 11, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 1000, Currency: "TWD"},
    }
    err := svc.UpdateTransactionComplete(context.Background(), 5, "Mixed", 0, model.StatusPending, model.TxTypeExpense, splits)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "currency")
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/service/ -run TestUpdateTransactionComplete/mixed_currency_splits_rejected -v`
Expected: FAIL — the update currently succeeds because no currency check exists.

- [ ] **Step 3: Add `ValidateSplitDetailsCurrency` helper**

Add this function in `internal/service/transaction_validation.go`, after `ValidateSplitsBalance`:

```go
// ValidateSplitDetailsCurrency checks that all SplitDetail entries use the same currency.
func (ts *TransactionService) ValidateSplitDetailsCurrency(splits []model.SplitDetail) error {
	var firstCurrency string
	for _, split := range splits {
		if firstCurrency == "" {
			firstCurrency = split.Currency
		} else if split.Currency != firstCurrency {
			return fmt.Errorf("splits must all use the same currency (got %q and %q)", firstCurrency, split.Currency)
		}
	}
	return nil
}
```

- [ ] **Step 4: Call the helper in `UpdateTransactionComplete`**

In `internal/service/transaction_ops.go`, add a currency check right after the existing balance check (after line 281, before the `ValidateSplitsMatchType` call):

```go
// Validate currency consistency
if err := ts.ValidateSplitDetailsCurrency(splits); err != nil {
    return err
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/service/ -run TestUpdateTransactionComplete/mixed_currency_splits_rejected -v`
Expected: PASS

- [ ] **Step 6: Run the full test suite to check for regressions**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/service/transaction_validation.go internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "fix: add currency consistency check to UpdateTransactionComplete

Closes #53"
```

---

### Task 2: Add currency consistency check to `ValidateTransactionEdit`

**Files:**
- Modify: `internal/service/transaction_validation.go:39-63` (call helper)
- Test: `internal/service/transaction_validation_test.go` (add test cases)

- [ ] **Step 1: Write failing test for `ValidateTransactionEdit` with mixed currencies**

Add this test case inside `TestValidateTransactionEdit` in `internal/service/transaction_validation_test.go`, after the `"multiple balanced splits pass validation"` subtest (around line 199):

```go
t.Run("mixed currency splits rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
    accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense})

    svc := newTestTransactionService(accRepo, newMockTransactionRepo())
    splits := []model.SplitDetail{
        {AccountID: 1, Amount: -1000, Currency: "USD"},
        {AccountID: 2, Amount: 1000, Currency: "TWD"},
    }
    err := svc.ValidateTransactionEdit(context.Background(), splits)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "currency")
})

t.Run("same currency splits pass currency check", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
    accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense})

    svc := newTestTransactionService(accRepo, newMockTransactionRepo())
    splits := []model.SplitDetail{
        {AccountID: 1, Amount: -1000, Currency: "TWD"},
        {AccountID: 2, Amount: 1000, Currency: "TWD"},
    }
    err := svc.ValidateTransactionEdit(context.Background(), splits)
    require.NoError(t, err)
})
```

- [ ] **Step 2: Run the test to verify the mixed-currency case fails**

Run: `go test ./internal/service/ -run TestValidateTransactionEdit/mixed_currency_splits_rejected -v`
Expected: FAIL — `ValidateTransactionEdit` currently has no currency check.

- [ ] **Step 3: Add currency check to `ValidateTransactionEdit`**

In `internal/service/transaction_validation.go`, add the currency check in `ValidateTransactionEdit` right after the balance check (after the `if total != 0` block, before the account-existence loop):

```go
// Check currency consistency
if err := ts.ValidateSplitDetailsCurrency(splits); err != nil {
    return err
}
```

- [ ] **Step 4: Run both new tests to verify they pass**

Run: `go test ./internal/service/ -run "TestValidateTransactionEdit/(mixed_currency|same_currency)" -v`
Expected: Both PASS.

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/service/transaction_validation.go internal/service/transaction_validation_test.go
git commit -m "fix: add currency consistency check to ValidateTransactionEdit"
```
