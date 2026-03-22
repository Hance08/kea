package service

import (
	"sort"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

// GetNetWorthAt returns net worth (assets - liabilities) using all posted splits up to endTime.
func (ts *TransactionService) GetNetWorthAt(endTime int64) (int64, error) {
	txSplitsMap, err := ts.txRepo.GetSplitsWithAccountsByDateRange(0, endTime)
	if err != nil {
		return 0, err
	}

	var totalAssets, totalLiabilities int64
	for _, splits := range txSplitsMap {
		for _, split := range splits {
			switch split.AccountType {
			case model.AccountTypeAsset:
				totalAssets += split.Amount
			case model.AccountTypeLiability:
				totalLiabilities += split.Amount
			}
		}
	}

	return totalAssets - totalLiabilities, nil
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
func (ts *TransactionService) buildReportMaps(startTime, endTime int64, includeIncome, includeExpense bool) (incomeByAccount, expenseByAccount map[string]*model.ReportRow, err error) {
	txSplitsMap, err := ts.txRepo.GetSplitsWithAccountsByDateRange(startTime, endTime)
	if err != nil {
		return nil, nil, err
	}

	incomeByAccount = map[string]*model.ReportRow{}
	expenseByAccount = map[string]*model.ReportRow{}

	for _, details := range txSplitsMap {
		txType, err := ts.DetermineType(details)
		if err != nil {
			return nil, nil, err
		}

		if includeIncome && txType == model.TxTypeIncome {
			offset := offsetAccountName(details, model.AccountTypeRevenue)
			for _, split := range details {
				if split.AccountType == model.AccountTypeRevenue {
					key := split.AccountName + "|" + offset
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
					key := split.AccountName + "|" + offset
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
func (ts *TransactionService) GenerateIncomeStatement(startTime, endTime int64) (*model.ReportResult, error) {
	incomeByAccount, expenseByAccount, err := ts.buildReportMaps(startTime, endTime, true, true)
	if err != nil {
		return nil, err
	}

	result := &model.ReportResult{
		Currency:    ts.config.Defaults.Currency,
		IncomeRows:  rowsFromMap(incomeByAccount),
		ExpenseRows: rowsFromMap(expenseByAccount),
	}
	for _, r := range result.IncomeRows {
		result.TotalIncome += r.Amount
	}
	for _, r := range result.ExpenseRows {
		result.TotalExpense += r.Amount
	}
	result.NetAmount = result.TotalIncome - result.TotalExpense

	return result, nil
}

// GenerateIncomeBreakdown produces a detailed income-only report for the given Unix time range.
func (ts *TransactionService) GenerateIncomeBreakdown(startTime, endTime int64) (*model.ReportResult, error) {
	incomeByAccount, _, err := ts.buildReportMaps(startTime, endTime, true, false)
	if err != nil {
		return nil, err
	}

	rows := rowsFromMap(incomeByAccount)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Amount > rows[j].Amount })

	var total int64
	for _, r := range rows {
		total += r.Amount
	}

	return &model.ReportResult{
		Currency:    ts.config.Defaults.Currency,
		IncomeRows:  rows,
		TotalIncome: total,
	}, nil
}

// GenerateExpenseBreakdown produces a detailed expense-only report for the given Unix time range.
func (ts *TransactionService) GenerateExpenseBreakdown(startTime, endTime int64) (*model.ReportResult, error) {
	_, expenseByAccount, err := ts.buildReportMaps(startTime, endTime, false, true)
	if err != nil {
		return nil, err
	}

	rows := rowsFromMap(expenseByAccount)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Amount > rows[j].Amount })

	var total int64
	for _, r := range rows {
		total += r.Amount
	}

	return &model.ReportResult{
		Currency:     ts.config.Defaults.Currency,
		ExpenseRows:  rows,
		TotalExpense: total,
	}, nil
}

// GenerateBalanceSheet produces a current snapshot of all asset, liability, and equity account balances.
func (ts *TransactionService) GenerateBalanceSheet() (*model.BalanceSheetResult, error) {
	allAccounts, err := ts.accRepo.GetAllAccounts()
	if err != nil {
		return nil, err
	}

	result := &model.BalanceSheetResult{
		Currency: ts.config.Defaults.Currency,
	}

	for _, acc := range allAccounts {
		balance, err := ts.accRepo.GetAccountBalance(acc.ID)
		if err != nil {
			return nil, err
		}

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
			result.TotalAssets += balance
		case model.AccountTypeLiability:
			result.Liabilities = append(result.Liabilities, row)
			result.TotalLiabilities += balance
		case model.AccountTypeEquity:
			result.Equity = append(result.Equity, row)
			result.TotalEquity += balance
		}
	}

	result.NetWorth = result.TotalAssets - result.TotalLiabilities

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
