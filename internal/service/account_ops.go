package service

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
)

func (as *AccountService) CreateAccount(name string, accType model.AccountType, currency, description string, parentID *int64) (*model.Account, error) {
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
		if err := as.txSvc.CreateOpeningBalance(account, balance); err != nil {
			return account, fmt.Errorf("account created but failed to set opening balance: %w", err)
		}
	}

	return account, nil
}

func (as *AccountService) FormatAccountName(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + ":" + name
}
