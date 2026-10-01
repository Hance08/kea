# Atomic CreateAccountWithBalance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wrap `CreateAccountWithBalance` in a single `ExecTx` so account creation and opening balance commit or roll back together (issue #29).

**Architecture:** Extract the repo-level write logic from `CreateAccount` and `createOpeningBalance` into internal helpers that accept a `repository.Repository` parameter. `CreateAccountWithBalance` calls both helpers inside one `ExecTx`. The public `CreateAccount` method stays unchanged (it delegates to the helper with `as.repo`). Remove the partial-success return path.

**Tech Stack:** Go, existing mock-based service tests

---

### Task 1: Add atomicity regression test

This test proves the bug exists today: when opening balance creation fails, the account should not be committed. After the fix, this test will pass.

**Files:**
- Modify: `internal/service/account_ops_test.go`

- [ ] **Step 1: Write the failing test**

Add this test at the end of `TestCreateAccountWithBalance_SplitDirection`:

```go
t.Run("opening balance failure rolls back account creation", func(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	addOpeningBalanceAccount(accRepo)
	txRepo.createErr = errors.New("forced DB error")
	svc := newTestAccountService(accRepo, txRepo)

	acc, err := svc.CreateAccountWithBalance(context.Background(), "Assets:Bank", model.AccountTypeAsset, "USD", "", nil, 5000)
	require.Error(t, err)
	assert.Nil(t, acc, "account should be nil on rollback")

	_, lookupErr := accRepo.GetAccountByName(context.Background(), "Assets:Bank")
	assert.ErrorIs(t, lookupErr, repository.ErrNotFound, "account should not exist after rollback")
})
```

Import `"github.com/hance08/kea/internal/repository"` at the top of the test file if not already present.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestCreateAccountWithBalance_SplitDirection/opening_balance_failure -v`

Expected: FAIL — the current code returns a non-nil account and leaves it in the repo.

- [ ] **Step 3: Commit**

```bash
git add internal/service/account_ops_test.go
git commit -m "test: add atomicity regression test for CreateAccountWithBalance (issue #29)"
```

---

### Task 2: Extract repo-scoped helpers and wrap in ExecTx

Refactor `CreateAccount` and `createOpeningBalance` so the write logic accepts a `repository.Repository` (or the narrower `repository.AccountRepository`), then call both from a single `ExecTx` in `CreateAccountWithBalance`.

**Files:**
- Modify: `internal/service/account_ops.go:49-155`

- [ ] **Step 1: Extract `createAccountInRepo` helper**

Add an unexported helper that contains the current write logic from `CreateAccount`, but takes a `repo` parameter instead of using `as.repo`. The validation logic stays on `AccountService` since it doesn't write.

Replace the current `CreateAccount` and `CreateAccountWithBalance` methods and add the helper. The full replacement for lines 49–155 of `account_ops.go`:

```go
func (as *AccountService) CreateAccount(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64) (*model.Account, error) {
	if err := as.validateAccountFields(ctx, name, accType, currency, parentID); err != nil {
		return nil, err
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))

	newID, err := as.repo.CreateAccount(ctx, name, accType, currency, description, parentID)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, fmt.Errorf("account %q: %w", name, ErrAlreadyExists)
		}
		return nil, err
	}

	return &model.Account{
		ID:          newID,
		Name:        name,
		Type:        accType,
		Currency:    currency,
		Description: description,
		ParentID:    parentID,
		IsHidden:    false,
	}, nil
}

func (as *AccountService) CreateAccountWithBalance(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64, balance int64) (*model.Account, error) {
	if err := as.validateAccountFields(ctx, name, accType, currency, parentID); err != nil {
		return nil, err
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))

	if balance == 0 {
		return as.createAccountViaRepo(ctx, as.repo, name, accType, currency, description, parentID)
	}

	var account *model.Account
	err := as.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, createErr := as.createAccountViaRepo(ctx, repo, name, accType, currency, description, parentID)
		if createErr != nil {
			return createErr
		}
		account = acc
		return as.createOpeningBalanceInRepo(ctx, repo, account, balance)
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

