# Fix: MarkSplitsReconciledByAccount Silent Zero-Row Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent `MarkSplitsReconciledByAccount` from silently succeeding when no splits matched the given `(accountID, txIDs)` pair by returning `rowsAffected` and checking it in the calling service.

**Architecture:** Change the repository interface method signature to `(int64, error)`, capture the SQL result's `RowsAffected()` in the store, update the service to treat `rowsAffected < len(txIDs)` as a hard error, and update the mock + add a regression test.

**Tech Stack:** Go standard library (`database/sql`), existing mock infrastructure in `internal/service/testhelper_test.go`.

---

## File Map

| File | Change |
|---|---|
| `internal/repository/interfaces.go:63` | Change `MarkSplitsReconciledByAccount` signature from `error` to `(int64, error)` |
| `internal/store/sqlite_reconcile.go:76-100` | Capture `sql.Result` from `Exec`, call `RowsAffected()`, return it |
| `internal/service/reconcile_ops.go:80` | Handle new `(int64, error)` return; add guard `rowsAffected < int64(len(txIDs))` |
| `internal/service/testhelper_test.go` | Add `markSplitsRowsOverride *int64` field; update mock method signature |
| `internal/service/reconcile_ops_test.go` | Add test `TestReconcileTransactions_MarkSplitsAffectsFewerRowsThanTxIDs_ReturnsError` |

---

### Task 1: Update interface and fix compilation

**Files:**
- Modify: `internal/repository/interfaces.go:63`
- Modify: `internal/store/sqlite_reconcile.go:76-100`
- Modify: `internal/service/testhelper_test.go` (mock method + new field)

- [ ] **Step 1: Change the interface signature**

In `internal/repository/interfaces.go`, change line 63:

```go
// Before:
MarkSplitsReconciledByAccount(accountID int64, txIDs []int64) error

// After:
MarkSplitsReconciledByAccount(accountID int64, txIDs []int64) (int64, error)
```

- [ ] **Step 2: Verify compilation now fails**

Run:
```bash
go build ./...
```

Expected: compile errors in `internal/store/sqlite_reconcile.go` and `internal/service/testhelper_test.go` (method signatures no longer satisfy the interface).

- [ ] **Step 3: Update the store implementation to capture and return rows affected**

Replace the `MarkSplitsReconciledByAccount` function body in `internal/store/sqlite_reconcile.go` (lines 76–100) with:

```go
func (s *Store) MarkSplitsReconciledByAccount(accountID int64, txIDs []int64) (int64, error) {
	if len(txIDs) == 0 {
		return 0, nil
	}

	placeholders := strings.Repeat("?,", len(txIDs))
	placeholders = placeholders[:len(placeholders)-1]

	// 1. Mark the account's splits as reconciled.
	splitArgs := make([]any, 0, len(txIDs)+1)
	splitArgs = append(splitArgs, accountID)
	for _, id := range txIDs {
		splitArgs = append(splitArgs, id)
	}
	splitQuery := fmt.Sprintf(
		"UPDATE splits SET reconciled = 1 WHERE account_id = ? AND transaction_id IN (%s)",
		placeholders,
	)
	result, err := s.db.Exec(splitQuery, splitArgs...)
	if err != nil {
		return 0, fmt.Errorf("failed to mark splits as reconciled: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected from splits update: %w", err)
	}

	// 2. Upgrade all affected transactions to StatusReconciled.
	return rowsAffected, s.BulkUpdateTransactionStatus(txIDs, model.StatusReconciled)
}
```

- [ ] **Step 4: Add `markSplitsRowsOverride` field to the mock and update its method signature**

In `internal/service/testhelper_test.go`, add the field to `mockTransactionRepo` (after the existing `markSplitsReconciledCalls` field, around line 237):

```go
	markSplitsReconciledCalls []struct {
		accountID int64
		txIDs     []int64
	}
	markSplitsRowsOverride *int64 // nil = return len(txIDs); non-nil = return *markSplitsRowsOverride
```

