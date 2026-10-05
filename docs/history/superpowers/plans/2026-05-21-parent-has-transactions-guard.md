# Parent-Has-Transactions Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject account creation when the specified parent already has transactions, preventing a leaf account from becoming a parent while it still holds splits (issue #150).

**Architecture:** Add a guard in `validateAccountFields` that calls `AccountHasTransactions` on the parent when `parentID != nil`. Filter posted-leaf accounts from the interactive parent selector in the UI. Both `CreateAccount` and `CreateAccountWithBalance` are covered because they both call `validateAccountFields`.

**Tech Stack:** Go, testify, existing mock infrastructure in `internal/service/testhelper_test.go`

---

### Task 1: Failing tests for parent-has-transactions guard

**Files:**
- Modify: `internal/service/account_ops_test.go` (append new test group after `TestCreateAccount_ParentNameConsistency` at line ~1097)

- [ ] **Step 1: Write the failing tests**

Add these tests at the end of `internal/service/account_ops_test.go`:

```go
// ──────────────────────────────────────────────
// Parent with transactions guard (#150)
// ──────────────────────────────────────────────

func TestCreateAccount_ParentHasTransactions(t *testing.T) {
	t.Run("parent with transactions rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		accRepo.txExistsMap[10] = true
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Checking",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "parent", ve.Field)
		assert.Contains(t, err.Error(), "transactions")
	})

	t.Run("parent without transactions accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		// txExistsMap[10] defaults to false
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

	t.Run("nil parent skips transaction check", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Equal(t, "Assets:Bank", acc.Name)
	})

	t.Run("rejected via CreateAccountWithBalance too", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		accRepo.txExistsMap[10] = true
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Checking",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
			Balance:  1000,
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "parent", ve.Field)
		assert.Contains(t, err.Error(), "transactions")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestCreateAccount_ParentHasTransactions -v`
Expected: FAIL — the "parent with transactions rejected" and "rejected via CreateAccountWithBalance too" subtests fail because the guard does not exist yet. The "parent without transactions accepted" and "nil parent skips transaction check" subtests pass.

---

### Task 2: Implement the parent-has-transactions guard

**Files:**
- Modify: `internal/service/account_ops.go:102-133` (inside `validateAccountFields`)

- [ ] **Step 3: Add the guard inside validateAccountFields**

In `internal/service/account_ops.go`, inside the `if parentID != nil` block of `validateAccountFields` (line 112-128), add the transaction check after the parent chain validation and before the name prefix check. Insert after line 114 (`}`):

```go
		hasTransactions, err := as.repo.AccountHasTransactions(ctx, *parentID)
		if err != nil {
			return fmt.Errorf("failed to check parent transactions for account %d: %w", *parentID, err)
		}
		if hasTransactions {
			return validationErrorf("parent", "account %q already has transactions; it cannot become a parent", parent.Name)
		}
```

Note: The variable `parent` is fetched later (line 116). We need to restructure slightly — move the `GetAccountByID` call and reuse its result. The full replacement for the `if parentID != nil` block (lines 112-128) should be:

```go
	if parentID != nil {
		if err := as.validateParentChain(ctx, 0, parentID); err != nil {
			return err
		}
		parent, err := as.repo.GetAccountByID(ctx, *parentID)
		if err != nil {
			return fmt.Errorf("failed to look up parent account %d: %w", *parentID, err)
		}
		hasTransactions, err := as.repo.AccountHasTransactions(ctx, parent.ID)
		if err != nil {
			return fmt.Errorf("failed to check parent transactions for account %d: %w", parent.ID, err)
		}
		if hasTransactions {
			return validationErrorf("parent", "account %q already has transactions; it cannot become a parent", parent.Name)
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestCreateAccount_ParentHasTransactions -v`
Expected: All 4 subtests PASS.

- [ ] **Step 5: Run the full test suite to check for regressions**

Run: `go test ./...`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_ops_test.go
git commit -m "fix: reject child account creation when parent has transactions (#150)"
```

---

### Task 3: Filter posted-leaf accounts from parent selector UI

**Files:**
- Modify: `cmd/account/create_actions.go:83-99` (`promptParent` method)

- [ ] **Step 7: Write the failing test**

This is a UI-layer filter in `cmd/account/create_actions.go`. The `promptParent` method calls `GetAllAccounts` and passes the full list to the interactive prompt. While a full interactive test isn't practical, the service guard from Task 2 already prevents the invalid state. The UI filter is a UX convenience — it prevents the user from selecting an account that would be rejected anyway.

Since `promptParent` is in the `cmd` package and tightly coupled to the interactive prompt (`huh.Select`), we implement the filter directly and rely on the service-layer tests for correctness.

- [ ] **Step 8: Add AccountHasTransactions method to AccountService**

In `internal/service/account_service.go`, add this method (after `CheckAccountExists` at line ~111):

```go
func (as *AccountService) AccountHasTransactions(ctx context.Context, accountID int64) (bool, error) {
	return as.repo.AccountHasTransactions(ctx, accountID)
}
```

- [ ] **Step 9: Filter accounts with transactions in promptParent**

In `cmd/account/create_actions.go`, modify `promptParent` to filter out accounts that have transactions. Replace the method (lines 83-99) with:

```go
func (r *createRunner) promptParent(ctx context.Context) (*model.Account, error) {
	allAccounts, err := r.accSvc.GetAllAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve accounts: %w", err)
	}

	var eligible []*model.Account
	for _, acc := range allAccounts {
		hasTx, err := r.accSvc.AccountHasTransactions(ctx, acc.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to check transactions for %q: %w", acc.Name, err)
		}
		if !hasTx {
			eligible = append(eligible, acc)
		}
	}

	if len(eligible) == 0 {
		return nil, fmt.Errorf("no eligible parent accounts found (all accounts have transactions)")
	}

	_, selectedAccount, err := prompts.PromptParentAccount(eligible)
	if err != nil {
		return nil, err
	}

	if selectedAccount == nil {
		return nil, fmt.Errorf("no account selected")
	}

	return selectedAccount, nil
}
```

- [ ] **Step 10: Build to verify compilation**

Run: `make build`
Expected: Build succeeds without errors.

- [ ] **Step 11: Run the full test suite**

Run: `go test ./...`
Expected: All tests PASS.

- [ ] **Step 12: Commit**

```bash
git add internal/service/account_service.go cmd/account/create_actions.go
git commit -m "feat: filter accounts with transactions from parent selector (#150)"
```