// validateAccountFields validates name, currency, type, and parent chain
// without performing any writes.
func (as *AccountService) validateAccountFields(ctx context.Context, name string, accType model.AccountType, currency string, parentID *int64) error {
	if err := as.ValidateFullAccountName(name); err != nil {
		return fmt.Errorf("invalid account name: %w", err)
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if err := as.ValidateCurrency(currency); err != nil {
		return fmt.Errorf("invalid currency: %w", err)
	}
	if !accType.IsValid() {
		return fmt.Errorf("invalid account type: %s", accType)
	}
	if parentID != nil {
		if err := as.validateParentChain(ctx, 0, parentID); err != nil {
			return err
		}
	}
	return nil
}

// createAccountViaRepo performs the account INSERT using the provided repo.
// This allows callers to pass either as.repo (non-transactional) or a
// transaction-scoped repo from ExecTx.
func (as *AccountService) createAccountViaRepo(_ context.Context, repo repository.AccountRepository, name string, accType model.AccountType, currency, description string, parentID *int64) (*model.Account, error) {
	ctx := context.TODO()
	newID, err := repo.CreateAccount(ctx, name, accType, currency, description, parentID)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, fmt.Errorf("account %q: %w", name, ErrAlreadyExists)
		}
		return nil, err
	}

	return &model.Account{
		ID:          newID,
		Name:        name,
		Type:        accType,
		Currency:    currency,
		Description: description,
		ParentID:    parentID,
		IsHidden:    false,
	}, nil
}

func (as *AccountService) createOpeningBalance(ctx context.Context, account *model.Account, amountInCents int64) error {
	return as.tm.ExecTx(ctx, func(repo repository.Repository) error {
		return as.createOpeningBalanceInRepo(ctx, repo, account, amountInCents)
	})
}

