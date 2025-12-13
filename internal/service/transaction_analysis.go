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
	if len(splits) != 2 {
		return TxTypeOther, nil
	}

	acc1, err := s.repo.GetAccountByID(splits[0].AccountID)
	if err != nil {
		return TxTypeOther, err
	}
	acc2, err := s.repo.GetAccountByID(splits[1].AccountID)
	if err != nil {
		return TxTypeOther, err
	}

	type1 := acc1.Type
	type2 := acc2.Type
	name1 := acc1.Name
	name2 := acc2.Name

	if type1 == constants.TypeEquity || type2 == constants.TypeEquity {
		if name1 == constants.SystemAccountOpeningBalance || name2 == constants.SystemAccountOpeningBalance {
			return TxTypeOpening, nil
		}
	}

	isAssetOrLiab1 := type1 == "A" || type1 == "L"
	isAssetOrLiab2 := type2 == "A" || type2 == "L"

	if isAssetOrLiab1 && type2 == "E" {
		return TxTypeExpense, nil
	}
	if type1 == "E" && isAssetOrLiab2 {
		return TxTypeExpense, nil
	}

	if type1 == "R" && isAssetOrLiab2 {
		return TxTypeIncome, nil
	}
	if isAssetOrLiab1 && type2 == "R" {
		return TxTypeIncome, nil
	}

	if isAssetOrLiab1 && isAssetOrLiab2 {
		return TxTypeTransfer, nil
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
