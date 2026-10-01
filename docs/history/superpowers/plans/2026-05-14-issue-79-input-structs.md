# Issue #79: Service Input Structs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace long positional parameter lists in service methods with input structs defined in `internal/model/`, so both the CLI layer and a future Web API layer can construct them from their respective input sources.

**Architecture:** Define `CreateAccountInput`, `CreateSimpleTransactionInput`, `CreateTransactionFromSplitsInput`, and `UpdateTransactionInput` structs in `internal/model/input.go`. Refactor each service method to accept its struct. Update all callers (cmd/, service tests, cmd test mocks) and the cmd-layer provider interfaces. `CreateTransactionFromSplitsInput` is a subset of `TransactionDetail` but keeping a dedicated input struct avoids coupling API contracts to internal domain types.

**Tech Stack:** Go, no new dependencies

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/model/input.go` | **Create** | All service-layer input structs |
| `internal/service/account_ops.go` | Modify | `CreateAccount`, `CreateAccountWithBalance`, `createAccountViaRepo` accept `CreateAccountInput` |
| `internal/service/transaction_ops.go` | Modify | `CreateSimpleTransaction`, `CreateTransactionFromSplits`, `UpdateTransactionComplete` accept input structs |
| `internal/service/account_ops_test.go` | Modify | Update all `CreateAccount`/`CreateAccountWithBalance` call sites |
| `internal/service/transaction_ops_test.go` | Modify | Update all `CreateSimpleTransaction`/`UpdateTransactionComplete` call sites |
| `internal/service/errors_test.go` | Modify | Update validation-error test call sites |
| `cmd/add_types.go` | Modify | Update `TransactionProvider` interface signatures |
| `cmd/add.go` | Modify | Update call sites to use input structs |
| `cmd/add_test.go` | Modify | Update mock implementations |
| `cmd/account/create_types.go` | Modify | Update `CreateProvider` interface signature |
| `cmd/account/create_actions.go` | Modify | Update call site to use `CreateAccountInput` |
| `cmd/transaction/edit_types.go` | Modify | Update `EditProvider` interface signature |
| `cmd/transaction/edit_actions.go` | Modify | Update call site to use `UpdateTransactionInput` |
| `cmd/root.go` | Modify | Update `CreateAccount` call in `migrateLegacySysAcc` |

---

### Task 1: Define input structs in model package

**Files:**
- Create: `internal/model/input.go`

- [ ] **Step 1: Create the input structs file**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type CreateAccountInput struct {
	Name        string
	Type        AccountType
	Currency    string
	Description string
	ParentID    *int64
	Balance     int64
}

type CreateSimpleTransactionInput struct {
	FromAccount string
	ToAccount   string
	Amount      int64
	Description string
	Timestamp   int64
	Status      TransactionStatus
	Type        TransactionType
}

type CreateTransactionFromSplitsInput struct {
	Splits      []SplitDetail
	Description string
	Timestamp   int64
	Status      TransactionStatus
	Type        TransactionType
}

type UpdateTransactionInput struct {
	ID          int64
	Description string
	Timestamp   int64
	Status      TransactionStatus
	Type        TransactionType
	Splits      []SplitDetail
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/model/...`
Expected: success, no errors

- [ ] **Step 3: Commit**

```bash
git add internal/model/input.go
git commit -m "feat: add service-layer input structs (issue #79)"
```

---

### Task 2: Refactor AccountService to use CreateAccountInput

**Files:**
- Modify: `internal/service/account_ops.go`

- [ ] **Step 1: Run existing account tests to establish baseline**

Run: `go test ./internal/service/ -run TestCreateAccount -v`
Expected: all PASS

- [ ] **Step 2: Refactor CreateAccount, CreateAccountWithBalance, and createAccountViaRepo**

In `internal/service/account_ops.go`, replace the three method signatures:

**CreateAccount** (line 49) — change from:
```go
func (as *AccountService) CreateAccount(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64) (*model.Account, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if err := as.validateAccountFields(ctx, name, accType, currency, parentID); err != nil {
		return nil, err
	}
	return as.createAccountViaRepo(ctx, as.repo, name, accType, currency, description, parentID)
}
```
to:
```go
func (as *AccountService) CreateAccount(ctx context.Context, input model.CreateAccountInput) (*model.Account, error) {
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if err := as.validateAccountFields(ctx, input.Name, input.Type, input.Currency, input.ParentID); err != nil {
		return nil, err
	}
	return as.createAccountViaRepo(ctx, as.repo, input)
}
```

