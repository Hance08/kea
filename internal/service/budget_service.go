// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

var budgetMonthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// BudgetService manages monthly budgets on Expense accounts and reports
// budget vs actual spending.
type BudgetService struct {
	budgetRepo repository.BudgetRepository
	accRepo    repository.AccountRepository
	txRepo     repository.TransactionRepository
	tm         repository.TransactionManager
	config     *config.Config
}

func NewBudgetService(
	budgetRepo repository.BudgetRepository,
	accRepo repository.AccountRepository,
	txRepo repository.TransactionRepository,
	tm repository.TransactionManager,
	cfg *config.Config,
) *BudgetService {
	return &BudgetService{budgetRepo: budgetRepo, accRepo: accRepo, txRepo: txRepo, tm: tm, config: cfg}
}

// ValidateBudgetMonth checks that month is YYYY-MM with a month of 01-12.
func ValidateBudgetMonth(month string) error {
	if !budgetMonthPattern.MatchString(month) {
		return validationErrorf("effective_month", "invalid month %q, expected YYYY-MM", month)
	}
	return nil
}

// SetBudget creates or replaces the budget version for an account starting at
// input.EffectiveMonth.
func (bs *BudgetService) SetBudget(ctx context.Context, input model.SetBudgetInput) (*model.Budget, error) {
	name := strings.TrimSpace(input.AccountName)
	if name == "" {
		return nil, validationErrorf("account_name", "account name is required")
	}
	if err := ValidateBudgetMonth(input.EffectiveMonth); err != nil {
		return nil, err
	}
	if input.Amount < 0 {
		return nil, validationErrorf("amount", "budget amount must not be negative")
	}

	var out *model.Budget
	err := bs.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, err := budgetAccount(ctx, repo, name)
		if err != nil {
			return err
		}
		id, err := repo.UpsertBudget(ctx, acc.ID, input.EffectiveMonth, input.Amount, false)
		if err != nil {
			return fmt.Errorf("failed to save budget: %w", err)
		}
		out = &model.Budget{ID: id, AccountID: acc.ID, AccountName: acc.Name, EffectiveMonth: input.EffectiveMonth, Amount: input.Amount}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StopBudget ends an account's active budget from input.EffectiveMonth on.
func (bs *BudgetService) StopBudget(ctx context.Context, input model.StopBudgetInput) (*model.Budget, error) {
	name := strings.TrimSpace(input.AccountName)
	if name == "" {
		return nil, validationErrorf("account_name", "account name is required")
	}
	if err := ValidateBudgetMonth(input.EffectiveMonth); err != nil {
		return nil, err
	}

	var out *model.Budget
	err := bs.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, err := budgetAccount(ctx, repo, name)
		if err != nil {
			return err
		}
		all, err := repo.ListBudgets(ctx)
		if err != nil {
			return fmt.Errorf("failed to list budgets: %w", err)
		}
		if !hasActiveBudget(all, acc.ID, input.EffectiveMonth) {
			return validationErrorf("effective_month", "account %q has no active budget in %s", acc.Name, input.EffectiveMonth)
		}
		id, err := repo.UpsertBudget(ctx, acc.ID, input.EffectiveMonth, 0, true)
		if err != nil {
			return fmt.Errorf("failed to stop budget: %w", err)
		}
		out = &model.Budget{ID: id, AccountID: acc.ID, AccountName: acc.Name, EffectiveMonth: input.EffectiveMonth, Stopped: true}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListBudgets returns every budget version, ordered by account name then month.
func (bs *BudgetService) ListBudgets(ctx context.Context) ([]model.Budget, error) {
	list, err := bs.budgetRepo.ListBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list budgets: %w", err)
	}
	if list == nil {
		list = []model.Budget{}
	}
	return list, nil
}

// DeleteBudget removes a single budget version.
func (bs *BudgetService) DeleteBudget(ctx context.Context, id int64) error {
	if err := bs.budgetRepo.DeleteBudget(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("budget %d: %w", id, ErrNotFound)
		}
		return fmt.Errorf("failed to delete budget: %w", err)
	}
	return nil
}

// budgetAccount resolves name to an Expense account.
func budgetAccount(ctx context.Context, repo repository.AccountRepository, name string) (*model.Account, error) {
	acc, err := repo.GetAccountByName(ctx, name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("account %q: %w", name, ErrNotFound)
		}
		return nil, fmt.Errorf("failed to look up account %q: %w", name, err)
	}
	if acc.Type != model.AccountTypeExpense {
		return nil, validationErrorf("account_name", "budgets can only be set on Expense accounts; %q is type %s", acc.Name, acc.Type)
	}
	return acc, nil
}

// activeBudgets returns the non-stopped version that applies to month for
// each account, in the input order.
func activeBudgets(budgets []model.Budget, month string) []model.Budget {
	return activeVersions(budgets, month, budgetVersionKey)
}

func hasActiveBudget(budgets []model.Budget, accountID int64, month string) bool {
	_, ok := activeVersionFor(budgets, accountID, month, budgetVersionKey)
	return ok
}
