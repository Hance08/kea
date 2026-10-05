# Fix Duplicate external_id Error Sentinel Wrapping

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wrap the duplicate `external_id` error in `CreateTransactionWithSplits` with `repository.ErrAlreadyExists` so upstream layers can detect it via `errors.Is`.

**Architecture:** Three-layer fix following the existing `CreateAccount` pattern: store sentinel → store usage → service translation. The store defines `ErrTransactionExists` wrapping `repository.ErrAlreadyExists`, the SQLite layer returns it, and the service layer translates `repository.ErrAlreadyExists` into `service.ErrAlreadyExists`.

**Tech Stack:** Go, SQLite, existing mock-based test infrastructure

---

### Task 1: Add `ErrTransactionExists` sentinel to store errors

**Files:**
- Modify: `internal/store/errors.go:12-17`

- [ ] **Step 1: Add the sentinel**

In `internal/store/errors.go`, add `ErrTransactionExists` after `ErrAccountExists`:

```go
var (
	ErrAccountExists       = fmt.Errorf("account already exists: %w", repository.ErrAlreadyExists)
	ErrTransactionExists   = fmt.Errorf("transaction already exists: %w", repository.ErrAlreadyExists)
	ErrRecordNotFound      = fmt.Errorf("record not found: %w", repository.ErrNotFound)
	ErrConstraintViolation = fmt.Errorf("database constraint violation")
	ErrInvalidAccountType  = fmt.Errorf("invalid account type")
)
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/store/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/store/errors.go
git commit -m "fix: add ErrTransactionExists sentinel wrapping repository.ErrAlreadyExists (#126)"
```

---

### Task 2: Wrap the duplicate external_id error in the store layer

**Files:**
- Modify: `internal/store/sqlite_transaction.go:42`

- [ ] **Step 1: Replace the plain string error with the sentinel**

In `internal/store/sqlite_transaction.go`, change line 42 from:

```go
return 0, fmt.Errorf("transaction already exists (duplicate external_id)")
```

to:

```go
return 0, fmt.Errorf("duplicate external_id: %w", ErrTransactionExists)
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/store/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/store/sqlite_transaction.go
git commit -m "fix: wrap duplicate external_id error with ErrTransactionExists (#126)"
```

---

### Task 3: Add service-layer test for duplicate external_id

**Files:**
- Modify: `internal/service/transaction_ops_test.go` (add test case inside `TestCreateTransaction`)

- [ ] **Step 1: Write the failing test**

Add this test case inside `TestCreateTransaction` in `internal/service/transaction_ops_test.go`, after the existing test cases (before the closing `}` of the function):

```go
	t.Run("duplicate external_id returns ErrAlreadyExists", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		txRepo.createErr = fmt.Errorf("duplicate external_id: %w", repository.ErrAlreadyExists)

		input := model.TransactionDetail{
			Description: "Lunch",
			Type:        model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrAlreadyExists), "expected ErrAlreadyExists, got: %v", err)
	})
```

Ensure `"fmt"` and `"github.com/hance08/kea/internal/repository"` are in the import block of the test file.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/service/ -run TestCreateTransaction/duplicate_external_id -v`
Expected: FAIL — `errors.Is(err, ErrAlreadyExists)` returns false because the service layer does not yet translate the error.

- [ ] **Step 3: Commit the failing test**

```bash
git add internal/service/transaction_ops_test.go
git commit -m "test: add failing test for duplicate external_id sentinel (#126)"
```

---

### Task 4: Translate `repository.ErrAlreadyExists` in the service layer

**Files:**
- Modify: `internal/service/transaction_ops.go:97-105`

- [ ] **Step 1: Add the error translation**

In `internal/service/transaction_ops.go`, change the `ExecTx` callback (lines 97–105) from:

```go
	err := ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
		var err error

		newTxID, err = repo.CreateTransactionWithSplits(ctx, tx, splits)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}
		return nil
	})
```

to:

```go
	err := ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
		var err error

		newTxID, err = repo.CreateTransactionWithSplits(ctx, tx, splits)
		if err != nil {
			if errors.Is(err, repository.ErrAlreadyExists) {
				return fmt.Errorf("transaction already exists: %w", ErrAlreadyExists)
			}
			return fmt.Errorf("failed to create transaction: %w", err)
		}
		return nil
	})
```

Ensure `"errors"` and `"github.com/hance08/kea/internal/repository"` are in the import block. The `errors` import is likely already present; check and add only if missing.

- [ ] **Step 2: Run the test to verify it passes**

Run: `go test ./internal/service/ -run TestCreateTransaction/duplicate_external_id -v`
Expected: PASS

- [ ] **Step 3: Run the full test suite**

Run: `go test ./...`
Expected: all tests pass, no regressions

- [ ] **Step 4: Commit**

```bash
git add internal/service/transaction_ops.go
git commit -m "fix: translate duplicate external_id to service.ErrAlreadyExists (#126)"
```
