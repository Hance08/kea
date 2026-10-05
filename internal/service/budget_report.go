// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hance08/kea/internal/model"
)

// GenerateBudgetReport compares every budget active in month (YYYY-MM, local
// time; empty means the current month) with actual spending. Actual uses the
// same transaction scope as the expense report (Expense splits of
// Expense-typed transactions) but sums signed amounts, so refunds reduce it.
func (bs *BudgetService) GenerateBudgetReport(ctx context.Context, month string) (*model.BudgetReport, error) {
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	if err := ValidateBudgetMonth(month); err != nil {
		return nil, validationErrorf("month", "invalid month %q, expected YYYY-MM", month)
	}
	start, end, _, err := parseMonth(month)
	if err != nil {
		return nil, err
	}

	report := &model.BudgetReport{
		Month:       month,
		Rows:        []model.BudgetReportRow{},
		TotalBudget: map[string]int64{},
		TotalActual: map[string]int64{},
	}

	all, err := bs.budgetRepo.ListBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list budgets: %w", err)
	}
	active := activeBudgets(all, month)
	if len(active) == 0 {
		return report, nil
	}

	accounts, err := bs.accRepo.GetAllAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load accounts: %w", err)
	}
	accByID := make(map[int64]*model.Account, len(accounts))
	for _, a := range accounts {
		accByID[a.ID] = a
	}

	splitsByTx, err := bs.txRepo.GetSplitsWithAccountsByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load splits: %w", err)
	}
	txs, err := bs.txRepo.GetTransactionsByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load transactions: %w", err)
	}
	txByID := make(map[int64]*model.Transaction, len(txs))
	for _, tx := range txs {
		txByID[tx.ID] = tx
	}

	for _, b := range active {
		acc, ok := accByID[b.AccountID]
		if !ok {
			continue
		}
		row := model.BudgetReportRow{
			AccountID:        acc.ID,
			AccountName:      acc.Name,
			Currency:         bs.currencyOrDefault(acc.Currency),
			EffectiveMonth:   b.EffectiveMonth,
			Budget:           b.Amount,
			ExcludedAccounts: []string{},
		}
		excluded := map[string]struct{}{}
		for txID, details := range splitsByTx {
			tx, ok := txByID[txID]
			if !ok || tx.Type != model.TxTypeExpense {
				continue
			}
			regular := tx.Regular != nil && *tx.Regular
			for _, d := range details {
				if d.AccountType != model.AccountTypeExpense || !isSelfOrDescendant(d.AccountName, acc.Name) {
					continue
				}
				if bs.currencyOrDefault(d.Currency) != row.Currency {
					excluded[d.AccountName] = struct{}{}
					continue
				}
				row.Actual += d.Amount
				if regular {
					row.ActualRegular += d.Amount
				} else {
					row.ActualIrregular += d.Amount
				}
			}
		}
		for name := range excluded {
			row.ExcludedAccounts = append(row.ExcludedAccounts, name)
		}
		sort.Strings(row.ExcludedAccounts)
		row.Remaining = row.Budget - row.Actual
		report.Rows = append(report.Rows, row)
	}

	sort.Slice(report.Rows, func(i, j int) bool { return report.Rows[i].AccountName < report.Rows[j].AccountName })

	for _, row := range report.Rows {
		if hasBudgetedAncestor(row.AccountName, report.Rows) {
			continue
		}
		report.TotalBudget[row.Currency] += row.Budget
		report.TotalActual[row.Currency] += row.Actual
	}
	return report, nil
}

func (bs *BudgetService) currencyOrDefault(ccy string) string {
	if ccy == "" {
		return bs.config.Defaults.Currency
	}
	return ccy
}

// isSelfOrDescendant reports whether name is ancestor or below it in the
// account tree. "Expenses:FoodTruck" is not below "Expenses:Food".
func isSelfOrDescendant(name, ancestor string) bool {
	return name == ancestor || strings.HasPrefix(name, ancestor+":")
}

func hasBudgetedAncestor(name string, rows []model.BudgetReportRow) bool {
	for _, r := range rows {
		if r.AccountName != name && isSelfOrDescendant(name, r.AccountName) {
			return true
		}
	}
	return false
}
