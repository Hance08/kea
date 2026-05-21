// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

const maxParentDepth = 100

func (as *AccountService) validateParentChain(ctx context.Context, accountID int64, parentID *int64) error {
	if parentID == nil {
		return nil
	}

	visited := make(map[int64]bool)
	if accountID != 0 {
		visited[accountID] = true
	}
	currentID := *parentID

	for range maxParentDepth {
		if visited[currentID] {
			return fmt.Errorf("parent chain contains a cycle at account %d: %w", currentID, ErrCircularParent)
		}
		visited[currentID] = true

		acc, err := as.repo.GetAccountByID(ctx, currentID)
		if err != nil {
			return fmt.Errorf("failed to look up parent account %d: %w", currentID, err)
		}
		if acc.ParentID == nil {
			return nil
		}
		currentID = *acc.ParentID
	}

	return fmt.Errorf("parent chain exceeds maximum depth (%d): %w", maxParentDepth, ErrCircularParent)
}

func (as *AccountService) CreateAccount(ctx context.Context, input model.CreateAccountInput) (*model.Account, error) {
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if err := as.validateAccountFields(ctx, input.Name, input.Type, input.Currency, input.ParentID); err != nil {
		return nil, err
	}
	if err := as.validateParentType(ctx, input.ParentID, input.Type); err != nil {
		return nil, err
	}
	return as.createAccountViaRepo(ctx, as.repo, input)
}

func (as *AccountService) CreateAccountWithBalance(ctx context.Context, input model.CreateAccountInput) (*model.Account, error) {
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if err := as.validateAccountFields(ctx, input.Name, input.Type, input.Currency, input.ParentID); err != nil {
		return nil, err
	}
	if err := as.validateParentType(ctx, input.ParentID, input.Type); err != nil {
		return nil, err
	}

	if input.Balance == 0 {
		return as.createAccountViaRepo(ctx, as.repo, input)
	}

	var account *model.Account
	err := as.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, createErr := as.createAccountViaRepo(ctx, repo, input)
		if createErr != nil {
			return createErr
		}
		account = acc
		return as.createOpeningBalanceInRepo(ctx, repo, account, input.Balance)
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (as *AccountService) validateParentType(ctx context.Context, parentID *int64, accType model.AccountType) error {
	if parentID == nil {
		return nil
	}
	parent, err := as.repo.GetAccountByID(ctx, *parentID)
	if err != nil {
		return fmt.Errorf("failed to look up parent account %d: %w", *parentID, err)
	}
	if parent.Type != accType {
		return validationErrorf("type", "child type %q must match parent type %q", accType, parent.Type)
	}
	return nil
}

func (as *AccountService) validateAccountFields(ctx context.Context, name string, accType model.AccountType, currency string, parentID *int64) error {
	if err := as.ValidateFullAccountName(name); err != nil {
		return validationWrap("name", "invalid account name", err)
	}
	if err := as.ValidateCurrency(currency); err != nil {
		return validationWrap("currency", "invalid currency", err)
	}
	if !accType.IsValid() {
		return validationErrorf("type", "invalid account type: %s", accType)
	}
	if parentID != nil {
		if err := as.validateParentChain(ctx, 0, parentID); err != nil {
			return err
		}
		parent, err := as.repo.GetAccountByID(ctx, *parentID)
		if err != nil {
			return fmt.Errorf("failed to look up parent account %d: %w", *parentID, err)
		}
		hasTransactions, err := as.repo.AccountHasTransactions(ctx, parent.ID)
		if err != nil {
			return fmt.Errorf("failed to check parent transactions for account %d: %w", parent.ID, err)
		}
		if hasTransactions {
			return validationErrorf("parent", "account %q already has transactions; it cannot become a parent", parent.Name)
		}
		expectedPrefix := parent.Name + ":"
		if !strings.HasPrefix(name, expectedPrefix) {
			return validationErrorf("parent", "account name %q is not a child of parent %q", name, parent.Name)
		}
		childSegment := strings.TrimPrefix(name, expectedPrefix)
		if strings.Contains(childSegment, ":") {
			return validationErrorf("parent", "account name %q is not a direct child of parent %q (nested segments found)", name, parent.Name)
		}
	}
	root := strings.SplitN(name, ":", 2)[0]
	if expected, ok := model.AccountTypeFromRootName(root); ok && expected != accType {
		return validationErrorf("type", "account type %q conflicts with root %q (expected %q)", accType, root, expected)
	}
	return nil
}

func (as *AccountService) createAccountViaRepo(ctx context.Context, repo repository.AccountRepository, input model.CreateAccountInput) (*model.Account, error) {
	newID, err := repo.CreateAccount(ctx, input.Name, input.Type, input.Currency, input.Description, input.ParentID)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, fmt.Errorf("account %q: %w", input.Name, ErrAlreadyExists)
		}
		return nil, err
	}
	return &model.Account{
		ID:          newID,
		Name:        input.Name,
		Type:        input.Type,
		Currency:    input.Currency,
		Description: input.Description,
		ParentID:    input.ParentID,
		IsHidden:    false,
	}, nil
}

