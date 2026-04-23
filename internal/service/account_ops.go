package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/hance08/kea/internal/store"
)

func (as *AccountService) CreateAccount(name string, accType model.AccountType, currency, description string, parentID *int64) (*model.Account, error) {
	if err := as.ValidateFullAccountName(name); err != nil {
		return nil, fmt.Errorf("invalid account name: %w", err)
	}
	if err := as.ValidateCurrency(currency); err != nil {
		return nil, fmt.Errorf("invalid currency: %w", err)
	}
	if !accType.IsValid() {
		return nil, fmt.Errorf("invalid account type: %s", accType)
	}

	newID, err := as.repo.CreateAccount(name, accType, currency, description, parentID)
	if err != nil {
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

func (as *AccountService) CreateAccountWithBalance(name string, accType model.AccountType, currency, description string, parentID *int64, balance int64) (*model.Account, error) {
	account, err := as.CreateAccount(name, accType, currency, description, parentID)
	if err != nil {
		return nil, err
	}

	if balance != 0 {
		if err := as.createOpeningBalance(account, balance); err != nil {
			return account, fmt.Errorf("account created but failed to set opening balance: %w", err)
		}
	}

	return account, nil
}

func (as *AccountService) createOpeningBalance(account *model.Account, amountInCents int64) error {
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

	return as.tm.ExecTx(context.Background(), func(repo repository.Repository) error {
		equityAcc, err := repo.GetAccountByName(equityAccountName)
		if err != nil {
			if !errors.Is(err, store.ErrRecordNotFound) {
				return fmt.Errorf("failed to look up %q: %w", equityAccountName, err)
			}
			// not found — create it
			newID, createErr := repo.CreateAccount(
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
		_, err = repo.CreateTransactionWithSplits(tx, splits)
		return err
	})
}

func (as *AccountService) DeleteAccountByName(name string) error {
	acc, err := as.repo.GetAccountByName(name)
	if err != nil {
		return err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return fmt.Errorf("account %q is a system account and cannot be deleted: %w", acc.Name, ErrNotEditable)
	}

	hasChildren, err := as.repo.HasChildAccounts(acc.ID)
	if err != nil {
		return err
	}
	if hasChildren {
		return fmt.Errorf("account %q has child accounts; delete or move them first", acc.Name)
	}

	hasTransactions, err := as.repo.AccountHasTransactions(acc.ID)
	if err != nil {
		return err
	}
	if hasTransactions {
		return fmt.Errorf("account %q has transactions and cannot be deleted", acc.Name)
	}

	return as.repo.DeleteAccount(acc.ID)
}

func (as *AccountService) FormatAccountName(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + ":" + name
}

func (as *AccountService) RenameAccount(oldName, newSegment string) error {
	acc, err := as.repo.GetAccountByName(oldName)
	if err != nil {
		return err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return fmt.Errorf("account %q is a system account and cannot be edited: %w", acc.Name, ErrNotEditable)
	}

	if err := as.ValidateAccountName(newSegment); err != nil {
		return fmt.Errorf("invalid account name: %w", err)
	}

	var newFullName string
	if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
		newFullName = acc.Name[:idx+1] + newSegment
	} else {
		newFullName = newSegment
	}

	exists, err := as.repo.AccountExists(newFullName)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("account %q already exists", newFullName)
	}

	return as.repo.RenameAccount(acc.Name, newFullName)
}
