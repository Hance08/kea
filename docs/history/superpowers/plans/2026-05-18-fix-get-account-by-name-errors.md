# Fix Ignored GetAccountByName Errors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix three ignored `GetAccountByName` errors in `cmd/transaction/edit_actions.go` that cause nil-pointer panics when the account lookup fails.

**Architecture:** Each of the three call sites discards the error with `_, _` and immediately dereferences the result. The fix checks the error and returns it, matching the existing pattern already used at line 71-74 of the same file.

**Tech Stack:** Go, existing `AccountProvider` interface

---

### Task 1: Add tests for error propagation in actionQuickChangeAccount

The `cmd/transaction` package has no tests yet. We need a test file with mock implementations of `EditView`, `EditProvider`, and `AccountProvider` to verify that `GetAccountByName` errors propagate correctly from each of the three call sites.

**Files:**
- Create: `cmd/transaction/edit_actions_test.go`

- [ ] **Step 1: Create test file with mocks and test for actionQuickChangeAccount (line 89)**

```go
package transaction

import (
	"context"
	"errors"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
)

var errNotFound = errors.New("account not found")

type stubAccountProvider struct {
	getByNameResult *model.Account
	getByNameErr    error
	getAllResult     []*model.Account
	getAllErr        error
}

func (s *stubAccountProvider) GetAccountByName(_ context.Context, _ string) (*model.Account, error) {
	return s.getByNameResult, s.getByNameErr
}

func (s *stubAccountProvider) GetAllAccounts(_ context.Context) ([]*model.Account, error) {
	return s.getAllResult, s.getAllErr
}

type stubEditProvider struct{}

func (s *stubEditProvider) GetTransactionByID(_ context.Context, _ int64) (*model.TransactionDetail, error) {
	return nil, nil
}
func (s *stubEditProvider) IsEditable(_ *model.TransactionDetail) (bool, service.NotEditableReason) {
	return true, ""
}
func (s *stubEditProvider) GetAllowedAccounts(_ model.TransactionType, _ model.AccountType, accs []*model.Account) []*model.Account {
	return accs
}
func (s *stubEditProvider) ValidateTransactionEdit(_ context.Context, _ []model.SplitDetail) error {
	return nil
}
func (s *stubEditProvider) ValidateSplitsMatchType(_ context.Context, _ model.TransactionType, _ []model.SplitDetail) error {
	return nil
}
func (s *stubEditProvider) UpdateTransactionComplete(_ context.Context, _ model.UpdateTransactionInput) error {
	return nil
}

type stubEditView struct {
	splitSelectionResult int
	accountFromListName  string
	amountResult         int64
	inputResult          string
}

func (s *stubEditView) ShowError(_ string, _ error)                        {}
func (s *stubEditView) ShowWarning(_ string)                               {}
func (s *stubEditView) ShowSuccess(_ string)                               {}
func (s *stubEditView) ShowInfo(_ string)                                  {}
func (s *stubEditView) RenderDetail(_ *model.TransactionDetail) error      { return nil }
func (s *stubEditView) RenderSplitsPreview(_ []model.SplitDetail)          {}
func (s *stubEditView) AskSelection(_ string, _ []string) (string, error)  { return "", nil }
func (s *stubEditView) AskInput(_ string, _ string) (string, error)        { return s.inputResult, nil }
func (s *stubEditView) AskConfirm(_ string) bool                           { return true }
func (s *stubEditView) AskDescription(c string) (string, error)            { return c, nil }
func (s *stubEditView) AskDate(c int64) (int64, error)                     { return c, nil }
func (s *stubEditView) AskStatus(c model.TransactionStatus) (model.TransactionStatus, error) {
	return c, nil
}
func (s *stubEditView) AskAccountFromList(_ []*model.Account, _ string) (string, error) {
	return s.accountFromListName, nil
}
func (s *stubEditView) AskAmount(_ string, c int64, _ bool) (int64, error) { return s.amountResult, nil }
func (s *stubEditView) AskSplitSelection(_ []model.SplitDetail) (int, error) {
	return s.splitSelectionResult, nil
}

func TestActionQuickChangeAccount_GetAccountByNameError(t *testing.T) {
	accProvider := &stubAccountProvider{
		getAllResult: []*model.Account{
			{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"},
		},
		getByNameErr: errNotFound,
	}
	// First call to GetAccountByName (line 71) succeeds for currentAcc lookup
	callCount := 0
	accProvider.getByNameResult = &model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"}
	originalGetByName := accProvider.getByNameErr
	_ = originalGetByName

	// We need a more sophisticated stub that fails on the second call
	secondCallFails := &accountProviderSecondCallFails{
		first: &model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"},
		err:   errNotFound,
	}
	_ = callCount

	r := &editRunner{
		txSvc:  &stubEditProvider{},
		accSvc: secondCallFails,
		view: &stubEditView{
			splitSelectionResult: 0,
			accountFromListName:  "Assets:Deleted",
		},
	}

	detail := &model.TransactionDetail{
		Type: model.TxTypeExpense,
		Splits: []model.SplitDetail{
			{AccountID: 1, AccountName: "Assets:Bank", Amount: -100},
			{AccountID: 2, AccountName: "Expenses:Food", Amount: 100},
		},
	}

	err := r.actionQuickChangeAccount(context.Background(), detail)
	if err == nil {
		t.Fatal("expected error from GetAccountByName, got nil")
	}
	if !errors.Is(err, errNotFound) {
		t.Fatalf("expected errNotFound, got: %v", err)
	}
}

// accountProviderSecondCallFails returns success on the first GetAccountByName call
// and an error on the second call.
type accountProviderSecondCallFails struct {
	first     *model.Account
	err       error
	callCount int
	allResult []*model.Account
}

func (a *accountProviderSecondCallFails) GetAccountByName(_ context.Context, _ string) (*model.Account, error) {
	a.callCount++
	if a.callCount == 1 {
		return a.first, nil
	}
	return nil, a.err
}

func (a *accountProviderSecondCallFails) GetAllAccounts(_ context.Context) ([]*model.Account, error) {
	return a.allResult, nil
}
```

