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
)

// SavingsService manages monthly savings targets on Asset accounts and
// reports target vs saved.
type SavingsService struct {
	savingsRepo repository.SavingsTargetRepository
	accRepo     repository.AccountRepository
	txRepo      repository.TransactionRepository
	tm          repository.TransactionManager
	config      *config.Config
}

func NewSavingsService(
	savingsRepo repository.SavingsTargetRepository,
	accRepo repository.AccountRepository,
	txRepo repository.TransactionRepository,
	tm repository.TransactionManager,
	cfg *config.Config,
) *SavingsService {
	return &SavingsService{savingsRepo: savingsRepo, accRepo: accRepo, txRepo: txRepo, tm: tm, config: cfg}
}

// SetSavingsTarget creates or replaces the target version for an account
// starting at input.EffectiveMonth.
func (ss *SavingsService) SetSavingsTarget(ctx context.Context, input model.SetSavingsTargetInput) (*model.SavingsTarget, error) {
	name := strings.TrimSpace(input.AccountName)
	if name == "" {
		return nil, validationErrorf("account_name", "account name is required")
	}
	if err := ValidateBudgetMonth(input.EffectiveMonth); err != nil {
		return nil, err
	}
	if input.Amount < 0 {
		return nil, validationErrorf("amount", "savings target must not be negative")
	}

	var out *model.SavingsTarget
	err := ss.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, err := savingsAccount(ctx, repo, name)
		if err != nil {
			return err
		}
		id, err := repo.UpsertSavingsTarget(ctx, acc.ID, input.EffectiveMonth, input.Amount, false)
		if err != nil {
			return fmt.Errorf("failed to save savings target: %w", err)
		}
		out = &model.SavingsTarget{ID: id, AccountID: acc.ID, AccountName: acc.Name, EffectiveMonth: input.EffectiveMonth, Amount: input.Amount}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StopSavingsTarget ends an account's target from input.EffectiveMonth on.
func (ss *SavingsService) StopSavingsTarget(ctx context.Context, input model.StopSavingsTargetInput) (*model.SavingsTarget, error) {
	name := strings.TrimSpace(input.AccountName)
	if name == "" {
		return nil, validationErrorf("account_name", "account name is required")
	}
	if err := ValidateBudgetMonth(input.EffectiveMonth); err != nil {
		return nil, err
	}

	var out *model.SavingsTarget
	err := ss.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, err := savingsAccount(ctx, repo, name)
		if err != nil {
			return err
		}
		all, err := repo.ListSavingsTargets(ctx)
		if err != nil {
			return fmt.Errorf("failed to list savings targets: %w", err)
		}
		if _, ok := activeVersionFor(all, acc.ID, input.EffectiveMonth, savingsVersionKey); !ok {
			return validationErrorf("effective_month", "account %q has no active savings target in %s", acc.Name, input.EffectiveMonth)
		}
		id, err := repo.UpsertSavingsTarget(ctx, acc.ID, input.EffectiveMonth, 0, true)
		if err != nil {
			return fmt.Errorf("failed to stop savings target: %w", err)
		}
		out = &model.SavingsTarget{ID: id, AccountID: acc.ID, AccountName: acc.Name, EffectiveMonth: input.EffectiveMonth, Stopped: true}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListSavingsTargets returns every target version, ordered by account name then month.
func (ss *SavingsService) ListSavingsTargets(ctx context.Context) ([]model.SavingsTarget, error) {
	list, err := ss.savingsRepo.ListSavingsTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list savings targets: %w", err)
	}
	if list == nil {
		list = []model.SavingsTarget{}
	}
	return list, nil
}

// DeleteSavingsTarget removes a single target version.
func (ss *SavingsService) DeleteSavingsTarget(ctx context.Context, id int64) error {
	if err := ss.savingsRepo.DeleteSavingsTarget(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("savings target %d: %w", id, ErrNotFound)
		}
		return fmt.Errorf("failed to delete savings target: %w", err)
	}
	return nil
}

func (ss *SavingsService) currencyOrDefault(ccy string) string {
	if ccy == "" {
		return ss.config.Defaults.Currency
	}
	return ccy
}

// savingsAccount resolves name to an Asset account.
func savingsAccount(ctx context.Context, repo repository.AccountRepository, name string) (*model.Account, error) {
	acc, err := repo.GetAccountByName(ctx, name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("account %q: %w", name, ErrNotFound)
		}
		return nil, fmt.Errorf("failed to look up account %q: %w", name, err)
	}
	if acc.Type != model.AccountTypeAsset {
		return nil, validationErrorf("account_name", "savings targets can only be set on Asset accounts; %q is type %s", acc.Name, acc.Type)
	}
	return acc, nil
}
