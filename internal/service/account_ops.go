package service

import (
	"github.com/hance08/kea/internal/store"
)

func (as *AccountService) CreateAccount(name, accType, currency, description string, parentID *int64) (*store.Account, error) {
	newID, err := as.repo.CreateAccount(name, accType, currency, description, parentID)
	if err != nil {
		return nil, err
	}

	return &store.Account{
		ID:          newID,
		Name:        name,
		Type:        accType,
		Currency:    currency,
		Description: description,
		ParentID:    parentID,
		IsHidden:    false,
	}, nil
}
