# Split Type Trust Boundary Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make all account-type resolution in `TransactionService` use the repository as the source of truth, never trusting caller-supplied `SplitDetail.AccountType`.

**Architecture:** Extract a shared `resolveAccountType` helper that resolves by `AccountID` or `AccountName` from the repo. Replace the four inline `resolveType` closures in `transaction_classifier.go` with this helper. Update existing tests to provide accounts in the mock repo since the `AccountType` field is no longer read.

**Tech Stack:** Go, testify (assert/require)

---

### Task 1: Add `resolveAccountType` helper and update `ValidateSplitsMatchType`

**Files:**
- Modify: `internal/service/transaction_classifier.go:318-328`

- [ ] **Step 1: Write the `resolveAccountType` method**

Add this method above `ValidateSplitsMatchType` in `internal/service/transaction_classifier.go`:

```go
func (ts *TransactionService) resolveAccountType(ctx context.Context, s model.SplitDetail) (model.AccountType, error) {
	if s.AccountID > 0 {
		acc, err := ts.accRepo.GetAccountByID(ctx, s.AccountID)
		if err != nil {
			return "", err
		}
		return acc.Type, nil
	}
	if s.AccountName != "" {
		acc, err := ts.accRepo.GetAccountByName(ctx, s.AccountName)
		if err != nil {
			return "", err
		}
		return acc.Type, nil
	}
	return "", fmt.Errorf("split has neither AccountID nor AccountName")
}
```

- [ ] **Step 2: Replace the `resolveType` closure in `ValidateSplitsMatchType`**

In `ValidateSplitsMatchType` (line ~318), remove the inline `resolveType` closure (lines 319-328) and replace all calls to `resolveType(s)` with `ts.resolveAccountType(ctx, s)`.

The function should look like:

```go
func (ts *TransactionService) ValidateSplitsMatchType(ctx context.Context, txType model.TransactionType, splits []model.SplitDetail) error {
	switch txType {
	case model.TxTypeOpening, model.TxTypeOther, model.TxTypeDeposit, model.TxTypeWithdrawal:
		return nil

	case model.TxTypeExpense:
		var hasExpense, hasAssetOrLiab bool
		for _, s := range splits {
			accType, err := ts.resolveAccountType(ctx, s)
			if err != nil {
				return err
			}
			// ... rest unchanged
```

- [ ] **Step 3: Verify build compiles**

Run: `cd /Users/hance/programming/kea && go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add internal/service/transaction_classifier.go
git commit -m "refactor: extract resolveAccountType and update ValidateSplitsMatchType (#153)"
```

---

### Task 2: Update `DetermineType`, `GetDisplayAccount`, and `GetDisplayOffsetAccount`

**Files:**
- Modify: `internal/service/transaction_classifier.go:14-41` (DetermineType)
- Modify: `internal/service/transaction_classifier.go:109-128` (GetDisplayAccount)
- Modify: `internal/service/transaction_classifier.go:202-217` (GetDisplayOffsetAccount)

- [ ] **Step 1: Update `DetermineType`**

Replace the inline resolution block (lines 34-41):

```go
		accType := split.AccountType
		if accType == "" {
			acc, err := ts.accRepo.GetAccountByID(ctx, split.AccountID)
			if err != nil {
				return model.TxTypeOther, err
			}
			accType = acc.Type
		}
```

With:

```go
		accType, err := ts.resolveAccountType(ctx, split)
		if err != nil {
			return model.TxTypeOther, err
		}
```

- [ ] **Step 2: Update `GetDisplayAccount`**

Remove the inline `resolveType` closure (lines 114-122) and replace all `resolveType(split)` calls with `ts.resolveAccountType(ctx, split)`.

- [ ] **Step 3: Update `GetDisplayOffsetAccount`**

Remove the inline `resolveAccountType` closure (lines 207-216) and replace all `resolveAccountType(split)` calls with `ts.resolveAccountType(ctx, split)`.

- [ ] **Step 4: Verify build compiles**