**CreateAccountWithBalance** (line 57) — change from:
```go
func (as *AccountService) CreateAccountWithBalance(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64, balance int64) (*model.Account, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if err := as.validateAccountFields(ctx, name, accType, currency, parentID); err != nil {
		return nil, err
	}

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
```
to:
```go
func (as *AccountService) CreateAccountWithBalance(ctx context.Context, input model.CreateAccountInput) (*model.Account, error) {
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if err := as.validateAccountFields(ctx, input.Name, input.Type, input.Currency, input.ParentID); err != nil {
		return nil, err
	}

	if input.Balance == 0 {
		return as.createAccountViaRepo(ctx, as.repo, input)
	}

	var account *model.Account
	err := as.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, createErr := as.createAccountViaRepo(ctx, repo, input)
		if createErr != nil {
			return createErr
		}
		account = acc
		return as.createOpeningBalanceInRepo(ctx, repo, account, input.Balance)
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}
```

**createAccountViaRepo** (line 100) — change from:
```go
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
```
to:
```go
func (as *AccountService) createAccountViaRepo(ctx context.Context, repo repository.AccountRepository, input model.CreateAccountInput) (*model.Account, error) {
	newID, err := repo.CreateAccount(ctx, input.Name, input.Type, input.Currency, input.Description, input.ParentID)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, fmt.Errorf("account %q: %w", input.Name, ErrAlreadyExists)
		}
		return nil, err
	}
	return &model.Account{
		ID:          newID,
		Name:        input.Name,
		Type:        input.Type,
		Currency:    input.Currency,
		Description: input.Description,
		ParentID:    input.ParentID,
		IsHidden:    false,
	}, nil
}
```

Note: `createAccountViaRepo` still passes individual fields to `repo.CreateAccount` because the repository interface (`AccountRepository`) is out of scope for this issue — it is the contract with the store layer and uses positional params by design.

- [ ] **Step 3: Update service-layer tests for CreateAccount and CreateAccountWithBalance**

In `internal/service/account_ops_test.go`, update every call to `svc.CreateAccount(...)` and `svc.CreateAccountWithBalance(...)` to use the struct form. Here is the pattern — apply to every occurrence:

Before:
```go
acc, err := svc.CreateAccount(context.Background(), "Assets:Bank", model.AccountTypeAsset, "USD", "My bank", nil)
```
After:
```go
acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
    Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", Description: "My bank",
})
```

Before:
```go
acc, err := svc.CreateAccountWithBalance(context.Background(), "Assets:Bank", model.AccountTypeAsset, "USD", "", nil, 5000)
```
After:
```go
acc, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{
    Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", Balance: 5000,
})
```

Also update calls in `internal/service/errors_test.go` that test `CreateAccount` and `CreateAccountWithBalance` (lines 171, 223).

- [ ] **Step 4: Run account tests to verify**

Run: `go test ./internal/service/ -run "TestCreateAccount|TestCreateAccountWithBalance" -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_ops_test.go internal/service/errors_test.go
git commit -m "refactor: AccountService methods accept CreateAccountInput (issue #79)"
```

---

### Task 3: Update account-related cmd callers

**Files:**
- Modify: `cmd/account/create_types.go`
- Modify: `cmd/account/create_actions.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Update CreateProvider interface in cmd/account/create_types.go**

Change line 24 from:
```go
CreateAccountWithBalance(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64, balance int64) (*model.Account, error)
```
to:
```go
CreateAccountWithBalance(ctx context.Context, input model.CreateAccountInput) (*model.Account, error)
```

- [ ] **Step 2: Update call site in cmd/account/create_actions.go**

Change the `createAccount` method (lines 15-24) from:
```go
func (r *createRunner) createAccount(ctx context.Context, input createInput) (*model.Account, error) {
	return r.accSvc.CreateAccountWithBalance(
		ctx,
		input.fullName,
		input.accountType,
		input.currency,
		input.description,
		input.parentID,
		input.balanceCents,
	)
}
```
to:
```go
func (r *createRunner) createAccount(ctx context.Context, input createInput) (*model.Account, error) {
	return r.accSvc.CreateAccountWithBalance(ctx, model.CreateAccountInput{
		Name:        input.fullName,
		Type:        input.accountType,
		Currency:    input.currency,
		Description: input.description,
		ParentID:    input.parentID,
		Balance:     input.balanceCents,
	})
}
```

- [ ] **Step 3: Update call site in cmd/root.go**

Change the `migrateLegacySysAcc` function (lines 160-167) from:
```go
_, err = svc.Account().CreateAccount(
    context.Background(),
    targetName,
    model.AccountTypeEquity,
    cfg.Defaults.Currency,
    "Opening Balances (System Account)",
    nil,
)
```
to:
```go
_, err = svc.Account().CreateAccount(context.Background(), model.CreateAccountInput{
    Name:        targetName,
    Type:        model.AccountTypeEquity,
    Currency:    cfg.Defaults.Currency,
    Description: "Opening Balances (System Account)",
})
```

- [ ] **Step 4: Run cmd tests to verify**

Run: `go test ./cmd/... -v`
Expected: all PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/account/create_types.go cmd/account/create_actions.go cmd/root.go
git commit -m "refactor: update account cmd callers for CreateAccountInput (issue #79)"
```

