# Remove SystemTransactionID Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the hardcoded `SystemTransactionID = 1` constant and all guard checks that use it, so opening-balance transactions are treated like normal transactions (only reconciliation protects them). Fixes #152.

**Architecture:** Delete the `SystemTransactionID` constant from `model/types.go`. Remove the three early-return guards in `transaction_ops.go` (`DeleteTransaction`, `UpdateTransactionStatus`, `UpdateTransactionComplete`). Remove the `NotEditableSystemTx` reason from `IsEditable` and simplify the enum. Update the `cmd/transaction/edit.go` switch case. Update all affected tests to remove or rewrite the system-transaction-ID test cases.

**Tech Stack:** Go, testify

---

### Task 1: Update tests for DeleteTransaction — remove system ID guard test, add ID=1 normal deletion test

**Files:**
- Modify: `internal/service/transaction_ops_test.go:428-433`

- [ ] **Step 1: Rewrite the "opening balance transaction (ID=1) rejected" test**

Replace the existing test that asserts `ErrNotEditable` for ID=1 with a test proving ID=1 can be deleted when it's a normal (non-reconciled) transaction:

```go
t.Run("transaction ID=1 is deletable when not reconciled", func(t *testing.T) {
    txRepo := newMockTransactionRepo()
    txRepo.addTransaction(&model.Transaction{ID: 1, Status: model.StatusCleared}, nil)
    svc := newTestTransactionService(newMockAccountRepo(), txRepo)

    err := svc.DeleteTransaction(context.Background(), 1)
    require.NoError(t, err)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestDeleteTransaction/transaction_ID=1 -v`
Expected: FAIL — the current code still has the `SystemTransactionID` guard that rejects ID=1.

- [ ] **Step 3: Commit failing test**

```bash
git add internal/service/transaction_ops_test.go
git commit -m "test: rewrite DeleteTransaction ID=1 test to expect success (#152)"
```

---

### Task 2: Update tests for UpdateTransactionStatus — remove system ID guard test, add ID=1 normal update test

**Files:**
- Modify: `internal/service/transaction_ops_test.go:518-523`

- [ ] **Step 1: Rewrite the "system transaction (ID=1) rejected" test**

Replace with a test proving ID=1 status can be updated:

```go
t.Run("transaction ID=1 status can be updated", func(t *testing.T) {
    txRepo := newMockTransactionRepo()
    txRepo.addTransaction(&model.Transaction{ID: 1, Status: model.StatusPending}, nil)
    svc := newTestTransactionService(newMockAccountRepo(), txRepo)

    err := svc.UpdateTransactionStatus(context.Background(), 1, model.StatusCleared)
    require.NoError(t, err)
    assert.Equal(t, model.StatusCleared, txRepo.transactions[1].Status)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run "TestUpdateTransactionStatus/transaction_ID=1" -v`
Expected: FAIL — current code rejects ID=1.

- [ ] **Step 3: Commit failing test**

```bash
git add internal/service/transaction_ops_test.go
git commit -m "test: rewrite UpdateTransactionStatus ID=1 test to expect success (#152)"
```

---

### Task 3: Update tests for IsEditable — remove system TX reason, keep reconciled check

**Files:**
- Modify: `internal/service/transaction_ops_test.go:899-928`

- [ ] **Step 1: Rewrite the IsEditable test suite**

Remove the "opening balance transaction (ID=1) is not editable" test case. Change the "ID=2 is editable" test to use ID=1 to prove ID=1 is now editable:

```go
func TestIsEditable(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	t.Run("regular transaction is editable", func(t *testing.T) {
		detail := &model.TransactionDetail{ID: 5}
		editable, reason := svc.IsEditable(detail)
		assert.True(t, editable)
		assert.Equal(t, EditableOK, reason)
	})

	t.Run("transaction ID=1 is editable", func(t *testing.T) {
		detail := &model.TransactionDetail{ID: 1}
		editable, reason := svc.IsEditable(detail)
		assert.True(t, editable)
		assert.Equal(t, EditableOK, reason)
	})

	t.Run("reconciled transaction is not editable", func(t *testing.T) {
		detail := &model.TransactionDetail{ID: 5, Status: model.StatusReconciled}
		editable, reason := svc.IsEditable(detail)
		assert.False(t, editable)
		assert.Equal(t, NotEditableReconciled, reason)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestIsEditable -v`
