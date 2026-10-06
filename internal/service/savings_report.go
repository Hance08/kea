// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hance08/kea/internal/model"
)

// GenerateSavingsReport compares each savings target active in month (YYYY-MM,
// local time; empty means the current month) with what was saved, for the
// month and from January of the same year.
func (ss *SavingsService) GenerateSavingsReport(ctx context.Context, month string) (*model.SavingsReport, error) {
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	if err := ValidateBudgetMonth(month); err != nil {
		return nil, validationErrorf("month", "invalid month %q, expected YYYY-MM", month)
	}
	firstMonth := month[:4] + "-01"
	start, _, _, err := parseMonth(firstMonth)
	if err != nil {
		return nil, err
	}
	_, end, _, err := parseMonth(month)
	if err != nil {
		return nil, err
	}

	report := &model.SavingsReport{
		Month:          month,
		Rows:           []model.SavingsReportRow{},
		TotalTarget:    map[string]int64{},
		TotalSaved:     map[string]int64{},
		TotalYTDTarget: map[string]int64{},
		TotalYTDSaved:  map[string]int64{},
	}

	all, err := ss.savingsRepo.ListSavingsTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list savings targets: %w", err)
	}
	active := activeVersions(all, month, savingsVersionKey)
	if len(active) == 0 {
		return report, nil
	}

	accounts, err := ss.accRepo.GetAllAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load accounts: %w", err)
	}
	accByID := make(map[int64]*model.Account, len(accounts))
	for _, a := range accounts {
		accByID[a.ID] = a
	}

	splitsByTx, err := ss.txRepo.GetSplitsWithAccountsByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load splits: %w", err)
	}
	txs, err := ss.txRepo.GetTransactionsByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load transactions: %w", err)
	}
	// Local YYYY-MM of every counted transaction. Opening balances are
	// existing money, not money saved, so they never count.
	txMonth := make(map[int64]string, len(txs))
	for _, tx := range txs {
		if tx.Type == model.TxTypeOpening {
			continue
		}
		m := time.Unix(tx.Timestamp, 0).Format("2006-01")
		if m < firstMonth || m > month {
			continue
		}
		txMonth[tx.ID] = m
	}

	months := monthsFromJanuary(month)
	for _, t := range active {
		acc, ok := accByID[t.AccountID]
		if !ok {
			continue
		}
		row := model.SavingsReportRow{
			AccountID:        acc.ID,
			AccountName:      acc.Name,
			Currency:         ss.currencyOrDefault(acc.Currency),
			EffectiveMonth:   t.EffectiveMonth,
			Target:           t.Amount,
			Months:           []model.SavingsMonth{},
			ExcludedAccounts: []string{},
		}
		saved := map[string]int64{}
		excluded := map[string]struct{}{}
		for txID, details := range splitsByTx {
			m, ok := txMonth[txID]
			if !ok {
				continue
			}
			for _, d := range details {
				if !isSelfOrDescendant(d.AccountName, acc.Name) {
					continue
				}
				if ss.currencyOrDefault(d.Currency) != row.Currency {
					if m == month {
						excluded[d.AccountName] = struct{}{}
					}
					continue
				}
				saved[m] += d.Amount
			}
		}
		for name := range excluded {
			row.ExcludedAccounts = append(row.ExcludedAccounts, name)
		}
		sort.Strings(row.ExcludedAccounts)

		row.Saved = saved[month]
		row.Remaining = row.Target - row.Saved
		for _, m := range months {
			v, ok := activeVersionFor(all, acc.ID, m, savingsVersionKey)
			if !ok {
				continue
			}
			row.Months = append(row.Months, model.SavingsMonth{Month: m, Target: v.Amount, Saved: saved[m]})
			row.YTDTarget += v.Amount
			row.YTDSaved += saved[m]
		}
		row.YTDRemaining = row.YTDTarget - row.YTDSaved
		report.Rows = append(report.Rows, row)
	}

	sort.Slice(report.Rows, func(i, j int) bool { return report.Rows[i].AccountName < report.Rows[j].AccountName })

	nameCcy := func(r model.SavingsReportRow) (string, string) { return r.AccountName, r.Currency }
	for _, row := range report.Rows {
		if hasAncestorInCurrency(row, report.Rows, nameCcy) {
			continue
		}
		report.TotalTarget[row.Currency] += row.Target
		report.TotalSaved[row.Currency] += row.Saved
		report.TotalYTDTarget[row.Currency] += row.YTDTarget
		report.TotalYTDSaved[row.Currency] += row.YTDSaved
	}
	return report, nil
}

// monthsFromJanuary returns "YYYY-01" through month (a validated YYYY-MM).
func monthsFromJanuary(month string) []string {
	var n int
	_, _ = fmt.Sscanf(month[5:], "%d", &n)
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, fmt.Sprintf("%s-%02d", month[:4], i))
	}
	return out
}
