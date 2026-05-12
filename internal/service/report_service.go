// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"sort"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

// GetNetWorthAt returns net worth (assets + liabilities) per currency using all posted splits up to endTime.
// Liability splits are stored as negative values (credit-normal accounts), so adding them to assets
// correctly subtracts the outstanding balance.
func (ts *TransactionService) GetNetWorthAt(ctx context.Context, endTime int64) (map[string]int64, error) {
	txSplitsMap, err := ts.txRepo.GetSplitsWithAccountsByDateRange(ctx, 0, endTime)
	if err != nil {
		return nil, err
	}

	assets := map[string]int64{}
	liabilities := map[string]int64{}
	for _, splits := range txSplitsMap {
		for _, split := range splits {
			switch split.AccountType {
			case model.AccountTypeAsset:
				assets[split.Currency] += split.Amount
			case model.AccountTypeLiability:
				liabilities[split.Currency] += split.Amount
			}
		}
	}

	nw := map[string]int64{}
	for ccy, amt := range assets {
		nw[ccy] = amt + liabilities[ccy]
	}
	for ccy, amt := range liabilities {
		if _, ok := nw[ccy]; !ok {
			nw[ccy] = amt
		}
	}
	return nw, nil
}

// offsetAccountName inspects the splits of a transaction and returns the name of the
// "other side" account relative to the given primary account type filter.
// If exactly one offset account exists it returns its name; if multiple exist it
// returns "(multiple)"; if none exist it returns an empty string.
func offsetAccountName(details []model.SplitDetail, primaryType model.AccountType) string {
	seen := map[string]struct{}{}
	for _, d := range details {
		if d.AccountType != primaryType {
			seen[d.AccountName] = struct{}{}
		}
	}
	switch len(seen) {
	case 0:
		return ""
	case 1:
		for name := range seen {
			return name
		}
	}
	return "(multiple)"
}

// buildReportMaps fetches all splits in the date range and aggregates them into
// income/expense row maps. Pass includeIncome=false to skip income classification
// (and vice versa) to avoid unnecessary work for breakdown-only queries.
func (ts *TransactionService) buildReportMaps(ctx context.Context, startTime, endTime int64, includeIncome, includeExpense bool) (incomeByAccount, expenseByAccount map[string]*model.ReportRow, err error) {
	txSplitsMap, err := ts.txRepo.GetSplitsWithAccountsByDateRange(ctx, startTime, endTime)
	if err != nil {
		return nil, nil, err
	}

	txs, err := ts.txRepo.GetTransactionsByDateRange(ctx, startTime, endTime)
	if err != nil {
		return nil, nil, err
	}
	txTypeMap := make(map[int64]model.TransactionType, len(txs))
	for _, tx := range txs {
		txTypeMap[tx.ID] = tx.Type
	}

	incomeByAccount = map[string]*model.ReportRow{}
	expenseByAccount = map[string]*model.ReportRow{}

	for txID, details := range txSplitsMap {
		txType := txTypeMap[txID]

		if includeIncome && txType == model.TxTypeIncome {
			offset := offsetAccountName(details, model.AccountTypeRevenue)
			for _, split := range details {
				if split.AccountType == model.AccountTypeRevenue {
					key := split.AccountName + "|" + offset + "|" + split.Currency
					row := getOrCreateRowWithOffset(incomeByAccount, key, split.AccountName, offset, split.Currency)
					row.Amount += utils.AbsInt64(split.Amount)
					row.TxCount++
				}
			}
		}

		if includeExpense && txType == model.TxTypeExpense {
			offset := offsetAccountName(details, model.AccountTypeExpense)
			for _, split := range details {
				if split.AccountType == model.AccountTypeExpense {
					key := split.AccountName + "|" + offset + "|" + split.Currency
					row := getOrCreateRowWithOffset(expenseByAccount, key, split.AccountName, offset, split.Currency)
					row.Amount += utils.AbsInt64(split.Amount)
					row.TxCount++
				}
			}
		}
	}

	return incomeByAccount, expenseByAccount, nil
}

// GenerateIncomeStatement produces an income/expense summary for the given Unix time range.
func (ts *TransactionService) GenerateIncomeStatement(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
	incomeByAccount, expenseByAccount, err := ts.buildReportMaps(ctx, startTime, endTime, true, true)
	if err != nil {
		return nil, err
	}

	result := &model.ReportResult{
		IncomeRows:  rowsFromMap(incomeByAccount),
		ExpenseRows: rowsFromMap(expenseByAccount),
		TotalIncome:  map[string]int64{},
		TotalExpense: map[string]int64{},
		NetAmount:    map[string]int64{},
	}
	for _, r := range result.IncomeRows {
		result.TotalIncome[r.Currency] += r.Amount
	}
	for _, r := range result.ExpenseRows {
		result.TotalExpense[r.Currency] += r.Amount
	}
	for ccy, inc := range result.TotalIncome {
		result.NetAmount[ccy] = inc - result.TotalExpense[ccy]
	}
	for ccy, exp := range result.TotalExpense {
		if _, ok := result.NetAmount[ccy]; !ok {
			result.NetAmount[ccy] = -exp
		}
	}

	return result, nil
}

