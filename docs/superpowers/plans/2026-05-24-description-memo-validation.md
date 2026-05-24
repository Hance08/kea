# Description & Memo Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add length and emptiness validation for Description and Memo fields in the service layer to prevent DB bloat and provide defense-in-depth (issue #123).

**Architecture:** Inline `validationErrorf()` checks in each affected service method, following the existing pattern used for `AccountNameMaxLength`. Two new constants in `model/types.go`. Transaction descriptions are required (non-empty); account descriptions and split memos are optional (length-only).

**Tech Stack:** Go, testify (assert/require)

---

### Task 1: Add constants to model

**Files:**
- Modify: `internal/model/types.go:26-31`

- [ ] **Step 1: Write the failing test**

Create file `internal/model/types_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescriptionMaxLength(t *testing.T) {
	assert.Equal(t, 500, DescriptionMaxLength)
}

func TestMemoMaxLength(t *testing.T) {
	assert.Equal(t, 200, MemoMaxLength)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run "TestDescriptionMaxLength|TestMemoMaxLength" -v`
Expected: FAIL — `DescriptionMaxLength` and `MemoMaxLength` undefined

- [ ] **Step 3: Add the constants**

In `internal/model/types.go`, add to the existing constant block at line 26:

```go
const (
	AccountNameMaxLength  = 100
	DescriptionMaxLength  = 500
	MemoMaxLength         = 200
	MaxSafeBalanceFloat   = 9223372036854775.0
	OpeningAccountMemo    = "Opening Balance"
	TypeEquity            = "C"
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/model/ -run "TestDescriptionMaxLength|TestMemoMaxLength" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/model/types.go internal/model/types_test.go
git commit -m "feat: add DescriptionMaxLength and MemoMaxLength constants (#123)"
```

---

### Task 2: Validate Description and Memo in CreateTransaction

**Files:**
- Modify: `internal/service/transaction_ops.go:6-14` (add `"strings"` import)
- Modify: `internal/service/transaction_ops.go:32-39` (add description validation after type check)
- Modify: `internal/service/transaction_ops.go:50-73` (add memo validation in split loop)
- Test: `internal/service/transaction_ops_test.go`

- [ ] **Step 1: Write the failing tests**

First, add `"strings"` to the import block in `internal/service/transaction_ops_test.go`:

```go
import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

Then add new test function:

```go
func TestCreateTransaction_DescriptionValidation(t *testing.T) {
	makeInput := func(desc string, memo string) model.TransactionDetail {
		return model.TransactionDetail{
			Description: desc,
			Type:        model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500, Memo: memo},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
	}

	t.Run("empty description rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateTransaction(context.Background(), makeInput("", ""))
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
		assert.Contains(t, ve.Message, "required")
	})

	t.Run("whitespace-only description rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateTransaction(context.Background(), makeInput("   ", ""))
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
	})

	t.Run("over-length description rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		longDesc := strings.Repeat("a", model.DescriptionMaxLength+1)
		_, err := svc.CreateTransaction(context.Background(), makeInput(longDesc, ""))
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
		assert.Contains(t, ve.Message, "too long")
	})

	t.Run("exactly max-length description accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		exactDesc := strings.Repeat("a", model.DescriptionMaxLength)
		id, err := svc.CreateTransaction(context.Background(), makeInput(exactDesc, ""))
		require.NoError(t, err)
		assert.Greater(t, id, int64(0))
	})

	t.Run("over-length memo on split rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		longMemo := strings.Repeat("m", model.MemoMaxLength+1)
		_, err := svc.CreateTransaction(context.Background(), makeInput("Valid desc", longMemo))
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "memo", ve.Field)
		assert.Contains(t, ve.Message, "too long")
	})

	t.Run("exactly max-length memo accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		exactMemo := strings.Repeat("m", model.MemoMaxLength)
		id, err := svc.CreateTransaction(context.Background(), makeInput("Valid desc", exactMemo))
		require.NoError(t, err)
		assert.Greater(t, id, int64(0))
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestCreateTransaction_DescriptionValidation -v`
Expected: FAIL — empty description and over-length tests pass when they should fail (no validation yet)

- [ ] **Step 3: Add validation to CreateTransaction**

In `internal/service/transaction_ops.go`:

1. Add `"strings"` to the import block:

```go
import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)
```

2. After the status validation (line 39), add description validation:

```go
	if strings.TrimSpace(input.Description) == "" {
		return 0, validationErrorf("description", "description is required")
	}
	if len(input.Description) > model.DescriptionMaxLength {
		return 0, validationErrorf("description", "description too long (max %d characters)", model.DescriptionMaxLength)
	}