---

### Task 4: Refactor TransactionService — CreateSimpleTransaction and CreateTransactionFromSplits

**Files:**
- Modify: `internal/service/transaction_ops.go`

- [ ] **Step 1: Run existing transaction tests to establish baseline**

Run: `go test ./internal/service/ -run "TestCreateSimpleTransaction|TestCreateTransactionFromSplits" -v`
Expected: all PASS

- [ ] **Step 2: Refactor CreateSimpleTransaction**

In `internal/service/transaction_ops.go`, change the signature and body (line 134) from:
```go
func (ts *TransactionService) CreateSimpleTransaction(ctx context.Context, fromAccount, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error) {
	if fromAccount == toAccount {
		return model.TransactionDetail{}, validationErrorf("account", "source and destination accounts cannot be the same")
	}
	if amount <= 0 {
		return model.TransactionDetail{}, validationErrorf("amount", "amount must be positive")
	}

	// If no type provided, infer from account types.
	if txType == "" {
		fromAcc, err := ts.accRepo.GetAccountByName(ctx, fromAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve from account: %w", err)
		}
		toAcc, err := ts.accRepo.GetAccountByName(ctx, toAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve to account: %w", err)
		}
		inferred, err := ts.DetermineType(ctx, []model.SplitDetail{
			{AccountType: toAcc.Type, Amount: amount},
			{AccountType: fromAcc.Type, Amount: -amount},
		})
		if err != nil {
			return model.TransactionDetail{}, err
		}
		txType = inferred
	}

	splits := []model.SplitDetail{
		{AccountName: toAccount, Amount: amount},
		{AccountName: fromAccount, Amount: -amount},
	}
	input := model.TransactionDetail{
		Timestamp:   timestamp,
		Description: desc,
		Status:      status,
		Type:        txType,
		Splits:      splits,
	}
	id, err := ts.CreateTransaction(ctx, input)
	if err != nil {
		return model.TransactionDetail{}, err
	}
	input.ID = id
	return input, nil
}
```
to:
```go
func (ts *TransactionService) CreateSimpleTransaction(ctx context.Context, input model.CreateSimpleTransactionInput) (model.TransactionDetail, error) {
	if input.FromAccount == input.ToAccount {
		return model.TransactionDetail{}, validationErrorf("account", "source and destination accounts cannot be the same")
	}
	if input.Amount <= 0 {
		return model.TransactionDetail{}, validationErrorf("amount", "amount must be positive")
	}

	txType := input.Type
	if txType == "" {
		fromAcc, err := ts.accRepo.GetAccountByName(ctx, input.FromAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve from account: %w", err)
		}
		toAcc, err := ts.accRepo.GetAccountByName(ctx, input.ToAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve to account: %w", err)
		}
		inferred, err := ts.DetermineType(ctx, []model.SplitDetail{
			{AccountType: toAcc.Type, Amount: input.Amount},
			{AccountType: fromAcc.Type, Amount: -input.Amount},
		})
		if err != nil {
			return model.TransactionDetail{}, err
		}
		txType = inferred
	}

	splits := []model.SplitDetail{
		{AccountName: input.ToAccount, Amount: input.Amount},
		{AccountName: input.FromAccount, Amount: -input.Amount},
	}
	txDetail := model.TransactionDetail{
		Timestamp:   input.Timestamp,
		Description: input.Description,
		Status:      input.Status,
		Type:        txType,
		Splits:      splits,
	}
	id, err := ts.CreateTransaction(ctx, txDetail)
	if err != nil {
		return model.TransactionDetail{}, err
	}
	txDetail.ID = id
	return txDetail, nil
}
```

