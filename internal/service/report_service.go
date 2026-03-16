package service

import (
	"sort"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

// splitsToDetails converts raw Split records to SplitDetail by resolving account names.
func (ts *TransactionService) splitsToDetails(splits []*model.Split, accountCache map[int64]*model.Account) ([]model.SplitDetail, error) {
	details := make([]model.SplitDetail, 0, len(splits))
	for _, s := range splits {
		acc, ok := accountCache[s.AccountID]
		if !ok {
			var err error
			acc, err = ts.accRepo.GetAccountByID(s.AccountID)
			if err != nil {
				return nil, err
			}
			accountCache[s.AccountID] = acc
		}
		details = append(details, model.SplitDetail{
			ID:          s.ID,
			AccountID:   s.AccountID,
			AccountName: acc.Name,
			AccountType: acc.Type,
			Amount:      s.Amount,
			Currency:    s.Currency,
			Memo:        s.Memo,
		})
	}
	return details, nil
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

// GenerateIncomeStatement produces an income/expense summary for the given Unix time range.
// It walks every transaction in the period, classifies it, and groups amounts by account.
// The aggregation key is "accountName|offsetAccount" so that the same expense account
// funded from different offset accounts appears as separate rows.
func (ts *TransactionService) GenerateIncomeStatement(startTime, endTime int64) (*model.ReportResult, error) {
	transactions, err := ts.txRepo.GetTransactionsByDateRange(startTime, endTime)
	if err != nil {
		return nil, err
	}

	incomeByAccount := map[string]*model.ReportRow{}
	expenseByAccount := map[string]*model.ReportRow{}
	accountCache := map[int64]*model.Account{}

	for _, tx := range transactions {
		splits, err := ts.txRepo.GetSplitsByTransaction(tx.ID)
		if err != nil {
			return nil, err
		}

		details, err := ts.splitsToDetails(splits, accountCache)
		if err != nil {
			return nil, err
		}

		txType, err := ts.DetermineType(details)
		if err != nil {
			return nil, err
		}

		incomeOffset := offsetAccountName(details, model.AccountTypeRevenue)
		expenseOffset := offsetAccountName(details, model.AccountTypeExpense)

		for _, split := range details {
			switch txType {
			case model.TxTypeIncome:
				if split.AccountType == model.AccountTypeRevenue {
					key := split.AccountName + "|" + incomeOffset
					row := getOrCreateRowWithOffset(incomeByAccount, key, split.AccountName, incomeOffset, split.Currency)
					row.Amount += utils.AbsInt64(split.Amount)
					row.TxCount++
				}
			case model.TxTypeExpense:
				if split.AccountType == model.AccountTypeExpense {
					key := split.AccountName + "|" + expenseOffset
					row := getOrCreateRowWithOffset(expenseByAccount, key, split.AccountName, expenseOffset, split.Currency)
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
