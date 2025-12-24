package service

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
)

func (as *AccountService) CreateAccount(name, accType, currency, description string, parentID *int64) (*model.Account, error) {
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

func (s *Service) CreateAccountWithBalance(name, accType, currency, description string, parentID *int64, balance int64) (*model.Account, error) {
	account, err := s.Account.CreateAccount(name, accType, currency, description, parentID)
	if err != nil {
		return nil, err
	}

	if balance != 0 {
		if err := s.Transaction.CreateOpeningBalance(account, balance); err != nil {
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
