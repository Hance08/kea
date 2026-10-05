# Parent–Name Consistency Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject account creation when `ParentID` points to a parent whose name is not the path prefix of the new account name, closing the divergence described in issue #155.

**Architecture:** Add a `validateParentNameConsistency` check inside `validateAccountFields` that loads the parent account and verifies `name == parent.Name + ":" + singleSegment`. Both `CreateAccount` and `CreateAccountWithBalance` already call `validateAccountFields`, so both paths are covered. A new sentinel error `ErrParentNameMismatch` is not needed — the existing `ValidationError` with field `"parent"` is sufficient.

**Tech Stack:** Go, existing service-layer mock test infrastructure

---

### Task 1: Add failing tests for parent–name mismatch

**Files:**
- Modify: `internal/service/account_ops_test.go` (append new test function after the existing `TestCreateAccount_ParentTypeMismatch` block, around line 1002)

- [ ] **Step 1: Write the failing tests**

Add a new test function `TestCreateAccount_ParentNameConsistency` at the end of `account_ops_test.go`:

```go
func TestCreateAccount_ParentNameConsistency(t *testing.T) {
	t.Run("name under different branch than parent rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Expenses:Food",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "parent", ve.Field)
		assert.Contains(t, err.Error(), "Assets:Bank")
	})

	t.Run("name shares prefix but adds extra nesting rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Sub:Deep",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "parent", ve.Field)
	})

	t.Run("correct child name accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Checking",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.NoError(t, err)
		assert.Equal(t, "Assets:Bank:Checking", acc.Name)
	})

	t.Run("mismatch rejected via CreateAccountWithBalance", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Cash",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
			Balance:  1000,
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "parent", ve.Field)
	})

	t.Run("nil parent skips name consistency check", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Equal(t, "Assets:Bank", acc.Name)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestCreateAccount_ParentNameConsistency -v`

Expected: The two mismatch sub-tests (`name under different branch than parent rejected` and `name shares prefix but adds extra nesting rejected`) and `mismatch rejected via CreateAccountWithBalance` PASS unexpectedly or FAIL because no `ValidationError` with field `"parent"` is returned. The two happy-path sub-tests pass.

- [ ] **Step 3: Commit failing tests**

```bash
git add internal/service/account_ops_test.go
git commit -m "test: add failing tests for parent-name consistency (#155)"
```

---

### Task 2: Implement parent–name consistency validation

**Files:**
- Modify: `internal/service/account_ops.go:102-122` (the `validateAccountFields` function)

- [ ] **Step 1: Add the validation logic**

In `validateAccountFields`, after the existing `validateParentChain` block (lines 116-120), add a parent-name consistency check. The final `validateAccountFields` function should look like this:

```go
func (as *AccountService) validateAccountFields(ctx context.Context, name string, accType model.AccountType, currency string, parentID *int64) error {
	if err := as.ValidateFullAccountName(name); err != nil {
		return validationWrap("name", "invalid account name", err)
	}
	if err := as.ValidateCurrency(currency); err != nil {
		return validationWrap("currency", "invalid currency", err)
	}
	if !accType.IsValid() {
		return validationErrorf("type", "invalid account type: %s", accType)
	}
	root := strings.SplitN(name, ":", 2)[0]
	if expected, ok := model.AccountTypeFromRootName(root); ok && expected != accType {
		return validationErrorf("type", "account type %q conflicts with root %q (expected %q)", accType, root, expected)
	}
	if parentID != nil {
		if err := as.validateParentChain(ctx, 0, parentID); err != nil {
			return err
		}
		parent, err := as.repo.GetAccountByID(ctx, *parentID)
		if err != nil {
			return fmt.Errorf("failed to look up parent account %d: %w", *parentID, err)
		}
		expectedPrefix := parent.Name + ":"
		if !strings.HasPrefix(name, expectedPrefix) {
			return validationErrorf("parent", "account name %q is not a child of parent %q", name, parent.Name)
		}
		childSegment := strings.TrimPrefix(name, expectedPrefix)
		if strings.Contains(childSegment, ":") {
			return validationErrorf("parent", "account name %q is not a direct child of parent %q (nested segments found)", name, parent.Name)
		}
	}
	return nil
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestCreateAccount_ParentNameConsistency -v`

Expected: All 5 sub-tests PASS.

- [ ] **Step 3: Run the full test suite to check for regressions**

Run: `go test ./...`

Expected: All tests pass. The existing `TestCreateAccount_ParentValidation` tests that use matching names (e.g., `"Assets:Bank:Checking"` with parent `"Assets:Bank"`) should continue to pass.

- [ ] **Step 4: Commit implementation**

```bash
git add internal/service/account_ops.go
git commit -m "fix: validate account name matches parent path on creation (#155)"
```
