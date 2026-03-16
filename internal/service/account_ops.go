package service

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
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
		if as.txSvc == nil {
			return account, fmt.Errorf("transaction service is not configured")
		}

		if err := as.txSvc.CreateOpeningBalance(account, balance); err != nil {
			return account, fmt.Errorf("account created but failed to set opening balance: %w", err)
		}
	}

	return account, nil
}

func (as *AccountService) DeleteAccountByName(name string) error {
	acc, err := as.repo.GetAccountByName(name)
	if err != nil {
		return err
	}

	if acc.Name == model.SystemAccountOpeningBalance {
		return fmt.Errorf("account %q is a system opening balance account and cannot be deleted", acc.Name)
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
