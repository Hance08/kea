// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/internal/utils"
)

type AccountService struct {
	repo   repository.AccountRepository
	config *config.Config
	tm     repository.TransactionManager
}

func NewAccountService(repo repository.AccountRepository, cfg *config.Config, tm repository.TransactionManager) *AccountService {
	return &AccountService{repo: repo, config: cfg, tm: tm}
}

func (as *AccountService) GetAllAccounts(ctx context.Context) ([]*model.Account, error) {
	return as.repo.GetAllAccounts(ctx)
}

func (as *AccountService) GetAccountByName(ctx context.Context, name string) (*model.Account, error) {
	acc, err := as.repo.GetAccountByName(ctx, name)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return nil, fmt.Errorf("account %q: %w", name, ErrNotFound)
		}
		return nil, err
	}
	return acc, nil
}

func (as *AccountService) GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error) {
	return as.repo.GetAccountsByType(ctx, accType)
}

func (as *AccountService) GetAccountBalance(ctx context.Context, accountID int64) (int64, error) {
	return as.repo.GetAccountBalance(ctx, accountID)
}

func (as *AccountService) GetAccountBalanceFormatted(ctx context.Context, accountID int64) (string, error) {
	balance, err := as.repo.GetAccountBalance(ctx, accountID)
	if err != nil {
		return "", err
	}
	return utils.FormatAmount(balance), nil
}

func (as *AccountService) GetRootNameByType(accType string) (string, error) {
	at := model.AccountType(strings.ToUpper(accType))
	name, ok := at.RootName()
	if !ok {
		return "", fmt.Errorf("invalid account type '%s' (must be A, L, C, R, E)", accType)
	}
	return name, nil
}

func (as *AccountService) CheckAccountExists(ctx context.Context, name string) (bool, error) {
	return as.repo.AccountExists(ctx, name)
}

func (as *AccountService) ValidateSelectableAccount(ctx context.Context, name string, allowedTypes []string) error {
	acc, err := as.repo.GetAccountByName(ctx, name)
	if err != nil {
		return err
	}

	if len(allowedTypes) > 0 {
		allowed := false
		for _, t := range allowedTypes {
			if string(acc.Type) == t {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("account %q has type %q, not allowed for this transaction type (allowed: %v)", name, acc.Type, allowedTypes)
		}
	}

	if acc.IsHidden {
		return fmt.Errorf("account %q is hidden", name)
	}

	hasChildren, err := as.repo.HasChildAccounts(ctx, acc.ID)
	if err != nil {
		return err
	}
	if hasChildren {
		return fmt.Errorf("account %q is a parent account; select a leaf account instead", name)
	}

	return nil
}

func (as *AccountService) UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) error {
	acc, err := as.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return fmt.Errorf("account %q is a system account and cannot be edited: %w", acc.Name, ErrNotEditable)
	}

	return as.repo.UpdateAccountMetadata(ctx, accountID, description, isHidden)
}
