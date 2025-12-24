package service

import (
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
