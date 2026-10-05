# Consistent ErrNotFound Translation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure every service method that calls a repo lookup translates `repository.ErrNotFound` → `service.ErrNotFound` so the web layer can reliably distinguish 404 from 500.

**Architecture:** Fix 5 service methods that leak raw `repository.ErrNotFound`, and fix the transaction mock so its `GetTransactionByID` wraps `repository.ErrNotFound` (matching the account mock pattern). No helper function — the issue lists only 5 call sites and the inline pattern is clear.

**Tech Stack:** Go, standard `errors` package

---

### Task 1: Fix transaction mock to wrap `repository.ErrNotFound`

The mock `GetTransactionByID` returns a plain error string when a transaction isn't found, unlike the account mock which wraps `repository.ErrNotFound`. This must be fixed first so the new translation tests can work.

**Files:**
- Modify: `internal/service/testhelper_test.go:310`

- [ ] **Step 1: Fix mock to wrap `repository.ErrNotFound`**

In `testhelper_test.go`, change the `GetTransactionByID` not-found return (line 310) from:

```go
return nil, fmt.Errorf("transaction ID %d not found", txID)
```

to:

```go
return nil, fmt.Errorf("transaction ID %d not found: %w", txID, repository.ErrNotFound)
```

- [ ] **Step 2: Run existing tests to verify no regressions**

Run: `go test ./internal/service/ -v -run "TestDeleteTransaction|TestUpdateTransactionStatus|TestUpdateTransactionComplete|TestGetTransactionByID" 2>&1 | tail -30`

Expected: All existing tests PASS. The "non-existent transaction returns error" subtests still pass because they only check `require.Error(t, err)` without asserting the error type.

- [ ] **Step 3: Commit**

```bash
git add internal/service/testhelper_test.go
git commit -m "fix: wrap repository.ErrNotFound in transaction mock GetTransactionByID (#106)"
```

---

### Task 2: Translate ErrNotFound in `GetTransactionByID`

**Files:**
- Modify: `internal/service/transaction_service.go:53-55`
- Modify: `internal/service/transaction_service_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Add to `internal/service/transaction_service_test.go`:

```go
func TestGetTransactionByID_NotFound_WrapsServiceErrNotFound(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	_, err := svc.GetTransactionByID(context.Background(), 999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound), "expected service.ErrNotFound, got: %v", err)
	assert.Contains(t, err.Error(), "transaction")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -v -run TestGetTransactionByID_NotFound 2>&1 | tail -10`

Expected: FAIL — the error wraps `repository.ErrNotFound`, not `service.ErrNotFound`.

- [ ] **Step 3: Add translation in `GetTransactionByID`**

In `internal/service/transaction_service.go`, change lines 53-55 from:

```go
tx, err := ts.txRepo.GetTransactionByID(ctx, txID)
if err != nil {
    return nil, fmt.Errorf("failed to retrieve transaction %d: %w", txID, err)
}
```

to:

```go
tx, err := ts.txRepo.GetTransactionByID(ctx, txID)
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        return nil, fmt.Errorf("transaction #%d: %w", txID, ErrNotFound)
    }
    return nil, fmt.Errorf("failed to retrieve transaction %d: %w", txID, err)
}
```

Ensure `"errors"` is in the import block (it's already imported via `repository`'s usage pattern — verify).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -v -run "TestGetTransactionByID" 2>&1 | tail -15`

