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

type ListAccountsOptions struct {
	Type       *model.AccountType
	ShowHidden bool
}

func (as *AccountService) ListAccounts(ctx context.Context, opts ListAccountsOptions) ([]*model.Account, error) {
	var accounts []*model.Account
	var err error

	if opts.Type != nil {
		accounts, err = as.repo.GetAccountsByType(ctx, *opts.Type)
	} else {
		accounts, err = as.repo.GetAllAccounts(ctx)
	}
	if err != nil {
		return nil, err
	}

	if !opts.ShowHidden {
		filtered := make([]*model.Account, 0, len(accounts))
		for _, acc := range accounts {
			if !acc.IsHidden {
				filtered = append(filtered, acc)
			}
		}
		accounts = filtered
	}

	return accounts, nil
}

func (as *AccountService) GetAllAccounts(ctx context.Context) ([]*model.Account, error) {
	return as.repo.GetAllAccounts(ctx)
}

func (as *AccountService) GetAccountByName(ctx context.Context, name string) (*model.Account, error) {
	acc, err := as.repo.GetAccountByName(ctx, name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("account %q: %w", name, ErrNotFound)
		}
		return nil, err
	}
	return acc, nil
}

func (as *AccountService) GetAccountByID(ctx context.Context, id int64) (*model.Account, error) {
	acc, err := as.repo.GetAccountByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("account #%d: %w", id, ErrNotFound)
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
		return "", validationErrorf("type", "invalid account type '%s' (must be A, L, C, R, E)", accType)
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
			return validationErrorf("type", "account %q has type %q, not allowed for this transaction type (allowed: %v)", name, acc.Type, allowedTypes)
		}
	}

	if acc.IsHidden {
		return validationErrorf("account", "account %q is hidden", name)
	}

	hasChildren, err := as.repo.HasChildAccounts(ctx, acc.ID)
	if err != nil {
		return err
	}
	if hasChildren {
		return validationErrorf("account", "account %q is a parent account; select a leaf account instead", name)
	}

	return nil
}

func (as *AccountService) SearchAccounts(ctx context.Context, filter model.AccountFilter, opts model.ListOptions) (*model.ListResult[*model.Account], error) {
	return as.repo.SearchAccounts(ctx, filter, opts)
}

func (as *AccountService) UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) (*model.Account, error) {
	acc, err := as.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return nil, fmt.Errorf("account %q is a system account and cannot be edited: %w", acc.Name, ErrNotEditable)
	}

	if err := as.repo.UpdateAccountMetadata(ctx, accountID, description, isHidden); err != nil {
		return nil, err
	}

	return as.repo.GetAccountByID(ctx, accountID)
}
