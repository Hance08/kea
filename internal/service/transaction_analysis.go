package service

import (
	"github.com/hance08/kea/internal/constants"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/internal/utils"
)

type TransactionType string

const (
	TxTypeExpense    TransactionType = "Expense"
	TxTypeIncome     TransactionType = "Income"
	TxTypeTransfer   TransactionType = "Transfer"
	TxTypeOpening    TransactionType = "Opening"
	TxTypeDeposit    TransactionType = "Deposit"
	TxTypeWithdrawal TransactionType = "Withdrawal"
	TxTypeOther      TransactionType = "Other"
)

func (s *TransactionService) DetectTransactionType(splits []SplitDetail) (TransactionType, error) {
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
		acc, err := s.repo.GetAccountByID(split.AccountID)
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

	// Prioritize transfer when there are two or more asset/liability legs,
	// even if there are extra expense/revenue splits (e.g., fees).
	if assetOrLiabCnt >= 2 {
		return TxTypeTransfer, nil
	}

	if hasExpense && hasRevenue {
		if totalRevenueAmount >= totalExpenseAmount {
			return TxTypeIncome, nil
		} else {
			return TxTypeExpense, nil
		}
	}
	if hasExpense && assetOrLiabCnt >= 1 {
		return TxTypeExpense, nil
	}

	if hasRevenue && assetOrLiabCnt >= 1 {
		return TxTypeIncome, nil
	}

	if hasEquity && assetOrLiabCnt >= 1 {
		if isAssetIncrease {
			return TxTypeDeposit, nil
		} else {
			return TxTypeWithdrawal, nil
		}

	}

	return TxTypeOther, nil
}

func (s *TransactionService) DetectTransactionAccount(splits []SplitDetail, txType string) (string, error) {
	if len(splits) == 0 {
		return "-", nil
	}

	switch txType {
	case "Expense":
		// Find and return the Expense account (E type)
		for _, split := range splits {
			account, err := s.repo.GetAccountByName(split.AccountName)
			if err == nil && account.Type == "E" {
				return split.AccountName, nil
			}
		}

	case "Income":
		// Find and return the Revenue account (R type)
		for _, split := range splits {
			account, err := s.repo.GetAccountByName(split.AccountName)
			if err == nil && account.Type == "R" {
				return split.AccountName, nil
			}
		}

	case "Transfer":
		// Find and return the Asset account with positive amount (receiving account)
		for _, split := range splits {
			if split.Amount > 0 {
				account, err := s.repo.GetAccountByName(split.AccountName)
				if err == nil && (account.Type == "A" || account.Type == "L") {
					return split.AccountName, nil
				}
			}
		}

	case "Opening":
		// For opening transactions, return the non-equity account
		for _, split := range splits {
			account, err := s.repo.GetAccountByName(split.AccountName)
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

func (s *TransactionService) GetEligibleAccountsForEdit(txType TransactionType, currentAccountType string, allAccounts []*store.Account) []*store.Account {
	switch txType {
	case TxTypeExpense:
		if currentAccountType == "E" {
			return s.filterAccountsByType(allAccounts, []string{"E"})
		}
		return s.filterAccountsByType(allAccounts, []string{"A", "L"})

	case TxTypeIncome:
		if currentAccountType == "R" {
			return s.filterAccountsByType(allAccounts, []string{"R"})
		}
		return s.filterAccountsByType(allAccounts, []string{"A", "L"})

	case TxTypeTransfer:
		return s.filterAccountsByType(allAccounts, []string{"A", "L"})

	default:
		return allAccounts
	}
}

func (s *TransactionService) filterAccountsByType(accounts []*store.Account, allowedTypes []string) []*store.Account {
	var filtered []*store.Account

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