- [ ] **Step 3: Refactor CreateTransactionFromSplits**

Change (line 184) from:
```go
func (ts *TransactionService) CreateTransactionFromSplits(
	ctx context.Context,
	splits []model.SplitDetail,
	desc string,
	timestamp int64,
	status model.TransactionStatus,
	txType model.TransactionType,
) (model.TransactionDetail, error) {
	input := model.TransactionDetail{
		Timestamp:   timestamp,
		Description: desc,
		Status:      status,
		Type:        txType,
		Splits:      splits,
	}
	id, err := ts.CreateTransaction(ctx, input)
	if err != nil {
		return model.TransactionDetail{}, err
	}
	input.ID = id
	return input, nil
}
```
to:
```go
func (ts *TransactionService) CreateTransactionFromSplits(ctx context.Context, input model.CreateTransactionFromSplitsInput) (model.TransactionDetail, error) {
	txDetail := model.TransactionDetail{
		Timestamp:   input.Timestamp,
		Description: input.Description,
		Status:      input.Status,
		Type:        input.Type,
		Splits:      input.Splits,
	}
	id, err := ts.CreateTransaction(ctx, txDetail)
	if err != nil {
		return model.TransactionDetail{}, err
	}
	txDetail.ID = id
	return txDetail, nil
}
```

- [ ] **Step 4: Update service-layer tests**

In `internal/service/transaction_ops_test.go`, update every call. Pattern:

Before:
```go
detail, err := svc.CreateSimpleTransaction(
    ctx, "Assets:Cash", "Expenses:Food", 1500, "Lunch", ts, model.StatusCleared, model.TxTypeExpense,
)
```
After:
```go
detail, err := svc.CreateSimpleTransaction(ctx, model.CreateSimpleTransactionInput{
    FromAccount: "Assets:Cash", ToAccount: "Expenses:Food", Amount: 1500,
    Description: "Lunch", Timestamp: ts, Status: model.StatusCleared, Type: model.TxTypeExpense,
})
```

Also update calls in `internal/service/errors_test.go` that test `CreateSimpleTransaction` (lines 271, 283).

- [ ] **Step 5: Run transaction tests to verify**

Run: `go test ./internal/service/ -run "TestCreateSimpleTransaction|TestCreateTransactionFromSplits" -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go internal/service/errors_test.go
git commit -m "refactor: CreateSimpleTransaction and CreateTransactionFromSplits accept input structs (issue #79)"
```

---

### Task 5: Refactor TransactionService — UpdateTransactionComplete

**Files:**
- Modify: `internal/service/transaction_ops.go`

- [ ] **Step 1: Run existing update tests to establish baseline**

Run: `go test ./internal/service/ -run "TestUpdateTransactionComplete" -v`
Expected: all PASS

- [ ] **Step 2: Refactor UpdateTransactionComplete**

In `internal/service/transaction_ops.go`, change (line 251) from:
```go
func (ts *TransactionService) UpdateTransactionComplete(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, splits []model.SplitDetail) error {
```
to:
```go
func (ts *TransactionService) UpdateTransactionComplete(ctx context.Context, input model.UpdateTransactionInput) error {
```

Replace all references inside the method body:
- `txID` → `input.ID`
- `description` → `input.Description`
- `timestamp` → `input.Timestamp`
- `status` → `input.Status`
- `txType` → `input.Type`
- `splits` → `input.Splits`

The full updated method body (showing only the parts that change variable names):