```

3. Inside the split loop (after building the `model.Split` struct around line 68-73), add memo validation before appending:

```go
		if len(splitInput.Memo) > model.MemoMaxLength {
			return 0, validationErrorf("memo", "split #%d memo too long (max %d characters)", i+1, model.MemoMaxLength)
		}

		splits = append(splits, model.Split{
```

- [ ] **Step 4: Fix existing tests that use empty Description**

Many existing `TestCreateTransaction` tests omit Description (which defaults to `""`). These will now fail because description is required. Add `Description: "test"` to every `TransactionDetail` and `CreateSimpleTransactionInput` in the test file that currently has an empty description. Specifically, update these test inputs (search for `model.TransactionDetail{` and `model.CreateSimpleTransactionInput{` without a `Description` field or with `Description: ""`):

- `fewer than 2 splits rejected` — add `Description: "test"` (won't reach description validation but keeps the input valid)
- `empty splits rejected` — add `Description: "test"`
- `error: missing type` — add `Description: "test"`
- `unbalanced splits rejected` — add `Description: "test"`
- `unknown account rejected` — add `Description: "test"`
- `zero timestamp is set to current time` — add `Description: "test"`
- `account currency overrides system default` — add `Description: "test"`
- `mixed currency splits rejected` — add `Description: "test"`
- `account without currency uses system default` — add `Description: "test"`
- And any other `CreateTransaction`/`UpdateTransactionComplete` test inputs with empty descriptions

For `UpdateTransactionComplete` tests: update all inputs that have `Description: ""` to `Description: "test"` — EXCEPT the "invalid status" test (line ~597) which fails before reaching description validation.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/service/ -run "TestCreateTransaction|TestUpdateTransactionComplete" -v`
Expected: ALL PASS (new validation tests + existing tests with fixed descriptions)

- [ ] **Step 6: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "feat: validate Description and Memo in CreateTransaction (#123)"
```

---

### Task 3: Validate Description and Memo in UpdateTransactionComplete

**Files:**
- Modify: `internal/service/transaction_ops.go:255-283` (add validation after status check)
- Test: `internal/service/transaction_ops_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/service/transaction_ops_test.go`:

```go
func TestUpdateTransactionComplete_DescriptionValidation(t *testing.T) {
	setupForUpdate := func() (*mockAccountRepo, *mockTransactionRepo, *TransactionService) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 1, Description: "Old", Status: model.StatusPending, Type: model.TxTypeExpense},
			[]*model.Split{
				{ID: 10, TransactionID: 1, AccountID: 2, Amount: 500, Currency: "USD"},
				{ID: 11, TransactionID: 1, AccountID: 1, Amount: -500, Currency: "USD"},
			},
		)
		svc := newTestTransactionService(accRepo, txRepo)
		return accRepo, txRepo, svc
	}

	makeInput := func(desc string, memo string) model.UpdateTransactionInput {
		return model.UpdateTransactionInput{
			ID:          1,
			Description: desc,
			Timestamp:   time.Now().Unix(),
			Status:      model.StatusPending,
			Type:        model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{ID: 10, AccountName: "Expenses:Food", Amount: 500, Memo: memo},
				{ID: 11, AccountName: "Assets:Bank", Amount: -500},
			},
		}
	}

	t.Run("empty description rejected", func(t *testing.T) {
		_, _, svc := setupForUpdate()
		err := svc.UpdateTransactionComplete(context.Background(), makeInput("", ""))
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
		assert.Contains(t, ve.Message, "required")
	})

	t.Run("whitespace-only description rejected", func(t *testing.T) {
		_, _, svc := setupForUpdate()
		err := svc.UpdateTransactionComplete(context.Background(), makeInput("   ", ""))
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
	})

	t.Run("over-length description rejected", func(t *testing.T) {
		_, _, svc := setupForUpdate()
		longDesc := strings.Repeat("a", model.DescriptionMaxLength+1)
		err := svc.UpdateTransactionComplete(context.Background(), makeInput(longDesc, ""))
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
		assert.Contains(t, ve.Message, "too long")
	})

	t.Run("over-length memo on split rejected", func(t *testing.T) {
		_, _, svc := setupForUpdate()
		longMemo := strings.Repeat("m", model.MemoMaxLength+1)
		err := svc.UpdateTransactionComplete(context.Background(), makeInput("Valid desc", longMemo))
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "memo", ve.Field)
		assert.Contains(t, ve.Message, "too long")
	})

	t.Run("valid description and memo accepted", func(t *testing.T) {
		_, _, svc := setupForUpdate()
		err := svc.UpdateTransactionComplete(context.Background(), makeInput("Updated lunch", "some memo"))
		require.NoError(t, err)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestUpdateTransactionComplete_DescriptionValidation -v`
Expected: FAIL — empty description and over-length tests pass when they should fail

- [ ] **Step 3: Add validation to UpdateTransactionComplete**

In `internal/service/transaction_ops.go`, in the `UpdateTransactionComplete` method, after the status validation (line 257) and before the `GetTransactionByID` call (line 260), add:

```go
	if strings.TrimSpace(input.Description) == "" {
		return validationErrorf("description", "description is required")
	}
	if len(input.Description) > model.DescriptionMaxLength {
		return validationErrorf("description", "description too long (max %d characters)", model.DescriptionMaxLength)
	}
```

After the `ValidateSplitsMatchType` call (line 281-283) and before the `ExecTx` call, add memo validation:

```go
	for i, s := range input.Splits {
		if len(s.Memo) > model.MemoMaxLength {
			return validationErrorf("memo", "split #%d memo too long (max %d characters)", i+1, model.MemoMaxLength)
		}
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run "TestUpdateTransactionComplete" -v`
Expected: ALL PASS (new validation tests + existing tests)

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "feat: validate Description and Memo in UpdateTransactionComplete (#123)"
```

---

### Task 4: Validate Description in CreateAccount and CreateAccountWithBalance

**Files:**
- Modify: `internal/service/account_ops.go:49-57` (add validation in CreateAccount)
- Modify: `internal/service/account_ops.go:60-67` (add validation in CreateAccountWithBalance)
- Test: `internal/service/account_ops_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/service/account_ops_test.go`:

```go
func TestCreateAccount_DescriptionValidation(t *testing.T) {
	t.Run("empty description is allowed", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		input := model.CreateAccountInput{
			Name:        "Checking",
			Type:        model.AccountTypeAsset,
			Currency:    "USD",
			Description: "",
		}
		acc, err := svc.CreateAccount(context.Background(), input)
		require.NoError(t, err)
		assert.NotNil(t, acc)
	})

	t.Run("over-length description rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		input := model.CreateAccountInput{
			Name:        "Checking",
			Type:        model.AccountTypeAsset,
			Currency:    "USD",
			Description: strings.Repeat("d", model.DescriptionMaxLength+1),
		}
		acc, err := svc.CreateAccount(context.Background(), input)
		assert.Nil(t, acc)
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
		assert.Contains(t, ve.Message, "too long")
	})

	t.Run("exactly max-length description accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		input := model.CreateAccountInput{
			Name:        "Checking",
			Type:        model.AccountTypeAsset,
			Currency:    "USD",
			Description: strings.Repeat("d", model.DescriptionMaxLength),
		}
		acc, err := svc.CreateAccount(context.Background(), input)
		require.NoError(t, err)
		assert.NotNil(t, acc)
	})
}

func TestCreateAccountWithBalance_DescriptionValidation(t *testing.T) {
	t.Run("over-length description rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		addOpeningBalancesForCurrency(accRepo, "USD")
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		input := model.CreateAccountInput{
			Name:        "Checking",
			Type:        model.AccountTypeAsset,
			Currency:    "USD",
			Description: strings.Repeat("d", model.DescriptionMaxLength+1),
			Balance:     10000,
		}
		acc, err := svc.CreateAccountWithBalance(context.Background(), input)
		assert.Nil(t, acc)
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestCreateAccount_DescriptionValidation|TestCreateAccountWithBalance_DescriptionValidation" -v`
Expected: FAIL — over-length tests pass when they should fail

- [ ] **Step 3: Add validation to both methods**

In `internal/service/account_ops.go`:

In `CreateAccount` (line 49), add before the `validateAccountFields` call:

```go
func (as *AccountService) CreateAccount(ctx context.Context, input model.CreateAccountInput) (*model.Account, error) {
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if len(input.Description) > model.DescriptionMaxLength {
		return nil, validationErrorf("description", "description too long (max %d characters)", model.DescriptionMaxLength)
	}
	if err := as.validateAccountFields(ctx, input.Name, input.Type, input.Currency, input.ParentID); err != nil {
```

In `CreateAccountWithBalance` (line 60), add the same check before `validateAccountFields`:

```go
func (as *AccountService) CreateAccountWithBalance(ctx context.Context, input model.CreateAccountInput) (*model.Account, error) {
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if len(input.Description) > model.DescriptionMaxLength {
		return nil, validationErrorf("description", "description too long (max %d characters)", model.DescriptionMaxLength)
	}
	if err := as.validateAccountFields(ctx, input.Name, input.Type, input.Currency, input.ParentID); err != nil {
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run "TestCreateAccount|TestCreateAccountWithBalance" -v`
Expected: ALL PASS (new validation tests + existing tests)

- [ ] **Step 5: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_ops_test.go
git commit -m "feat: validate Description in CreateAccount and CreateAccountWithBalance (#123)"
```

---

### Task 5: Validate Description in UpdateAccountMetadata

**Files:**
- Modify: `internal/service/account_service.go:168-183` (add validation at method entry)
- Modify: `internal/service/account_service_test.go:6-15` (add `"strings"` import)
- Test: `internal/service/account_service_test.go`

- [ ] **Step 1: Write the failing tests**

First, add `"strings"` to the import block in `internal/service/account_service_test.go`:

```go
import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

Then add to `internal/service/account_service_test.go`:

```go
func TestUpdateAccountMetadata_DescriptionValidation(t *testing.T) {
	t.Run("empty description is allowed", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.UpdateAccountMetadata(context.Background(), 1, "", false)
		require.NoError(t, err)
		assert.NotNil(t, acc)
	})

	t.Run("over-length description rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		longDesc := strings.Repeat("d", model.DescriptionMaxLength+1)
		acc, err := svc.UpdateAccountMetadata(context.Background(), 1, longDesc, false)
		assert.Nil(t, acc)
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
		assert.Contains(t, ve.Message, "too long")
	})

	t.Run("exactly max-length description accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		exactDesc := strings.Repeat("d", model.DescriptionMaxLength)
		acc, err := svc.UpdateAccountMetadata(context.Background(), 1, exactDesc, false)
		require.NoError(t, err)
		assert.NotNil(t, acc)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestUpdateAccountMetadata_DescriptionValidation -v`
Expected: FAIL — over-length test passes when it should fail

- [ ] **Step 3: Add validation to UpdateAccountMetadata**

In `internal/service/account_service.go`, at the start of `UpdateAccountMetadata` (line 168), add before the `GetAccountByID` call:

```go
func (as *AccountService) UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) (*model.Account, error) {
	if len(description) > model.DescriptionMaxLength {
		return nil, validationErrorf("description", "description too long (max %d characters)", model.DescriptionMaxLength)
	}

	acc, err := as.repo.GetAccountByID(ctx, accountID)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run "TestUpdateAccountMetadata" -v`
Expected: ALL PASS (new validation tests + existing tests)

- [ ] **Step 5: Commit**

```bash
git add internal/service/account_service.go internal/service/account_service_test.go
git commit -m "feat: validate Description in UpdateAccountMetadata (#123)"
```

---

### Task 6: Full regression test

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: ALL PASS — no regressions

Note: Tests in `internal/service/errors_test.go` (lines 230-264) create `TransactionDetail` without Description but they fail on splits/type validation before reaching description validation, so they won't break.

The `transaction_classifier_test.go` tests create `TransactionDetail` structs for classification logic (not passed to `CreateTransaction`), so those are unaffected.

- [ ] **Step 2: Commit (if any fixups needed)**

If any tests broke due to missing descriptions in test fixtures, fix them and commit:

```bash
git add -u
git commit -m "fix: update test fixtures with valid descriptions (#123)"
```