- [ ] **Step 2: Run test to verify it fails (panics)**

Run: `go test ./cmd/transaction/ -run TestActionQuickChangeAccount_GetAccountByNameError -v`
Expected: PANIC with nil pointer dereference at line 92

### Task 2: Add tests for actionAddSplit and actionEditSplit

**Files:**
- Modify: `cmd/transaction/edit_actions_test.go`

- [ ] **Step 1: Add test for actionAddSplit GetAccountByName error (line 182)**

Append to `edit_actions_test.go`:

```go
func TestActionAddSplit_GetAccountByNameError(t *testing.T) {
	r := &editRunner{
		txSvc: &stubEditProvider{},
		accSvc: &stubAccountProvider{
			getAllResult:    []*model.Account{{ID: 1, Name: "Assets:Bank"}},
			getByNameErr:   errNotFound,
			getByNameResult: nil,
		},
		view: &stubEditView{
			accountFromListName: "Assets:Deleted",
			amountResult:        100,
			inputResult:         "",
		},
	}

	detail := &model.TransactionDetail{
		Splits: []model.SplitDetail{
			{AccountID: 1, AccountName: "Assets:Bank", Amount: -100},
			{AccountID: 2, AccountName: "Expenses:Food", Amount: 100},
		},
	}

	err := r.actionAddSplit(context.Background(), detail)
	if err == nil {
		t.Fatal("expected error from GetAccountByName, got nil")
	}
	if !errors.Is(err, errNotFound) {
		t.Fatalf("expected errNotFound, got: %v", err)
	}
}

func TestActionEditSplit_GetAccountByNameError(t *testing.T) {
	r := &editRunner{
		txSvc: &stubEditProvider{},
		accSvc: &stubAccountProvider{
			getAllResult:    []*model.Account{{ID: 1, Name: "Assets:Bank"}},
			getByNameErr:   errNotFound,
			getByNameResult: nil,
		},
		view: &stubEditView{
			splitSelectionResult: 0,
			accountFromListName:  "Assets:Deleted",
			amountResult:         100,
			inputResult:          "",
		},
	}

	detail := &model.TransactionDetail{
		Splits: []model.SplitDetail{
			{AccountID: 1, AccountName: "Assets:Bank", Amount: -100},
			{AccountID: 2, AccountName: "Expenses:Food", Amount: 100},
		},
	}

	err := r.actionEditSplit(context.Background(), detail)
	if err == nil {
		t.Fatal("expected error from GetAccountByName, got nil")
	}
	if !errors.Is(err, errNotFound) {
		t.Fatalf("expected errNotFound, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail (panic)**

Run: `go test ./cmd/transaction/ -run "TestActionAddSplit_GetAccountByNameError|TestActionEditSplit_GetAccountByNameError" -v`
Expected: PANIC with nil pointer dereference at lines 183 and 222

### Task 3: Fix the three GetAccountByName error sites

**Files:**
- Modify: `cmd/transaction/edit_actions.go:89`, `cmd/transaction/edit_actions.go:182`, `cmd/transaction/edit_actions.go:221`

- [ ] **Step 1: Fix line 89 in actionQuickChangeAccount**

Replace:
```go
	newAcc, _ := r.accSvc.GetAccountByName(ctx, newAccName)
```

With:
```go
	newAcc, err := r.accSvc.GetAccountByName(ctx, newAccName)
	if err != nil {
		return fmt.Errorf("account %q not found: %w", newAccName, err)
	}
```

- [ ] **Step 2: Fix line 182 in actionAddSplit**

Replace:
```go
	acc, _ := r.accSvc.GetAccountByName(ctx, accName)
```

With:
```go
	acc, err := r.accSvc.GetAccountByName(ctx, accName)
	if err != nil {
		return fmt.Errorf("account %q not found: %w", accName, err)
	}
```

- [ ] **Step 3: Fix line 221 in actionEditSplit**

Replace:
```go
	acc, _ := r.accSvc.GetAccountByName(ctx, newAccName)
```

With:
```go
	acc, err := r.accSvc.GetAccountByName(ctx, newAccName)
	if err != nil {
		return fmt.Errorf("account %q not found: %w", newAccName, err)
	}
```

- [ ] **Step 4: Run all three tests to verify they pass**

Run: `go test ./cmd/transaction/ -run "TestAction.*GetAccountByNameError" -v`
Expected: All 3 tests PASS

- [ ] **Step 5: Run full test suite to check for regressions**

Run: `go test ./...`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/transaction/edit_actions.go cmd/transaction/edit_actions_test.go
git commit -m "fix: check GetAccountByName errors in transaction edit actions

Closes #120"
```