```go
func (ts *TransactionService) UpdateTransactionComplete(ctx context.Context, input model.UpdateTransactionInput) error {
	if input.Status != model.StatusPending && input.Status != model.StatusCleared && input.Status != model.StatusReconciled {
		return validationErrorf("status", "invalid status: must be 0 (Pending), 1 (Cleared) or 2 (Reconciled)")
	}

	if input.ID == model.SystemTransactionID {
		return fmt.Errorf("cannot modify the initial opening transaction: %w", ErrNotEditable)
	}

	oldTx, err := ts.txRepo.GetTransactionByID(ctx, input.ID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if oldTx.Status == model.StatusReconciled {
		return fmt.Errorf("transaction #%d cannot be modified: %w", oldTx.ID, ErrReconciled)
	}

	if len(input.Splits) < 2 {
		return validationErrorf("splits", "transaction must have at least 2 splits for double-entry bookkeeping")
	}

	if err := ts.ValidateSplitDetailsBalance(input.Splits); err != nil {
		return err
	}

	if err := ts.ValidateSplitsMatchType(ctx, input.Type, input.Splits); err != nil {
		return fmt.Errorf("splits do not match transaction type %q: %w", input.Type, err)
	}

	return ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
		if err := repo.UpdateTransactionBasic(ctx, input.ID, input.Description, input.Timestamp, input.Status, input.Type); err != nil {
			return err
		}

		existingSplits, err := repo.GetSplitsByTransaction(ctx, input.ID)
		if err != nil {
			return err
		}

		existingAccountByID := make(map[int64]int64, len(existingSplits))
		existingSplitMap := make(map[int64]*model.Split)
		for _, s := range existingSplits {
			existingAccountByID[s.ID] = s.AccountID
			existingSplitMap[s.ID] = s
		}

		seenSplitIDs := make(map[int64]bool, len(input.Splits))
		for _, split := range input.Splits {
			if split.ID == 0 {
				continue
			}
			if seenSplitIDs[split.ID] {
				return validationErrorf("splits", "duplicate split ID %d in input", split.ID)
			}
			seenSplitIDs[split.ID] = true
			if _, ok := existingAccountByID[split.ID]; !ok {
				return validationErrorf("splits", "split ID %d does not belong to transaction %d", split.ID, input.ID)
			}
		}

		for _, split := range input.Splits {
			account, err := repo.GetAccountByID(ctx, split.AccountID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return validationErrorf("splits", "account ID %d not found", split.AccountID)
				}
				return err
			}
			isNew := split.ID == 0
			accountChanged := split.ID != 0 && existingAccountByID[split.ID] != split.AccountID
			if isNew || accountChanged {
				if err := ts.checkAccountSelectable(ctx, repo, account); err != nil {
					return fmt.Errorf("split (account ID %d): %w", split.AccountID, err)
				}
			}
		}

		newSplitMap := make(map[int64]bool)
		for _, split := range input.Splits {
			if split.ID != 0 {
				newSplitMap[split.ID] = true
			}
		}

		for id := range existingSplitMap {
			if !newSplitMap[id] {
				if err := repo.DeleteSplit(ctx, id); err != nil {
					return fmt.Errorf("failed to delete split: %w", err)
				}
			}
		}

		for _, split := range input.Splits {
			if split.ID == 0 {
				newSplit := &model.Split{
					TransactionID: input.ID,
					AccountID:     split.AccountID,
					Amount:        split.Amount,
					Currency:      split.Currency,
					Memo:          split.Memo,
				}
				_, err := repo.CreateSplit(ctx, input.ID, newSplit)
				if err != nil {
					return err
				}
			} else {
				if err := repo.UpdateSplit(ctx, split.ID, split.AccountID, split.Amount, split.Currency, split.Memo); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
```

- [ ] **Step 3: Update service-layer tests**

In `internal/service/transaction_ops_test.go`, update every `UpdateTransactionComplete` call. Pattern:

Before:
```go
err := svc.UpdateTransactionComplete(ctx, txID, "updated", ts, model.StatusCleared, model.TxTypeExpense, splits)
```
After:
```go
err := svc.UpdateTransactionComplete(ctx, model.UpdateTransactionInput{
    ID: txID, Description: "updated", Timestamp: ts,
    Status: model.StatusCleared, Type: model.TxTypeExpense, Splits: splits,
})
```

- [ ] **Step 4: Run update tests to verify**

Run: `go test ./internal/service/ -run "TestUpdateTransactionComplete" -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "refactor: UpdateTransactionComplete accepts UpdateTransactionInput (issue #79)"
```

---

### Task 6: Update transaction-related cmd callers

**Files:**
- Modify: `cmd/add_types.go`
- Modify: `cmd/add.go`
- Modify: `cmd/add_test.go`
- Modify: `cmd/transaction/edit_types.go`
- Modify: `cmd/transaction/edit_actions.go`

- [ ] **Step 1: Update TransactionProvider interface in cmd/add_types.go**

Change lines 25-26 from:
```go
CreateSimpleTransaction(ctx context.Context, fromAccount string, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error)
CreateTransactionFromSplits(ctx context.Context, splits []model.SplitDetail, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error)
```
to:
```go
CreateSimpleTransaction(ctx context.Context, input model.CreateSimpleTransactionInput) (model.TransactionDetail, error)
CreateTransactionFromSplits(ctx context.Context, input model.CreateTransactionFromSplitsInput) (model.TransactionDetail, error)
```

