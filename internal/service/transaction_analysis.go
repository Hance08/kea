package service

import (
	"github.com/hance08/kea/internal/constants"
	"github.com/hance08/kea/internal/store"
)

type TransactionType string

const (
	TxTypeExpense  TransactionType = "Expense"
	TxTypeIncome   TransactionType = "Income"
	TxTypeTransfer TransactionType = "Transfer"
	TxTypeOpening  TransactionType = "Opening"
	TxTypeOther    TransactionType = "Other"
)

func (s *TransactionService) DetectTransactionType(splits []SplitDetail) (TransactionType, error) {
	// Fallback for empty splits
	if len(splits) == 0 {
		return TxTypeOther, nil
	}

	// Collect account types for all splits
	accountTypes := make([]string, 0, len(splits))
	accountNames := make([]string, 0, len(splits))

	for _, split := range splits {
		acc, err := s.repo.GetAccountByID(split.AccountID)
		if err != nil {
			return TxTypeOther, err
		}
		accountTypes = append(accountTypes, acc.Type)
		accountNames = append(accountNames, acc.Name)
	}

	// Opening check: any equity split with system opening balance account
	isOpening := false
	for i, t := range accountTypes {
		if t == constants.TypeEquity && accountNames[i] == constants.SystemAccountOpeningBalance {
			isOpening = true
			break
		}
	}
	if isOpening {
		return TxTypeOpening, nil
	}

	var (
		hasExpense     bool
		hasRevenue     bool
		assetOrLiabCnt int
	)

	for _, t := range accountTypes {
		switch t {
		case "E":
			hasExpense = true
		case "R":
			hasRevenue = true
		case "A", "L":
			assetOrLiabCnt++
		}
	}

	// Prioritize transfer when there are two or more asset/liability legs,
	// even if there are extra expense/revenue splits (e.g., fees).
	if assetOrLiabCnt >= 2 {
		return TxTypeTransfer, nil
	}

	if hasExpense && assetOrLiabCnt >= 1 {
		return TxTypeExpense, nil
	}

	if hasRevenue && assetOrLiabCnt >= 1 {
		return TxTypeIncome, nil
	}

	return TxTypeOther, nil
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