Run: `cd /Users/hance/programming/kea && go build ./...`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_classifier.go
git commit -m "refactor: use resolveAccountType in DetermineType and display helpers (#153)"
```

---

### Task 3: Fix existing tests to provide accounts in mock repos

After the changes, existing tests that rely on `SplitDetail.AccountType` being trusted will fail because the resolver now queries the repo. Tests that use the `split()` helper with an empty mock repo need accounts added.

**Files:**
- Modify: `internal/service/transaction_classifier_test.go`

- [ ] **Step 1: Create a shared test helper that builds a mock repo with standard accounts**

Add a helper function near the top of `transaction_classifier_test.go` (after the existing `split`/`splitWithMemo` helpers):

```go
func classifierAccRepo() *mockAccountRepo {
	repo := newMockAccountRepo()
	repo.addAccount(&model.Account{ID: 1, Name: "Expenses:Food", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 3, Name: "Revenue:Salary", Type: model.AccountTypeRevenue})
	repo.addAccount(&model.Account{ID: 4, Name: "Liabilities:Card", Type: model.AccountTypeLiability})
	repo.addAccount(&model.Account{ID: 5, Name: model.OpeningBalancesAccountName("USD"), Type: model.AccountTypeEquity})
	repo.addAccount(&model.Account{ID: 6, Name: "Assets:Cash", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 7, Name: "Assets:Savings", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 8, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 9, Name: "Equity:Retained", Type: model.AccountTypeEquity})
	repo.addAccount(&model.Account{ID: 10, Name: "Expenses:Drink", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 11, Name: "Revenue:Bonus", Type: model.AccountTypeRevenue})
	repo.addAccount(&model.Account{ID: 12, Name: "Expenses:Tax", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 13, Name: "Expenses:A", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 14, Name: "Expenses:B", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 15, Name: "Assets:Investments:00878", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 16, Name: "Expenses:Fees:Stocks", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 17, Name: "Assets:Bank:DAWHO", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 18, Name: "Expenses:Food:Drink", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 19, Name: "Assets:Receivable:Friends", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 20, Name: "Assets:Ewallet:LinePayMoney", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 21, Name: model.OpeningBalancesAccountName("TWD"), Type: model.AccountTypeEquity})
	repo.addAccount(&model.Account{ID: 22, Name: "Assets:Card", Type: model.AccountTypeAsset})
	return repo
}
```

- [ ] **Step 2: Update `TestDetermineType` to use `classifierAccRepo()`**

Change line:
```go
svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
```
To:
```go
svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
```

- [ ] **Step 3: Update `TestValidateSplitsMatchType` to use `classifierAccRepo()`**

Same change — replace `newMockAccountRepo()` with `classifierAccRepo()`.

- [ ] **Step 4: Update `TestGetDisplayOffsetAccount` subtests that use inline `AccountType`**

The subtests "Expense: single offset", "Expense: multiple offsets", "Transfer: excludes primary", and "no offset accounts" create an empty mock and rely on `AccountType`. Update each to use `classifierAccRepo()` and remove the `AccountType` field from splits (or leave it — it's now ignored, but removing it makes the test clearer).

For example, the first subtest becomes:
```go
t.Run("Expense: single offset (asset account)", func(t *testing.T) {
    svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
    splits := []model.SplitDetail{
        {AccountName: "Expenses:Food", Amount: 500},
        {AccountName: "Assets:Cash", Amount: -500},
    }
    got, err := svc.GetDisplayOffsetAccount(context.Background(), splits, "Expense", "Expenses:Food")
    require.NoError(t, err)
    assert.Equal(t, "Assets:Cash", got)
})
```

- [ ] **Step 5: Update `TestBuildTransactionListItems` tests**

The `TestBuildTransactionListItems`, `TestBuildTransactionListItems_MultipleTransactions` tests already have accounts in the mock for name resolution. However, the splits use the `split()` helper which sets `AccountType`. These tests should still pass since `AccountName` is set and the accounts are in the mock. Verify by running:

Run: `go test ./internal/service/ -run TestBuildTransactionListItems -v`
Expected: All PASS.

- [ ] **Step 6: Run all tests**

Run: `go test ./internal/service/ -v`
Expected: All PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/service/transaction_classifier_test.go
git commit -m "test: update classifier tests to use repo-backed account resolution (#153)"
```

---

### Task 4: Add tests proving fake `AccountType` is rejected

**Files:**
- Modify: `internal/service/transaction_classifier_test.go`

- [ ] **Step 1: Write test for `ValidateSplitsMatchType` rejecting a lying `AccountType`**

Add after the existing `TestValidateSplitsMatchType` function:

```go
func TestValidateSplitsMatchType_IgnoresCallerAccountType(t *testing.T) {
	accRepo := classifierAccRepo()
	svc := newTestTransactionService(accRepo, newMockTransactionRepo())

	t.Run("lying AccountType on expense tx is rejected", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: -1000},
			{AccountName: "Revenue:Salary", AccountType: model.AccountTypeExpense, Amount: 1000},
		}
		err := svc.ValidateSplitsMatchType(context.Background(), model.TxTypeExpense, splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Expense account")
	})

	t.Run("lying AccountType on income tx is rejected", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 1000},
			{AccountName: "Expenses:Food", AccountType: model.AccountTypeRevenue, Amount: -1000},
		}
		err := svc.ValidateSplitsMatchType(context.Background(), model.TxTypeIncome, splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Revenue account")
	})

	t.Run("lying AccountType on transfer tx is rejected", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 500},
			{AccountName: "Expenses:Food", AccountType: model.AccountTypeAsset, Amount: -500},
		}
		err := svc.ValidateSplitsMatchType(context.Background(), model.TxTypeTransfer, splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Asset and Liability")
	})

	t.Run("correct AccountType still passes (repo is truth)", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500},
			{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: -500},
		}
		err := svc.ValidateSplitsMatchType(context.Background(), model.TxTypeExpense, splits)
		require.NoError(t, err)
	})
}
```

- [ ] **Step 2: Write test for `DetermineType` ignoring fake `AccountType`**

```go
func TestDetermineType_IgnoresCallerAccountType(t *testing.T) {
	accRepo := classifierAccRepo()
	svc := newTestTransactionService(accRepo, newMockTransactionRepo())

	t.Run("Revenue account claimed as Expense resolves correctly", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountName: "Revenue:Salary", AccountType: model.AccountTypeExpense, Amount: -1000},
			{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 1000},
		}
		got, err := svc.DetermineType(context.Background(), splits)
		require.NoError(t, err)
		assert.Equal(t, model.TxTypeIncome, got)
	})

	t.Run("resolves by AccountID when both ID and name are set", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountID: 1, AccountName: "Expenses:Food", AccountType: model.AccountTypeRevenue, Amount: 500},
			{AccountID: 2, AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: -500},
		}
		got, err := svc.DetermineType(context.Background(), splits)
		require.NoError(t, err)
		assert.Equal(t, model.TxTypeExpense, got)
	})
}
```

- [ ] **Step 3: Run the new tests**

Run: `go test ./internal/service/ -run "IgnoresCallerAccountType" -v`
Expected: All PASS.

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_classifier_test.go
git commit -m "test: add tests proving fake AccountType is rejected (#153)"
```