- [ ] **Step 2: Update call site in cmd/add.go**

Change lines 88-103 from:
```go
var result model.TransactionDetail
if len(input.Splits) > 0 {
    result, err = r.txSvc.CreateTransactionFromSplits(
        ctx, input.Splits, input.Description, input.Timestamp, input.Status, input.Type,
    )
} else {
    result, err = r.txSvc.CreateSimpleTransaction(
        ctx,
        input.FromAccountID,
        input.ToAccountID,
        input.AmountCents,
        input.Description,
        input.Timestamp,
        input.Status,
        input.Type,
    )
}
```
to:
```go
var result model.TransactionDetail
if len(input.Splits) > 0 {
    result, err = r.txSvc.CreateTransactionFromSplits(ctx, model.CreateTransactionFromSplitsInput{
        Splits: input.Splits, Description: input.Description,
        Timestamp: input.Timestamp, Status: input.Status, Type: input.Type,
    })
} else {
    result, err = r.txSvc.CreateSimpleTransaction(ctx, model.CreateSimpleTransactionInput{
        FromAccount: input.FromAccountID, ToAccount: input.ToAccountID,
        Amount: input.AmountCents, Description: input.Description,
        Timestamp: input.Timestamp, Status: input.Status, Type: input.Type,
    })
}
```

- [ ] **Step 3: Update mock implementations in cmd/add_test.go**

Change the mock method signatures (lines 31, 38) to match the new interface:

Before:
```go
func (m *mockTransactionProvider) CreateSimpleTransaction(_ context.Context, fromAccount, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error) {
```
After:
```go
func (m *mockTransactionProvider) CreateSimpleTransaction(_ context.Context, input model.CreateSimpleTransactionInput) (model.TransactionDetail, error) {
```

Before:
```go
func (m *mockTransactionProvider) CreateTransactionFromSplits(_ context.Context, splits []model.SplitDetail, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error) {
```
After:
```go
func (m *mockTransactionProvider) CreateTransactionFromSplits(_ context.Context, input model.CreateTransactionFromSplitsInput) (model.TransactionDetail, error) {
```

Update the mock bodies to use `input.Field` instead of positional params. The mock return values stay the same — only the parameter access changes.

- [ ] **Step 4: Update EditProvider interface in cmd/transaction/edit_types.go**

Change line 40 from:
```go
UpdateTransactionComplete(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, splits []model.SplitDetail) error
```
to:
```go
UpdateTransactionComplete(ctx context.Context, input model.UpdateTransactionInput) error
```

- [ ] **Step 5: Update call site in cmd/transaction/edit_actions.go**

Change the `actionSave` method (lines 287-289) from:
```go
if err := r.txSvc.UpdateTransactionComplete(
    ctx, r.txID, detail.Description, detail.Timestamp, detail.Status, detail.Type, splits,
); err != nil {
```
to:
```go
if err := r.txSvc.UpdateTransactionComplete(ctx, model.UpdateTransactionInput{
    ID: r.txID, Description: detail.Description, Timestamp: detail.Timestamp,
    Status: detail.Status, Type: detail.Type, Splits: splits,
}); err != nil {
```

- [ ] **Step 6: Run all cmd tests**

Run: `go test ./cmd/... -v`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/add_types.go cmd/add.go cmd/add_test.go cmd/transaction/edit_types.go cmd/transaction/edit_actions.go
git commit -m "refactor: update transaction cmd callers for input structs (issue #79)"
```

---

### Task 7: Final verification and cleanup

**Files:**
- No new changes — verification only

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: all PASS

- [ ] **Step 2: Build the binary**

Run: `make build`
Expected: success

- [ ] **Step 3: Verify no remaining positional calls**

Run: `grep -rn 'CreateSimpleTransaction(ctx' --include='*.go' | grep -v 'input model\.' | grep -v '_test.go' | grep -v 'interface'`

Run: `grep -rn 'CreateAccountWithBalance(ctx' --include='*.go' | grep -v 'input model\.' | grep -v '_test.go' | grep -v 'interface'`

Run: `grep -rn 'UpdateTransactionComplete(ctx' --include='*.go' | grep -v 'input model\.' | grep -v '_test.go' | grep -v 'interface'`

Expected: no results (all call sites use the struct form)

- [ ] **Step 4: Commit any stragglers if needed, then final commit**

If everything is clean, no commit needed. If any files were missed, commit them with:
```bash
git commit -m "refactor: fix remaining input struct call sites (issue #79)"
```
