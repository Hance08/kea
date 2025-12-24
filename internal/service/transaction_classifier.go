package service

import (
	"github.com/hance08/kea/internal/constants"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

func (ts *TransactionService) DetermineType(splits []SplitDetail) (TransactionType, error) {
	// Fallback for empty splits
	if len(splits) == 0 {
		return TxTypeOther, nil
	}

	var totalRevenueAmount int64
	var totalExpenseAmount int64

	var (
		hasExpense      bool
		hasRevenue      bool
		hasEquity       bool
		assetOrLiabCnt  int
		isOpening       bool
		isAssetIncrease bool
	)

	for _, split := range splits {
		acc, err := ts.repo.GetAccountByID(split.AccountID)
		if err != nil {
			return TxTypeOther, err
		}

		if split.Memo == constants.OpeningAccountMemo {
			isOpening = true
		}

		switch acc.Type {
		case "E":
			hasExpense = true
			totalExpenseAmount += split.Amount
		case "R":
			hasRevenue = true
			totalRevenueAmount += utils.AbsInt64(split.Amount)
		case "A":
			assetOrLiabCnt++
			if split.Amount > 0 {
				isAssetIncrease = true
			}
		case "L":
			assetOrLiabCnt++
		case "C":
			hasEquity = true
		}
	}

	if isOpening {
		return TxTypeOpening, nil
	}

	if hasExpense && hasRevenue {
		if totalRevenueAmount >= totalExpenseAmount {
			return TxTypeIncome, nil
		}
		return TxTypeExpense, nil
	}

	if hasExpense && assetOrLiabCnt >= 1 {
		return TxTypeExpense, nil
	}

	if hasRevenue && assetOrLiabCnt >= 1 {
		return TxTypeIncome, nil
	}

	if assetOrLiabCnt >= 2 {
		return TxTypeTransfer, nil
	}

	if hasEquity && assetOrLiabCnt >= 1 {
		if isAssetIncrease {
			return TxTypeDeposit, nil
		}
		return TxTypeWithdrawal, nil
	}

	return TxTypeOther, nil
}

func (ts *TransactionService) GetDisplayAccount(splits []SplitDetail, txType string) (string, error) {
	if len(splits) == 0 {
		return "-", nil
	}

	switch txType {
	case "Expense":
		// Find and return the Expense account (E type)
		for _, split := range splits {
			account, err := ts.repo.GetAccountByName(split.AccountName)
			if err == nil && account.Type == "E" {
				return split.AccountName, nil
			}
		}

	case "Income":
		// Find and return the Revenue account (R type)
		for _, split := range splits {
			account, err := ts.repo.GetAccountByName(split.AccountName)
			if err == nil && account.Type == "R" {
				return split.AccountName, nil
			}
		}

	case "Transfer":
		// Find and return the Asset account with positive amount (receiving account)
		for _, split := range splits {
			if split.Amount > 0 {
				account, err := ts.repo.GetAccountByName(split.AccountName)
				if err == nil && (account.Type == "A" || account.Type == "L") {
					return split.AccountName, nil
				}
			}
		}

	case "Opening":
		// For opening transactions, return the non-equity account
		for _, split := range splits {
			account, err := ts.repo.GetAccountByName(split.AccountName)
			if err == nil && account.Type != "C" {
				return split.AccountName, nil
			}
		}

	case "Other":
		// For other types, return the first account with positive amount
		for _, split := range splits {
			if split.Amount > 0 {
				return split.AccountName, nil
			}
		}
	}

	// Fallback: return first account name
	if len(splits) > 0 {
		return splits[0].AccountName, nil
	}

	return "-", nil
}

func (ts *TransactionService) GetDisplayAmount(splits []SplitDetail) (int64, string) {
	if len(splits) == 0 {
		return 0, ""
	}

	var maxAmount int64
	var currency string
	if len(splits) > 0 {
		currency = splits[0].Currency
	}

	for _, split := range splits {
		if split.Amount > maxAmount {
			maxAmount = split.Amount
			currency = split.Currency
		}
	}

	return maxAmount, currency
}

func (ts *TransactionService) GetAllowedAccounts(txType TransactionType, currentAccountType string, allAccounts []*model.Account) []*model.Account {
	switch txType {
	case TxTypeExpense:
		if currentAccountType == "E" {
			return ts.filterAccountsByTypes(allAccounts, []string{"E"})
		}
		return ts.filterAccountsByTypes(allAccounts, []string{"A", "L"})

	case TxTypeIncome:
		if currentAccountType == "R" {
			return ts.filterAccountsByTypes(allAccounts, []string{"R"})
		}
		return ts.filterAccountsByTypes(allAccounts, []string{"A", "L"})

	case TxTypeTransfer:
		return ts.filterAccountsByTypes(allAccounts, []string{"A", "L"})

	default:
		return allAccounts
	}
}

func (ts *TransactionService) filterAccountsByTypes(accounts []*model.Account, allowedTypes []string) []*model.Account {
	var filtered []*model.Account

	typeMap := make(map[string]bool)
	for _, t := range allowedTypes {
		typeMap[t] = true
	}

	for _, acc := range accounts {
		if typeMap[acc.Type] {
			filtered = append(filtered, acc)
		}
	}
	return filtered
}