// GenerateIncomeBreakdown produces a detailed income-only report for the given Unix time range.
func (ts *TransactionService) GenerateIncomeBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
	incomeByAccount, _, err := ts.buildReportMaps(ctx, startTime, endTime, true, false)
	if err != nil {
		return nil, err
	}

	rows := rowsFromMap(incomeByAccount)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Amount > rows[j].Amount })

	total := map[string]int64{}
	for _, r := range rows {
		total[r.Currency] += r.Amount
	}

	return &model.ReportResult{
		IncomeRows:  rows,
		TotalIncome: total,
	}, nil
}

// GenerateExpenseBreakdown produces a detailed expense-only report for the given Unix time range.
func (ts *TransactionService) GenerateExpenseBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
	_, expenseByAccount, err := ts.buildReportMaps(ctx, startTime, endTime, false, true)
	if err != nil {
		return nil, err
	}

	rows := rowsFromMap(expenseByAccount)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Amount > rows[j].Amount })

	total := map[string]int64{}
	for _, r := range rows {
		total[r.Currency] += r.Amount
	}

	return &model.ReportResult{
		ExpenseRows:  rows,
		TotalExpense: total,
	}, nil
}

// GenerateBalanceSheet produces a snapshot of all asset, liability, and equity account balances
// as of the given Unix timestamp.
func (ts *TransactionService) GenerateBalanceSheet(ctx context.Context, asOf int64) (*model.BalanceSheetResult, error) {
	allAccounts, err := ts.accRepo.GetAllAccounts(ctx)
	if err != nil {
		return nil, err
	}

	balances, err := ts.accRepo.GetAllAccountBalances(ctx, asOf)
	if err != nil {
		return nil, err
	}

	result := &model.BalanceSheetResult{
		AsOf:             asOf,
		TotalAssets:      map[string]int64{},
		TotalLiabilities: map[string]int64{},
		TotalEquity:      map[string]int64{},
		NetWorth:         map[string]int64{},
	}

	for _, acc := range allAccounts {
		balance := balances[acc.ID]
		if balance == 0 {
			continue
		}

		currency := acc.Currency
		if currency == "" {
			currency = ts.config.Defaults.Currency
		}

		row := model.ReportRow{
			AccountName: acc.Name,
			Amount:      balance,
			Currency:    currency,
		}

		switch acc.Type {
		case model.AccountTypeAsset:
			result.Assets = append(result.Assets, row)
			result.TotalAssets[currency] += balance
		case model.AccountTypeLiability:
			displayBalance := -balance
			row := model.ReportRow{
				AccountName: acc.Name,
				Amount:      displayBalance,
				Currency:    currency,
			}
			result.Liabilities = append(result.Liabilities, row)
			result.TotalLiabilities[currency] += displayBalance
		case model.AccountTypeEquity:
			result.Equity = append(result.Equity, row)
			result.TotalEquity[currency] += balance
		}
	}

	for ccy, assets := range result.TotalAssets {
		result.NetWorth[ccy] = assets - result.TotalLiabilities[ccy]
	}
	for ccy, liabs := range result.TotalLiabilities {
		if _, ok := result.NetWorth[ccy]; !ok {
			result.NetWorth[ccy] = -liabs
		}
	}

	sort.Slice(result.Assets, func(i, j int) bool {
		return result.Assets[i].Amount > result.Assets[j].Amount
	})
	sort.Slice(result.Liabilities, func(i, j int) bool {
		return result.Liabilities[i].Amount > result.Liabilities[j].Amount
	})
	sort.Slice(result.Equity, func(i, j int) bool {
		return result.Equity[i].Amount > result.Equity[j].Amount
	})

	return result, nil
}

// getOrCreateRowWithOffset is like getOrCreateRow but also sets OffsetAccount.
// The map key is expected to already encode both dimensions (e.g. "accName|offsetName").
func getOrCreateRowWithOffset(m map[string]*model.ReportRow, key, name, offset, currency string) *model.ReportRow {
	if _, ok := m[key]; !ok {
		m[key] = &model.ReportRow{AccountName: name, OffsetAccount: offset, Currency: currency}
	}
	return m[key]
}

// rowsFromMap converts a map of ReportRows to a slice sorted by amount descending.
func rowsFromMap(m map[string]*model.ReportRow) []model.ReportRow {
	rows := make([]model.ReportRow, 0, len(m))
	for _, r := range m {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Amount > rows[j].Amount
	})
	return rows
}
