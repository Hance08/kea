# Validate Transaction Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent callers from bypassing the reconciliation workflow by creating already-reconciled transactions or updating transactions to reconciled status outside ReconcileTransactions.

**Architecture:** Two surgical changes in `internal/service/transaction_ops.go`: (1) add status validation to `CreateTransaction` rejecting anything other than Pending/Cleared, (2) tighten `UpdateTransactionComplete` validation to also reject StatusReconciled — matching the existing `UpdateTransactionStatus` behavior.

**Tech Stack:** Go, testify (assert/require)

---

### Task 1: Add status validation to CreateTransaction

**Files:**
- Modify: `internal/service/transaction_ops.go:24-35`
- Test: `internal/service/transaction_ops_test.go`

- [ ] **Step 1: Write failing tests for CreateTransaction status validation**

Add two sub-tests to the existing `TestCreateTransaction` function in `internal/service/transaction_ops_test.go`. Insert them after the existing sub-tests (after the last `t.Run` block inside `TestCreateTransaction`):

```go
t.Run("status reconciled(2) rejected on create", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    setupStandardAccounts(accRepo)
    svc := newTestTransactionService(accRepo, newMockTransactionRepo())

    input := model.TransactionDetail{
        Type:   model.TxTypeExpense,
        Status: model.StatusReconciled,
        Splits: []model.SplitDetail{
            {AccountName: "Expenses:Food", Amount: 500},
            {AccountName: "Assets:Bank", Amount: -500},
        },
    }
    _, err := svc.CreateTransaction(context.Background(), input)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "invalid status")

    var ve *ValidationError
    assert.True(t, errors.As(err, &ve))
    assert.Equal(t, "status", ve.Field)
})

t.Run("invalid status(99) rejected on create", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    setupStandardAccounts(accRepo)
    svc := newTestTransactionService(accRepo, newMockTransactionRepo())

    input := model.TransactionDetail{
        Type:   model.TxTypeExpense,
        Status: model.TransactionStatus(99),
        Splits: []model.SplitDetail{
            {AccountName: "Expenses:Food", Amount: 500},
            {AccountName: "Assets:Bank", Amount: -500},
        },
    }
    _, err := svc.CreateTransaction(context.Background(), input)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "invalid status")

    var ve *ValidationError
    assert.True(t, errors.As(err, &ve))
    assert.Equal(t, "status", ve.Field)
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestCreateTransaction/status_reconciled|TestCreateTransaction/invalid_status.99._rejected_on_create" -v`
Expected: FAIL — both tests pass without error because no validation exists yet.

- [ ] **Step 3: Add status validation to CreateTransaction**

In `internal/service/transaction_ops.go`, add the following validation block after the existing type validation (after line 35, `return 0, validationErrorf("type", "transaction type is required")`), before the timestamp default:

```go
if input.Status != model.StatusPending && input.Status != model.StatusCleared {
    return 0, validationErrorf("status", "invalid status: new transactions must be Pending or Cleared")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestCreateTransaction -v`
Expected: ALL sub-tests PASS, including both new ones.

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "fix: validate status in CreateTransaction — reject Reconciled and invalid values

Closes #113 (part 1/2)"
```

---

### Task 2: Remove StatusReconciled from UpdateTransactionComplete

**Files:**
- Modify: `internal/service/transaction_ops.go:251-254`
- Test: `internal/service/transaction_ops_test.go`

- [ ] **Step 1: Write failing test for UpdateTransactionComplete rejecting reconciled status**

Add a new sub-test to the existing `TestUpdateTransactionComplete` function in `internal/service/transaction_ops_test.go`. Insert it after the existing `"invalid status(99) rejected"` sub-test:

```go
t.Run("setting status to reconciled(2) rejected", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    txRepo := newMockTransactionRepo()
    setupStandardAccounts(accRepo)
    txRepo.addTransaction(
        &model.Transaction{ID: 5, Status: model.StatusPending},
        makeExistingSplits(10, 11),
    )
    svc := newTestTransactionService(accRepo, txRepo)

    splits := []model.SplitDetail{
        {ID: 10, AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -800, Currency: "USD"},
        {ID: 11, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 800, Currency: "USD"},
    }
    err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{
        ID: 5, Description: "Trying reconciled", Timestamp: 0,
        Status: model.StatusReconciled, Type: model.TxTypeExpense, Splits: splits,
    })
    require.Error(t, err)
    assert.Contains(t, err.Error(), "invalid status")

    var ve *ValidationError
    assert.True(t, errors.As(err, &ve))
    assert.Equal(t, "status", ve.Field)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run "TestUpdateTransactionComplete/setting_status_to_reconciled" -v`
Expected: FAIL — currently StatusReconciled is allowed.

- [ ] **Step 3: Tighten UpdateTransactionComplete status validation**

In `internal/service/transaction_ops.go`, change the status validation at lines 251-254 from:

```go
// Validate status
if input.Status != model.StatusPending && input.Status != model.StatusCleared && input.Status != model.StatusReconciled {
    return validationErrorf("status", "invalid status: must be 0 (Pending), 1 (Cleared) or 2 (Reconciled)")
}
```

to:

```go
if input.Status != model.StatusPending && input.Status != model.StatusCleared {
    return validationErrorf("status", "invalid status: must be Pending or Cleared")
}
```

- [ ] **Step 4: Run all tests to verify they pass**

Run: `go test ./internal/service/ -run "TestUpdateTransactionComplete" -v`
Expected: ALL sub-tests PASS including the new one. The existing `"invalid status(99) rejected"` test should still pass.

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: ALL tests pass — no regressions.

- [ ] **Step 6: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "fix: reject StatusReconciled in UpdateTransactionComplete

Reconciliation must go through the ReconcileTransactions workflow.
Closes #113 (part 2/2)"
```
