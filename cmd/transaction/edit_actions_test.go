// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package transaction

import (
	"context"
	"errors"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
)

// --- stub: EditView ---

type stubEditView struct{}

func (s *stubEditView) ShowError(msg string, err error)                           {}
func (s *stubEditView) ShowWarning(msg string)                                    {}
func (s *stubEditView) ShowSuccess(msg string)                                    {}
func (s *stubEditView) ShowInfo(msg string)                                       {}
func (s *stubEditView) RenderDetail(detail *model.TransactionDetail) error        { return nil }
func (s *stubEditView) RenderSplitsPreview(splits []model.SplitDetail)            {}
func (s *stubEditView) AskSelection(label string, options []string) (string, error) {
	return "", nil
}
func (s *stubEditView) AskInput(label, defaultVal string) (string, error)  { return defaultVal, nil }
func (s *stubEditView) AskConfirm(label string) bool                       { return false }
func (s *stubEditView) AskDescription(current string) (string, error)      { return current, nil }
func (s *stubEditView) AskDate(currentTimestamp int64) (int64, error)      { return currentTimestamp, nil }
func (s *stubEditView) AskStatus(current model.TransactionStatus) (model.TransactionStatus, error) {
	return current, nil
}
func (s *stubEditView) AskAccountFromList(accounts []*model.Account, defaultName string) (string, error) {
	// Return the first account name in the list that is different from defaultName, or defaultName.
	for _, a := range accounts {
		if a.Name != defaultName {
			return a.Name, nil
		}
	}
	return defaultName, nil
}
func (s *stubEditView) AskAmount(label string, currentCents int64, allowEmpty bool) (int64, error) {
	return currentCents, nil
}
func (s *stubEditView) AskSplitSelection(splits []model.SplitDetail) (int, error) { return 0, nil }

// --- stub: EditProvider ---

type stubEditProvider struct{}

func (s *stubEditProvider) GetTransactionByID(ctx context.Context, txID int64) (*model.TransactionDetail, error) {
	return nil, nil
}
func (s *stubEditProvider) IsEditable(detail *model.TransactionDetail) (bool, service.NotEditableReason) {
	return true, 0
}
func (s *stubEditProvider) GetAllowedAccounts(txType model.TransactionType, currentAccountType model.AccountType, allAccounts []*model.Account) []*model.Account {
	return allAccounts
}
func (s *stubEditProvider) ValidateTransactionEdit(ctx context.Context, splits []model.SplitDetail) error {
	return nil
}
func (s *stubEditProvider) ValidateSplitsMatchType(ctx context.Context, txType model.TransactionType, splits []model.SplitDetail) error {
	return nil
}
func (s *stubEditProvider) UpdateTransactionComplete(ctx context.Context, input model.UpdateTransactionInput) error {
	return nil
}

// --- stub: AccountProvider (second GetAccountByName call fails) ---

// accountProviderSecondCallFails succeeds on the first GetAccountByName call
// (used for the currentAcc lookup) but returns an error on the second call
// (used for the newAcc lookup). This exercises the bug at line 89 of
// edit_actions.go where the error is discarded with `_, _`.
type accountProviderSecondCallFails struct {
	callCount int
	accounts  map[string]*model.Account
}

func (p *accountProviderSecondCallFails) GetAccountByName(ctx context.Context, name string) (*model.Account, error) {
	p.callCount++
	if p.callCount == 1 {
		if acc, ok := p.accounts[name]; ok {
			return acc, nil
		}
		return nil, errors.New("account not found: " + name)
	}
	// Second call always fails.
	return nil, errors.New("simulated lookup failure on second call")
}

func (p *accountProviderSecondCallFails) GetAllAccounts(ctx context.Context) ([]*model.Account, error) {
	result := make([]*model.Account, 0, len(p.accounts))
	for _, a := range p.accounts {
		result = append(result, a)
	}
	return result, nil
}

// --- test ---

// TestActionQuickChangeAccount_GetAccountByNameError verifies that when the
// second GetAccountByName call (line 89 of edit_actions.go) returns an error,
// actionQuickChangeAccount propagates that error instead of panicking with a
// nil-pointer dereference.
//
// Currently the bug exists: line 89 uses `newAcc, _ := ...` which discards the
// error and then immediately dereferences the nil pointer at line 92, causing a
// panic. This test is expected to PANIC until the bug is fixed.
func TestActionQuickChangeAccount_GetAccountByNameError(t *testing.T) {
	currentAcc := &model.Account{
		ID:       1,
		Name:     "Expenses:Food",
		Type:     model.AccountTypeExpense,
		Currency: "USD",
	}
	alternateAcc := &model.Account{
		ID:       2,
		Name:     "Expenses:Travel",
		Type:     model.AccountTypeExpense,
		Currency: "USD",
	}

	accSvc := &accountProviderSecondCallFails{
		accounts: map[string]*model.Account{
			currentAcc.Name:   currentAcc,
			alternateAcc.Name: alternateAcc,
		},
	}

	// stubEditView.AskAccountFromList returns the first account that differs
	// from the defaultName, so it will return alternateAcc.Name — triggering
	// the second GetAccountByName call which will fail.
	view := &stubEditView{}

	runner := &editRunner{
		txSvc:  &stubEditProvider{},
		accSvc: accSvc,
		view:   view,
		txID:   42,
	}

	detail := &model.TransactionDetail{
		ID:   42,
		Type: model.TxTypeExpense,
		Splits: []model.SplitDetail{
			{AccountID: 1, AccountName: "Expenses:Food", Currency: "USD", Amount: 1000},
			{AccountID: 3, AccountName: "Assets:Checking", Currency: "USD", Amount: -1000},
		},
	}

	err := runner.actionQuickChangeAccount(context.Background(), detail)
	if err == nil {
		t.Fatal("expected an error from the failed GetAccountByName call, got nil")
	}
}