func (as *AccountService) createOpeningBalanceInRepo(ctx context.Context, repo repository.Repository, account *model.Account, amountInCents int64) error {
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
		return validationErrorf("type", "only Assets(A) and Liabilities(L) accounts can set an opening balance")
	}

	tx := model.Transaction{
		Timestamp:   time.Now().Unix(),
		Description: model.OpeningAccountMemo,
		Status:      model.StatusCleared,
		Type:        model.TxTypeOpening,
	}

	equityAcc, err := repo.GetAccountByName(ctx, equityAccountName)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("failed to look up %q: %w", equityAccountName, err)
		}
		newID, createErr := repo.CreateAccount(
			ctx,
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
	_, err = repo.CreateTransactionWithSplits(ctx, tx, splits)
	return err
}

func (as *AccountService) DeleteAccountByName(ctx context.Context, name string) error {
	acc, err := as.repo.GetAccountByName(ctx, name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("account %q: %w", name, ErrNotFound)
		}
		return err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return fmt.Errorf("account %q is a system account and cannot be deleted: %w", acc.Name, ErrNotEditable)
	}

	hasChildren, err := as.repo.HasChildAccounts(ctx, acc.ID)
	if err != nil {
		return err
	}
	if hasChildren {
		return validationErrorf("", "account %q has child accounts; delete or move them first", acc.Name)
	}

	hasTransactions, err := as.repo.AccountHasTransactions(ctx, acc.ID)
	if err != nil {
		return err
	}
	if hasTransactions {
		return validationErrorf("", "account %q has transactions and cannot be deleted", acc.Name)
	}

	return as.repo.DeleteAccount(ctx, acc.ID)
}

func (as *AccountService) FormatAccountName(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + ":" + name
}

func (as *AccountService) RenameAccount(ctx context.Context, oldName, newFullName string) (*model.Account, error) {
	acc, err := as.repo.GetAccountByName(ctx, oldName)
	if err != nil {
		return nil, err
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return nil, fmt.Errorf("account %q is a system account and cannot be edited: %w", acc.Name, ErrNotEditable)
	}

	oldPrefix := ""
	if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
		oldPrefix = acc.Name[:idx+1]
	}

	newPrefix := ""
	segment := newFullName
	if idx := strings.LastIndex(newFullName, ":"); idx >= 0 {
		newPrefix = newFullName[:idx+1]
		segment = newFullName[idx+1:]
	}

	if newPrefix != oldPrefix {
		return nil, validationErrorf("name", "rename cannot change parent path")
	}

	if err := as.ValidateAccountName(segment); err != nil {
		return nil, validationWrap("name", "invalid account name", err)
	}

	exists, err := as.repo.AccountExists(ctx, newFullName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, validationErrorf("name", "account %q already exists", newFullName)
	}

	if err := as.repo.RenameAccount(ctx, acc.Name, newFullName); err != nil {
		return nil, err
	}

	return as.repo.GetAccountByName(ctx, newFullName)
}