Expected: Both `TestGetTransactionByID_NotFound_WrapsServiceErrNotFound` and `TestGetTransactionByID_RepoError_WrapsWithContext` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_service.go internal/service/transaction_service_test.go
git commit -m "fix: translate repository.ErrNotFound in GetTransactionByID (#106)"
```

---

### Task 3: Translate ErrNotFound in `DeleteTransaction`

**Files:**
- Modify: `internal/service/transaction_ops.go:216-218`
- Modify: `internal/service/transaction_ops_test.go` (update existing test)

- [ ] **Step 1: Write the failing test**

In `internal/service/transaction_ops_test.go`, update the existing subtest `"non-existent transaction returns error"` (inside `TestDeleteTransaction`) to assert `ErrNotFound`:

```go
t.Run("non-existent transaction returns ErrNotFound", func(t *testing.T) {
    svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
    err := svc.DeleteTransaction(context.Background(), 999)
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound, got: %v", err)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -v -run "TestDeleteTransaction/non-existent" 2>&1 | tail -10`

Expected: FAIL — error wraps `repository.ErrNotFound`, not `service.ErrNotFound`.

- [ ] **Step 3: Add translation in `DeleteTransaction`**

In `internal/service/transaction_ops.go`, change lines 216-218 from:

```go
tx, err := ts.txRepo.GetTransactionByID(ctx, txID)
if err != nil {
    return fmt.Errorf("failed to get transaction: %w", err)
}
```

to:

```go
tx, err := ts.txRepo.GetTransactionByID(ctx, txID)
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        return fmt.Errorf("transaction #%d: %w", txID, ErrNotFound)
    }
    return fmt.Errorf("failed to get transaction: %w", err)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -v -run TestDeleteTransaction 2>&1 | tail -20`

Expected: All subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "fix: translate repository.ErrNotFound in DeleteTransaction (#106)"
```

---

### Task 4: Translate ErrNotFound in `UpdateTransactionStatus`

**Files:**
- Modify: `internal/service/transaction_ops.go:240-242`
- Modify: `internal/service/transaction_ops_test.go` (update existing test)

- [ ] **Step 1: Write the failing test**

In `internal/service/transaction_ops_test.go`, update the existing subtest `"non-existent transaction returns error"` (inside `TestUpdateTransactionStatus`) to assert `ErrNotFound`:

```go
t.Run("non-existent transaction returns ErrNotFound", func(t *testing.T) {
    svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
    err := svc.UpdateTransactionStatus(context.Background(), 999, model.StatusCleared)
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound, got: %v", err)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -v -run "TestUpdateTransactionStatus/non-existent" 2>&1 | tail -10`

Expected: FAIL.

- [ ] **Step 3: Add translation in `UpdateTransactionStatus`**

In `internal/service/transaction_ops.go`, change lines 240-242 from:

```go
oldTx, err := ts.txRepo.GetTransactionByID(ctx, txID)
if err != nil {
    return fmt.Errorf("transaction not found: %w", err)
}
```

to:

```go
oldTx, err := ts.txRepo.GetTransactionByID(ctx, txID)
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        return fmt.Errorf("transaction #%d: %w", txID, ErrNotFound)
    }
    return fmt.Errorf("failed to get transaction: %w", err)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -v -run TestUpdateTransactionStatus 2>&1 | tail -20`

Expected: All subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "fix: translate repository.ErrNotFound in UpdateTransactionStatus (#106)"
```

---

### Task 5: Translate ErrNotFound in `UpdateTransactionComplete`

**Files:**
- Modify: `internal/service/transaction_ops.go:263-265`
- Modify: `internal/service/transaction_ops_test.go` (add subtest)

- [ ] **Step 1: Write the failing test**

Add a subtest to `TestUpdateTransactionComplete` in `internal/service/transaction_ops_test.go`:

```go
t.Run("non-existent transaction returns ErrNotFound", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    txRepo := newMockTransactionRepo()
    svc := newTestTransactionService(accRepo, txRepo)

    err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{
        ID:     999,
        Status: model.StatusPending,
        Splits: []model.SplitDetail{{AccountID: 1, Amount: 100}, {AccountID: 2, Amount: -100}},
    })
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound, got: %v", err)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -v -run "TestUpdateTransactionComplete/non-existent" 2>&1 | tail -10`

Expected: FAIL.

- [ ] **Step 3: Add translation in `UpdateTransactionComplete`**

In `internal/service/transaction_ops.go`, change lines 263-265 from:

```go
oldTx, err := ts.txRepo.GetTransactionByID(ctx, input.ID)
if err != nil {
    return fmt.Errorf("transaction not found: %w", err)
}
```

to:

```go
oldTx, err := ts.txRepo.GetTransactionByID(ctx, input.ID)
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        return fmt.Errorf("transaction #%d: %w", input.ID, ErrNotFound)
    }
    return fmt.Errorf("failed to get transaction: %w", err)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -v -run TestUpdateTransactionComplete 2>&1 | tail -20`

Expected: All subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "fix: translate repository.ErrNotFound in UpdateTransactionComplete (#106)"
```

---

### Task 6: Translate ErrNotFound in `DeleteAccountByName`

**Files:**
- Modify: `internal/service/account_ops.go:173-176`
- Modify: `internal/service/account_ops_test.go` (add subtest)

- [ ] **Step 1: Write the failing test**

Add a subtest to `TestDeleteAccountByName` in `internal/service/account_ops_test.go`:

```go
t.Run("non-existent account returns ErrNotFound", func(t *testing.T) {
    accRepo := newMockAccountRepo()
    svc := newTestAccountService(accRepo, newMockTransactionRepo())

    err := svc.DeleteAccountByName(context.Background(), "Assets:DoesNotExist")
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound, got: %v", err)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -v -run "TestDeleteAccountByName/non-existent" 2>&1 | tail -10`

Expected: FAIL — error wraps `repository.ErrNotFound`, not `service.ErrNotFound`.

- [ ] **Step 3: Add translation in `DeleteAccountByName`**

In `internal/service/account_ops.go`, change lines 173-176 from:

```go
func (as *AccountService) DeleteAccountByName(ctx context.Context, name string) error {
	acc, err := as.repo.GetAccountByName(ctx, name)
	if err != nil {
		return err
	}
```

to:

```go
func (as *AccountService) DeleteAccountByName(ctx context.Context, name string) error {
	acc, err := as.repo.GetAccountByName(ctx, name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("account %q: %w", name, ErrNotFound)
		}
		return err
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -v -run TestDeleteAccountByName 2>&1 | tail -20`

Expected: All subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_ops_test.go
git commit -m "fix: translate repository.ErrNotFound in DeleteAccountByName (#106)"
```

---

### Task 7: Full test suite verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... 2>&1 | tail -20`

Expected: All packages PASS with no regressions.

- [ ] **Step 2: Final commit (if any cleanup needed)**

No additional changes expected. If the full suite passes, the fix is complete.