// createOpeningBalanceInRepo contains the opening-balance logic but
// delegates all writes to the provided repo, so callers can wrap it in
// a broader transaction.
func (as *AccountService) createOpeningBalanceInRepo(ctx context.Context, repo repository.Repository, account *model.Account, amountInCents int64) error {
	currency := account.Currency
	if currency == "" {
		currency = as.config.Defaults.Currency
	}

	equityAccountName := model.OpeningBalancesAccountName(currency)

	var balanceAmount, equityAmount int64
	switch account.Type {
	case model.AccountTypeAsset:
		balanceAmount = amountInCents
		equityAmount = -amountInCents
	case model.AccountTypeLiability:
		balanceAmount = -amountInCents
		equityAmount = amountInCents
	default:
		return fmt.Errorf("only Assets(A) and Liabilities(L) accounts can set a balance")
	}

	tx := model.Transaction{
		Timestamp:   time.Now().Unix(),
		Description: model.OpeningAccountMemo,
		Status:      model.StatusCleared,
		Type:        model.TxTypeOpening,
	}

	equityAcc, err := repo.GetAccountByName(ctx, equityAccountName)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("failed to look up %q: %w", equityAccountName, err)
		}
		newID, createErr := repo.CreateAccount(
			ctx,
			equityAccountName,
			model.AccountTypeEquity,
			currency,
			"Opening Balances (System Account)",
			nil,
		)
		if createErr != nil {
			return fmt.Errorf("failed to create %q: %w", equityAccountName, createErr)
		}
		equityAcc = &model.Account{ID: newID}
	}

	splits := []model.Split{
		{AccountID: account.ID, Amount: balanceAmount, Currency: currency, Memo: model.OpeningAccountMemo},
		{AccountID: equityAcc.ID, Amount: equityAmount, Currency: currency, Memo: model.OpeningAccountMemo},
	}
	_, err = repo.CreateTransactionWithSplits(ctx, tx, splits)
	return err
}
```

Wait — let me reconsider. The `createAccountViaRepo` helper takes a context but ignores it in favor of `context.TODO()`. That's wrong. Let me fix the approach.

Actually, looking more carefully, the simpler approach: `createAccountViaRepo` should just forward the `ctx` parameter. Let me revise.

Also, I notice `createOpeningBalance` (the standalone version) is not called anywhere except in the old `CreateAccountWithBalance`. After the refactor it's dead code. But keeping it doesn't hurt for now — remove it to keep things clean.

Here's the corrected implementation:

```go
func (as *AccountService) CreateAccount(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64) (*model.Account, error) {
	if err := as.validateAccountFields(ctx, name, accType, currency, parentID); err != nil {
		return nil, err
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	return as.createAccountViaRepo(ctx, as.repo, name, accType, currency, description, parentID)
}

func (as *AccountService) CreateAccountWithBalance(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64, balance int64) (*model.Account, error) {
	if err := as.validateAccountFields(ctx, name, accType, currency, parentID); err != nil {
		return nil, err
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))

	if balance == 0 {
		return as.createAccountViaRepo(ctx, as.repo, name, accType, currency, description, parentID)
	}

	var account *model.Account
	err := as.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, createErr := as.createAccountViaRepo(ctx, repo, name, accType, currency, description, parentID)
		if createErr != nil {
			return createErr
		}
		account = acc
		return as.createOpeningBalanceInRepo(ctx, repo, account, balance)
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (as *AccountService) validateAccountFields(ctx context.Context, name string, accType model.AccountType, currency string, parentID *int64) error {
	if err := as.ValidateFullAccountName(name); err != nil {
		return fmt.Errorf("invalid account name: %w", err)
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if err := as.ValidateCurrency(currency); err != nil {
		return fmt.Errorf("invalid currency: %w", err)
	}
	if !accType.IsValid() {
		return fmt.Errorf("invalid account type: %s", accType)
	}
	if parentID != nil {
		if err := as.validateParentChain(ctx, 0, parentID); err != nil {
			return err
		}
	}
	return nil
}

func (as *AccountService) createAccountViaRepo(ctx context.Context, repo repository.AccountRepository, name string, accType model.AccountType, currency, description string, parentID *int64) (*model.Account, error) {
	newID, err := repo.CreateAccount(ctx, name, accType, currency, description, parentID)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, fmt.Errorf("account %q: %w", name, ErrAlreadyExists)
		}
		return nil, err
	}
	return &model.Account{
		ID:          newID,
		Name:        name,
		Type:        accType,
		Currency:    currency,
		Description: description,
		ParentID:    parentID,
		IsHidden:    false,
	}, nil
}