Then update the `MarkSplitsReconciledByAccount` method on `mockTransactionRepo` (around line 437):

```go
func (m *mockTransactionRepo) MarkSplitsReconciledByAccount(accountID int64, txIDs []int64) (int64, error) {
	if m.markSplitsReconciledErr != nil {
		return 0, m.markSplitsReconciledErr
	}
	ids := make([]int64, len(txIDs))
	copy(ids, txIDs)
	m.markSplitsReconciledCalls = append(m.markSplitsReconciledCalls, struct {
		accountID int64
		txIDs     []int64
	}{accountID, ids})
	if m.markSplitsRowsOverride != nil {
		return *m.markSplitsRowsOverride, nil
	}
	return int64(len(txIDs)), nil
}
```

- [ ] **Step 5: Verify compilation passes**

Run:
```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Run existing tests to confirm nothing regressed**

Run:
```bash
go test ./...
```

Expected: all tests PASS (the service still reads the first return value but ignores it; this step confirms the plumbing change itself doesn't break anything).

---

### Task 2: Write a failing test for the guard

**Files:**
- Modify: `internal/service/reconcile_ops_test.go`

- [ ] **Step 1: Add the new test**

Append to `internal/service/reconcile_ops_test.go`:

```go
func TestReconcileTransactions_MarkSplitsAffectsFewerRowsThanTxIDs_ReturnsError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
		{ID: 11, Amount: -50000},
	})
	// Simulate the store updating only 1 row even though 2 txIDs were requested —
	// meaning txID 11 had no split for this account.
	one := int64(1)
	txRepo.markSplitsRowsOverride = &one

	_, err := svc.ReconcileTransactions(1, 50000, []int64{10, 11})

	if err == nil {
		t.Fatal("expected error when fewer rows were affected than txIDs, got nil")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when the rows guard fires")
	}
}
```

- [ ] **Step 2: Run the new test and confirm it fails**

Run:
```bash
go test ./internal/service/ -run TestReconcileTransactions_MarkSplitsAffectsFewerRowsThanTxIDs_ReturnsError -v
```

Expected: FAIL — the service doesn't yet check `rowsAffected`, so `err` is `nil` and `t.Fatal` fires.

---

### Task 3: Add the guard in the service and verify all tests pass

**Files:**
- Modify: `internal/service/reconcile_ops.go:80`

- [ ] **Step 1: Update `ReconcileTransactions` to check rows affected**

In `internal/service/reconcile_ops.go`, replace step 5 (around line 78–82):

```go
	// Before:
	if err := ts.txRepo.MarkSplitsReconciledByAccount(accountID, txIDs); err != nil {
		return 0, fmt.Errorf("failed to reconcile transactions: %w", err)
	}

	// After:
	rowsAffected, err := ts.txRepo.MarkSplitsReconciledByAccount(accountID, txIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to reconcile transactions: %w", err)
	}
	if rowsAffected < int64(len(txIDs)) {
		return 0, fmt.Errorf(
			"reconcile: expected splits for %d transactions to be marked, but only %d rows were affected; transaction IDs may not have a split for this account",
			len(txIDs), rowsAffected,
		)
	}
```

- [ ] **Step 2: Run the new test and verify it passes**

Run:
```bash
go test ./internal/service/ -run TestReconcileTransactions_MarkSplitsAffectsFewerRowsThanTxIDs_ReturnsError -v
```

Expected: PASS.

- [ ] **Step 3: Run the full test suite**

Run:
```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/repository/interfaces.go \
        internal/store/sqlite_reconcile.go \
        internal/service/reconcile_ops.go \
        internal/service/testhelper_test.go \
        internal/service/reconcile_ops_test.go
git commit -m "fix: return rowsAffected from MarkSplitsReconciledByAccount and guard in service

Fixes #27. The store UPDATE could silently affect 0 rows if a caller passed
a transaction ID with no split for the given account. Now the store returns
the affected row count; the service errors if rowsAffected < len(txIDs)."
```
