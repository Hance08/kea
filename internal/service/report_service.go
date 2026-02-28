package service

import (
	"sort"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

// splitsToDetails converts raw Split records to SplitDetail by resolving account names.
func (ts *TransactionService) splitsToDetails(splits []*model.Split) ([]model.SplitDetail, error) {
	details := make([]model.SplitDetail, 0, len(splits))
	for _, s := range splits {
		acc, err := ts.accRepo.GetAccountByID(s.AccountID)
		if err != nil {
			return nil, err
		}
		details = append(details, model.SplitDetail{
			ID:          s.ID,
			AccountID:   s.AccountID,
			AccountName: acc.Name,
			Amount:      s.Amount,
			Currency:    s.Currency,
			Memo:        s.Memo,
		})
	}
	return details, nil
}

// GenerateIncomeStatement produces an income/expense summary for the given Unix time range.
// It walks every transaction in the period, classifies it, and groups amounts by account.
func (ts *TransactionService) GenerateIncomeStatement(startTime, endTime int64) (*model.ReportResult, error) {
	transactions, err := ts.txRepo.GetTransactionsByDateRange(startTime, endTime)
	if err != nil {
		return nil, err
	}

	incomeByAccount := map[string]*model.ReportRow{}
	expenseByAccount := map[string]*model.ReportRow{}

	for _, tx := range transactions {
		splits, err := ts.txRepo.GetSplitsByTransaction(tx.ID)
		if err != nil {
			return nil, err
		}

		details, err := ts.splitsToDetails(splits)
		if err != nil {
			return nil, err
		}

		txType, err := ts.DetermineType(details)
		if err != nil {
			return nil, err
		}

		for _, split := range details {
			acc, err := ts.accRepo.GetAccountByID(split.AccountID)
			if err != nil {
				return nil, err
			}

			switch txType {
			case model.TxTypeIncome:
				if acc.Type == model.AccountTypeRevenue {
					row := getOrCreateRow(incomeByAccount, acc.Name, split.Currency)
					row.Amount += utils.AbsInt64(split.Amount)
					row.TxCount++
				}
			case model.TxTypeExpense:
				if acc.Type == model.AccountTypeExpense {
					row := getOrCreateRow(expenseByAccount, acc.Name, split.Currency)
					row.Amount += utils.AbsInt64(split.Amount)
					row.TxCount++
				}
			}
		}
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

// GenerateExpenseBreakdown produces a detailed expense-only report for the given Unix time range.
func (ts *TransactionService) GenerateExpenseBreakdown(startTime, endTime int64) (*model.ReportResult, error) {
	result, err := ts.GenerateIncomeStatement(startTime, endTime)
	if err != nil {
		return nil, err
	}

	// Sort expense rows by amount descending for readability.
	sort.Slice(result.ExpenseRows, func(i, j int) bool {
		return result.ExpenseRows[i].Amount > result.ExpenseRows[j].Amount
	})

	// Keep only expense information.
	result.IncomeRows = nil
	result.TotalIncome = 0
	result.NetAmount = 0

	return result, nil
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

	return result, nil
}

// getOrCreateRow retrieves an existing ReportRow from the map or initialises a new one.
func getOrCreateRow(m map[string]*model.ReportRow, name, currency string) *model.ReportRow {
	if _, ok := m[name]; !ok {
		m[name] = &model.ReportRow{AccountName: name, Currency: currency}
	}
	return m[name]
}

// rowsFromMap converts a map of ReportRows to a slice sorted by account name.
func rowsFromMap(m map[string]*model.ReportRow) []model.ReportRow {
	rows := make([]model.ReportRow, 0, len(m))
	for _, r := range m {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].AccountName < rows[j].AccountName
	})
	return rows
}
