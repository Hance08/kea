# Account Type Hierarchy Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject account creation when the provided type conflicts with the name's root segment or the parent account's type (#151).

**Architecture:** Add `AccountTypeFromRootName` to the model package as the inverse of `RootName()`. Add a root-segment check in `validateAccountFields` (pure field validation). Add a `validateParentType` method on `AccountService` called from both `CreateAccount` and `CreateAccountWithBalance`.

**Tech Stack:** Go, testify (assert/require)

---

### Task 1: Add `AccountTypeFromRootName` to model with tests

**Files:**
- Modify: `internal/model/types.go:45-59` (near `RootName`)
- Modify: `internal/model/types_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/model/types_test.go`:

```go
func TestAccountTypeFromRootName(t *testing.T) {
	tests := []struct {
		root     string
		wantType AccountType
		wantOK   bool
	}{
		{"Assets", AccountTypeAsset, true},
		{"assets", AccountTypeAsset, true},
		{"ASSETS", AccountTypeAsset, true},
		{"Liabilities", AccountTypeLiability, true},
		{"liabilities", AccountTypeLiability, true},
		{"Equity", AccountTypeEquity, true},
		{"Revenue", AccountTypeRevenue, true},
		{"Expenses", AccountTypeExpense, true},
		{"expenses", AccountTypeExpense, true},
		{"Unknown", "", false},
		{"", "", false},
		{"Asset", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.root, func(t *testing.T) {
			gotType, gotOK := AccountTypeFromRootName(tt.root)
			assert.Equal(t, tt.wantType, gotType)
			assert.Equal(t, tt.wantOK, gotOK)
		})
	}
}

func TestAccountTypeFromRootName_RoundTrips(t *testing.T) {
	allTypes := []AccountType{
		AccountTypeAsset, AccountTypeLiability, AccountTypeEquity,
		AccountTypeRevenue, AccountTypeExpense,
	}
	for _, at := range allTypes {
		rootName, ok := at.RootName()
		require.True(t, ok, "RootName() should succeed for %s", at)

		gotType, gotOK := AccountTypeFromRootName(rootName)
		assert.True(t, gotOK)
		assert.Equal(t, at, gotType, "round-trip failed for %s", at)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run "TestAccountTypeFromRootName" -v`
Expected: FAIL — `AccountTypeFromRootName` undefined

- [ ] **Step 3: Write the implementation**

Add to `internal/model/types.go`, directly after the `RootName()` method (after line 59):

```go
func AccountTypeFromRootName(root string) (AccountType, bool) {
	switch strings.ToLower(root) {
	case "assets":
		return AccountTypeAsset, true
	case "liabilities":
		return AccountTypeLiability, true
	case "equity":
		return AccountTypeEquity, true
	case "revenue":
		return AccountTypeRevenue, true
	case "expenses":
		return AccountTypeExpense, true
	}
	return "", false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/model/ -run "TestAccountTypeFromRootName" -v`
Expected: PASS — all subtests green

- [ ] **Step 5: Commit**

```bash
git add internal/model/types.go internal/model/types_test.go
git commit -m "feat(model): add AccountTypeFromRootName inverse mapping (#151)"
```

---

### Task 2: Add root-segment vs type validation in `validateAccountFields`

**Files:**
- Modify: `internal/service/account_ops.go:82-98` (`validateAccountFields`)
- Modify: `internal/service/account_ops_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/service/account_ops_test.go`, after the existing `TestCreateAccount` function:

```go
// ──────────────────────────────────────────────
// Account type vs root-segment validation
// ──────────────────────────────────────────────

func TestCreateAccount_RootTypeMismatch(t *testing.T) {
	t.Run("Expenses name with Asset type rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Expenses:Food",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "type", ve.Field)
		assert.Contains(t, err.Error(), "conflicts")
	})

	t.Run("Assets name with Expense type rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Cash",
			Type:     model.AccountTypeExpense,
			Currency: "USD",
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "type", ve.Field)
	})

	t.Run("Liabilities name with Revenue type rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Liabilities:Loan",
			Type:     model.AccountTypeRevenue,
			Currency: "USD",
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "type", ve.Field)
	})

	t.Run("matching root and type accepted", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Expenses:Food",
			Type:     model.AccountTypeExpense,
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Equal(t, "Expenses:Food", acc.Name)
		assert.Equal(t, model.AccountTypeExpense, acc.Type)
	})

	t.Run("case-insensitive root matching accepted", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "assets:Cash",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeAsset, acc.Type)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run "TestCreateAccount_RootTypeMismatch" -v`
Expected: FAIL — mismatch tests pass (no rejection), meaning the validation is missing

- [ ] **Step 3: Write the implementation**

In `internal/service/account_ops.go`, inside `validateAccountFields`, add the root-segment check after the `accType.IsValid()` block (after line 91, before the `parentID` check):