Expected: FAIL — `IsEditable` still returns `NotEditableSystemTx` for ID=1.

- [ ] **Step 3: Commit failing test**

```bash
git add internal/service/transaction_ops_test.go
git commit -m "test: rewrite IsEditable tests to expect ID=1 editable (#152)"
```

---

### Task 4: Remove SystemTransactionID constant and guards from service

**Files:**
- Modify: `internal/model/types.go:154` — delete the `SystemTransactionID` line
- Modify: `internal/service/transaction_ops.go:212-214` — delete the guard in `DeleteTransaction`
- Modify: `internal/service/transaction_ops.go:239-241` — delete the guard in `UpdateTransactionStatus`
- Modify: `internal/service/transaction_ops.go:265-267` — delete the guard in `UpdateTransactionComplete`
- Modify: `internal/service/transaction_ops.go:388-403` — simplify `IsEditable` and remove `NotEditableSystemTx`

- [ ] **Step 1: Delete `SystemTransactionID` from `internal/model/types.go`**

Remove line 154 (`SystemTransactionID int64 = 1`) from the const block.

- [ ] **Step 2: Remove the ID=1 guard from `DeleteTransaction`**

In `internal/service/transaction_ops.go`, delete lines 212-214:

```go
if txID == model.SystemTransactionID {
    return fmt.Errorf("cannot delete the initial opening transaction: %w", ErrNotEditable)
}
```

- [ ] **Step 3: Remove the ID=1 guard from `UpdateTransactionStatus`**

In `internal/service/transaction_ops.go`, delete lines 239-241:

```go
if txID == model.SystemTransactionID {
    return fmt.Errorf("transaction #%d cannot be modified: %w", txID, ErrNotEditable)
}
```

- [ ] **Step 4: Remove the ID=1 guard from `UpdateTransactionComplete`**

In `internal/service/transaction_ops.go`, delete lines 265-267:

```go
if input.ID == model.SystemTransactionID {
    return fmt.Errorf("cannot modify the initial opening transaction: %w", ErrNotEditable)
}
```

- [ ] **Step 5: Simplify `IsEditable` and remove `NotEditableSystemTx`**

Replace the `NotEditableReason` enum and `IsEditable` function with:

```go
type NotEditableReason int

const (
	EditableOK            NotEditableReason = 0
	NotEditableReconciled NotEditableReason = 1
)

func (ts *TransactionService) IsEditable(detail *model.TransactionDetail) (bool, NotEditableReason) {
	if detail.Status == model.StatusReconciled {
		return false, NotEditableReconciled
	}
	return true, EditableOK
}
```

- [ ] **Step 6: Run all tests to verify they pass**

Run: `go test ./internal/service/ -v`
Expected: All tests pass, including the rewritten tests from Tasks 1-3.

- [ ] **Step 7: Commit**

```bash
git add internal/model/types.go internal/service/transaction_ops.go
git commit -m "fix: remove SystemTransactionID constant and ID-based guards (#152)

Opening-balance transactions are no longer identified by numeric ID.
Only reconciliation status protects transactions from modification."
```

---

### Task 5: Update cmd layer — remove NotEditableSystemTx switch case

**Files:**
- Modify: `cmd/transaction/edit.go:59-61`
- Modify: `cmd/transaction/edit_actions_test.go:56`

- [ ] **Step 1: Simplify the switch in `cmd/transaction/edit.go`**

Replace the `switch reason` block (lines 58-65) with:

```go
isEditable, reason := r.txSvc.IsEditable(detail)
if !isEditable {
    if reason == service.NotEditableReconciled {
        r.view.ShowWarning("This transaction cannot be edited (Reconciled Transaction)")
    }
    return nil
}
```

- [ ] **Step 2: Update the stub in `cmd/transaction/edit_actions_test.go`**

Check that the `stubEditProvider.IsEditable` mock at line 56 still compiles. It returns `(bool, service.NotEditableReason)` — the type still exists, so no change needed unless it references `NotEditableSystemTx`. If it does, update accordingly.

- [ ] **Step 3: Run cmd tests**

Run: `go test ./cmd/transaction/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/transaction/edit.go cmd/transaction/edit_actions_test.go
git commit -m "fix(cmd): remove NotEditableSystemTx switch case from edit command (#152)"
```

---

### Task 6: Full test suite verification

- [ ] **Step 1: Run the entire test suite**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 2: Build the binary**

Run: `make build`
Expected: Build succeeds with no errors.
