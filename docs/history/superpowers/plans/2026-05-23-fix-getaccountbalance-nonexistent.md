# Fix GetAccountBalance for Nonexistent Accounts — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `GetAccountBalance` and `GetAccountBalanceFormatted` should return `service.ErrNotFound` when called with a nonexistent account ID, instead of silently returning balance 0.

**Architecture:** Add an account existence check via `repo.GetAccountByID` at the top of both service methods, following the same pattern used by `GetAccountByID` (account_service.go:74-82). The store layer is unchanged — the fix is purely in the service layer.

**Tech Stack:** Go, testify (assert/require)

---

## File Map

- **Modify:** `internal/service/account_service.go:89-98` — add existence check to `GetAccountBalance` and `GetAccountBalanceFormatted`
- **Modify:** `internal/service/account_service_test.go` — add `TestGetAccountBalance` and `TestGetAccountBalanceFormatted` test functions

---

### Task 1: Add tests for GetAccountBalance

**Files:**
- Modify: `internal/service/account_service_test.go`

- [ ] **Step 1: Write failing tests for GetAccountBalance**

Add this test function after the existing `TestGetAccountByID` function:

```go
func TestGetAccountBalance(t *testing.T) {
	t.Run("returns balance for existing account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.balances[1] = 5000
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalance(context.Background(), 1)

		require.NoError(t, err)
		assert.Equal(t, int64(5000), bal)
	})

	t.Run("returns ErrNotFound for nonexistent account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalance(context.Background(), 999)

		assert.Equal(t, int64(0), bal)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.Contains(t, err.Error(), "999")
	})

	t.Run("passes through other errors from GetAccountByID", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		dbErr := fmt.Errorf("connection refused")
		accRepo.getByIDErr[42] = dbErr
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalance(context.Background(), 42)

		assert.Equal(t, int64(0), bal)
		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrNotFound))
		assert.Equal(t, dbErr, err)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestGetAccountBalance -v`
Expected: the "returns ErrNotFound for nonexistent account" subtest PASSes (since `GetAccountBalance` currently returns 0 with no error for a nonexistent account, the `require.Error` assertion will FAIL — wait, actually the mock `GetAccountBalance` returns `m.balances[accountID]` which is `0` with `nil` error for a missing key). The second subtest should FAIL because the current implementation returns `nil` error, not `ErrNotFound`.

---

### Task 2: Implement GetAccountBalance existence check

**Files:**
- Modify: `internal/service/account_service.go:89-91`

- [ ] **Step 3: Add existence check to GetAccountBalance**

Replace the current `GetAccountBalance` method:

```go
func (as *AccountService) GetAccountBalance(ctx context.Context, accountID int64) (int64, error) {
	if _, err := as.repo.GetAccountByID(ctx, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, fmt.Errorf("account #%d: %w", accountID, ErrNotFound)
		}
		return 0, err
	}
	return as.repo.GetAccountBalance(ctx, accountID)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestGetAccountBalance -v`
Expected: all 3 subtests PASS.

---

### Task 3: Add tests for GetAccountBalanceFormatted

**Files:**
- Modify: `internal/service/account_service_test.go`

- [ ] **Step 5: Write failing tests for GetAccountBalanceFormatted**

Add this test function after `TestGetAccountBalance`:

```go
func TestGetAccountBalanceFormatted(t *testing.T) {
	t.Run("returns formatted balance for existing account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.balances[1] = 5000
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalanceFormatted(context.Background(), 1)

		require.NoError(t, err)
		assert.Equal(t, "50", bal)
	})

	t.Run("returns ErrNotFound for nonexistent account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalanceFormatted(context.Background(), 999)

		assert.Empty(t, bal)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.Contains(t, err.Error(), "999")
	})

	t.Run("passes through other errors from GetAccountByID", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		dbErr := fmt.Errorf("connection refused")
		accRepo.getByIDErr[42] = dbErr
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalanceFormatted(context.Background(), 42)

		assert.Empty(t, bal)
		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrNotFound))
		assert.Equal(t, dbErr, err)
	})
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestGetAccountBalanceFormatted -v`
Expected: "returns ErrNotFound for nonexistent account" FAILS (nil error returned instead of ErrNotFound).

---

### Task 4: Implement GetAccountBalanceFormatted existence check

**Files:**
- Modify: `internal/service/account_service.go:93-99`

- [ ] **Step 7: Add existence check to GetAccountBalanceFormatted**

Replace the current `GetAccountBalanceFormatted` method:

```go
func (as *AccountService) GetAccountBalanceFormatted(ctx context.Context, accountID int64) (string, error) {
	if _, err := as.repo.GetAccountByID(ctx, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", fmt.Errorf("account #%d: %w", accountID, ErrNotFound)
		}
		return "", err
	}
	balance, err := as.repo.GetAccountBalance(ctx, accountID)
	if err != nil {
		return "", err
	}
	return utils.FormatAmount(balance), nil
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestGetAccountBalanceFormatted -v`
Expected: all 3 subtests PASS.

---

### Task 5: Full test suite verification and commit

- [ ] **Step 9: Run the full test suite**

Run: `go test ./...`
Expected: all tests PASS, no regressions.

- [ ] **Step 10: Commit**

```bash
git add internal/service/account_service.go internal/service/account_service_test.go
git commit -m "fix: return ErrNotFound from GetAccountBalance for nonexistent accounts (#122)"
```