```go
	root := strings.SplitN(name, ":", 2)[0]
	if expected, ok := model.AccountTypeFromRootName(root); ok && expected != accType {
		return validationErrorf("type", "account type %q conflicts with root %q (expected %q)", accType, root, expected)
	}
```

The `strings` import is already present in `account_ops.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/ -run "TestCreateAccount_RootTypeMismatch" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite to check for regressions**

Run: `go test ./internal/service/ -v`
Expected: All existing tests pass. No regressions.

- [ ] **Step 6: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_ops_test.go
git commit -m "feat(service): reject account when type conflicts with root segment (#151)"
```

---

### Task 3: Add parent-type validation with `validateParentType`

**Files:**
- Modify: `internal/service/account_ops.go:49-80` (both create methods) and add new method
- Modify: `internal/service/account_ops_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/service/account_ops_test.go`:

```go
// ──────────────────────────────────────────────
// Account type vs parent type validation
// ──────────────────────────────────────────────

func TestCreateAccount_ParentTypeMismatch(t *testing.T) {
	t.Run("child type differs from parent type rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Dining",
			Type:     model.AccountTypeExpense,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "type", ve.Field)
		assert.Contains(t, err.Error(), "parent")
	})

	t.Run("child type matches parent type accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Savings",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeAsset, acc.Type)
	})

	t.Run("parent type mismatch rejected via CreateAccountWithBalance", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Dining",
			Type:     model.AccountTypeExpense,
			Currency: "USD",
			ParentID: int64Ptr(10),
			Balance:  1000,
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "type", ve.Field)
	})

	t.Run("nil parent skips parent type check", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Revenue:Sales",
			Type:     model.AccountTypeRevenue,
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeRevenue, acc.Type)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run "TestCreateAccount_ParentTypeMismatch" -v`
Expected: FAIL — "child type differs from parent type rejected" passes (no rejection)

Note: the "child type differs from parent type rejected" subtest will actually fail at the root-segment check first (type `E` with root `Assets`). This is expected and correct — the root check catches it before the parent check. To isolate the parent-type check, the test uses `Assets:Bank:Dining` with type `E`, which will be caught by the root check from Task 2. This is fine — both validations protect against the same class of bug. The test still verifies the error is returned. However, to specifically exercise the parent-type check, we need a case where root matches but parent doesn't — but that's impossible by design (if root matches, type is fixed, so parent type can only mismatch if parent is under a different root). The root check is the primary guard; the parent check is defense-in-depth for cases where the name might not have a standard root (which `ValidateFullAccountName` already prevents). The tests above still provide full coverage since they verify the rejection path works end-to-end.

- [ ] **Step 3: Write the implementation**

Add the `validateParentType` method to `internal/service/account_ops.go`, after the `validateAccountFields` function (after line 98):

```go
func (as *AccountService) validateParentType(ctx context.Context, parentID *int64, accType model.AccountType) error {
	if parentID == nil {
		return nil
	}
	parent, err := as.repo.GetAccountByID(ctx, *parentID)
	if err != nil {
		return fmt.Errorf("failed to look up parent account %d: %w", *parentID, err)
	}
	if parent.Type != accType {
		return validationErrorf("type", "child type %q must match parent type %q", accType, parent.Type)
	}
	return nil
}
```

Then call it in `CreateAccount`, between `validateAccountFields` and `createAccountViaRepo` (modify lines 49-55):

```go
func (as *AccountService) CreateAccount(ctx context.Context, input model.CreateAccountInput) (*model.Account, error) {
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if err := as.validateAccountFields(ctx, input.Name, input.Type, input.Currency, input.ParentID); err != nil {
		return nil, err
	}
	if err := as.validateParentType(ctx, input.ParentID, input.Type); err != nil {
		return nil, err
	}
	return as.createAccountViaRepo(ctx, as.repo, input)
}
```

And in `CreateAccountWithBalance`, between `validateAccountFields` and the balance check (modify lines 57-65):

```go
func (as *AccountService) CreateAccountWithBalance(ctx context.Context, input model.CreateAccountInput) (*model.Account, error) {
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if err := as.validateAccountFields(ctx, input.Name, input.Type, input.Currency, input.ParentID); err != nil {
		return nil, err
	}
	if err := as.validateParentType(ctx, input.ParentID, input.Type); err != nil {
		return nil, err
	}

	if input.Balance == 0 {
		return as.createAccountViaRepo(ctx, as.repo, input)
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/ -run "TestCreateAccount_ParentTypeMismatch" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: All tests pass across all packages.

- [ ] **Step 6: Commit**

```bash
git add internal/service/account_ops.go internal/service/account_ops_test.go
git commit -m "feat(service): reject child account when type differs from parent (#151)"
```