func (as *AccountService) createOpeningBalanceInRepo(ctx context.Context, repo repository.Repository, account *model.Account, amountInCents int64) error {
	currency := account.Currency
	if currency == "" {
		currency = as.config.Defaults.Currency
	}

	equityAccountName := model.OpeningBalancesAccountName(currency)

	var balanceAmount, equityAmount int64
	switch account.Type {
	case model.AccountTypeAsset:
		balanceAmount = amountInCents
		equityAmount = -amountInCents
	case model.AccountTypeLiability:
		balanceAmount = -amountInCents
		equityAmount = amountInCents
	default:
		return fmt.Errorf("only Assets(A) and Liabilities(L) accounts can set a balance")
	}

	tx := model.Transaction{
		Timestamp:   time.Now().Unix(),
		Description: model.OpeningAccountMemo,
		Status:      model.StatusCleared,
		Type:        model.TxTypeOpening,
	}

	equityAcc, err := repo.GetAccountByName(ctx, equityAccountName)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("failed to look up %q: %w", equityAccountName, err)
		}
		newID, createErr := repo.CreateAccount(
			ctx,
			equityAccountName,
			model.AccountTypeEquity,
			currency,
			"Opening Balances (System Account)",
			nil,
		)
		if createErr != nil {
			return fmt.Errorf("failed to create %q: %w", equityAccountName, createErr)
		}
		equityAcc = &model.Account{ID: newID}
	}

	splits := []model.Split{
		{AccountID: account.ID, Amount: balanceAmount, Currency: currency, Memo: model.OpeningAccountMemo},
		{AccountID: equityAcc.ID, Amount: equityAmount, Currency: currency, Memo: model.OpeningAccountMemo},
	}
	_, err = repo.CreateTransactionWithSplits(ctx, tx, splits)
	return err
}
```

- [ ] **Step 2: Run all tests to verify nothing is broken**

Run: `go test ./internal/service/ -v -count=1`

Expected: ALL PASS, including the new atomicity test from Task 1.

- [ ] **Step 3: Commit**

```bash
git add internal/service/account_ops.go
git commit -m "fix: make CreateAccountWithBalance atomic via single ExecTx (issue #29)"
```

---

### Task 3: Verify the mock simulates rollback correctly

The `mockTransactionManager.ExecTx` currently runs `fn` and if it returns an error, that error propagates — but the mock account repo still has the account in memory (it was inserted before the error). For the atomicity test to be meaningful, the mock must undo writes on error.

**Files:**
- Modify: `internal/service/testhelper_test.go:496-505`

- [ ] **Step 1: Update `mockTransactionManager.ExecTx` to snapshot and rollback**

Replace the current `ExecTx` implementation:

```go
func (m *mockTransactionManager) ExecTx(_ context.Context, fn func(repository.Repository) error) error {
	if m.failTx {
		return errors.New("transaction manager: forced failure")
	}

	// Snapshot account state before the transaction so we can roll back.
	accSnapshot := make(map[string]*model.Account, len(m.accRepo.accountsByName))
	for k, v := range m.accRepo.accountsByName {
		accSnapshot[k] = v
	}
	idSnapshot := make(map[int64]*model.Account, len(m.accRepo.accountsByID))
	for k, v := range m.accRepo.accountsByID {
		idSnapshot[k] = v
	}
	nextIDSnapshot := m.accRepo.nextID

	txSnapshot := make(map[int64]*model.Transaction, len(m.txRepo.transactions))
	for k, v := range m.txRepo.transactions {
		txSnapshot[k] = v
	}
	splitsSnapshot := make(map[int64][]*model.Split, len(m.txRepo.splits))
	for k, v := range m.txRepo.splits {
		splitsSnapshot[k] = v
	}
	nextTxIDSnapshot := m.txRepo.nextTxID
	nextSplitIDSnapshot := m.txRepo.nextSplitID

	combined := &mockCombinedRepo{
		mockAccountRepo:     m.accRepo,
		mockTransactionRepo: m.txRepo,
	}
	if err := fn(combined); err != nil {
		// Rollback: restore pre-transaction state.
		m.accRepo.accountsByName = accSnapshot
		m.accRepo.accountsByID = idSnapshot
		m.accRepo.nextID = nextIDSnapshot
		m.txRepo.transactions = txSnapshot
		m.txRepo.splits = splitsSnapshot
		m.txRepo.nextTxID = nextTxIDSnapshot
		m.txRepo.nextSplitID = nextSplitIDSnapshot
		return err
	}
	return nil
}
```

- [ ] **Step 2: Run all tests to verify rollback behavior**

Run: `go test ./internal/service/ -v -count=1`

Expected: ALL PASS. The atomicity test now properly verifies that the account does not exist after rollback.

- [ ] **Step 3: Commit**

```bash
git add internal/service/testhelper_test.go
git commit -m "test: add rollback simulation to mockTransactionManager for atomicity tests"
```

---

### Task 4: Run full test suite and verify

- [ ] **Step 1: Run full project tests**

Run: `go test ./... -count=1`

Expected: ALL PASS across all packages.

- [ ] **Step 2: Build the binary**

Run: `make build`

Expected: Clean build with no errors.
