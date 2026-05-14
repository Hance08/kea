// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

func (ts *TransactionService) DetermineType(ctx context.Context, splits []model.SplitDetail) (model.TransactionType, error) {
	// Fallback for empty splits
	if len(splits) == 0 {
		return model.TxTypeOther, nil
	}

	var totalRevenueAmount int64
	var totalExpenseAmount int64
	var totalPositiveAssetLiabAmount int64

	var (
		hasExpense      bool
		hasRevenue      bool
		hasEquity       bool
		assetOrLiabCnt  int
		isOpening       bool
		isAssetIncrease bool
	)

	for _, split := range splits {
		accType := split.AccountType
		if accType == "" {
			acc, err := ts.accRepo.GetAccountByID(ctx, split.AccountID)
			if err != nil {
				return model.TxTypeOther, err
			}
			accType = acc.Type
		}

		if split.Memo == model.OpeningAccountMemo {
			isOpening = true
		}

		switch accType {
		case "E":
			hasExpense = true
			totalExpenseAmount += utils.AbsInt64(split.Amount)
		case "R":
			hasRevenue = true
			totalRevenueAmount += utils.AbsInt64(split.Amount)
		case "A":
			assetOrLiabCnt++
			if split.Amount > 0 {
				isAssetIncrease = true
				totalPositiveAssetLiabAmount += split.Amount
			}
		case "L":
			assetOrLiabCnt++
			if split.Amount > 0 {
				totalPositiveAssetLiabAmount += split.Amount
			}
		case "C":
			hasEquity = true
		}
	}

	if isOpening {
		return model.TxTypeOpening, nil
	}

	if hasExpense && hasRevenue {
		if totalRevenueAmount >= totalExpenseAmount {
			return model.TxTypeIncome, nil
		}
		return model.TxTypeExpense, nil
	}

	if hasExpense && assetOrLiabCnt >= 2 {
		if totalPositiveAssetLiabAmount > totalExpenseAmount {
			return model.TxTypeTransfer, nil
		}
		return model.TxTypeExpense, nil
	}
	if hasExpense && assetOrLiabCnt == 1 {
		return model.TxTypeExpense, nil
	}

	if hasRevenue && assetOrLiabCnt >= 1 {
		return model.TxTypeIncome, nil
	}

	if assetOrLiabCnt >= 2 {
		return model.TxTypeTransfer, nil
	}

	if hasEquity && assetOrLiabCnt >= 1 {
		if isAssetIncrease {
			return model.TxTypeDeposit, nil
		}
		return model.TxTypeWithdrawal, nil
	}

	return model.TxTypeOther, nil
}

func (ts *TransactionService) GetDisplayAccount(ctx context.Context, splits []model.SplitDetail, txType string) (string, error) {
	if len(splits) == 0 {
		return "-", nil
	}

	resolveType := func(split model.SplitDetail) (model.AccountType, error) {
		if split.AccountType != "" {
			return split.AccountType, nil
		}
		account, err := ts.accRepo.GetAccountByName(ctx, split.AccountName)
		if err != nil {
			return "", err
		}
		return account.Type, nil
	}

	switch txType {
	case "Expense":
		// Find and return the Expense account (E type)
		for _, split := range splits {
			accType, err := resolveType(split)
			if err == nil && accType == model.AccountTypeExpense {
				return split.AccountName, nil
			}
		}

	case "Income":
		// Find and return the Revenue account (R type)
		for _, split := range splits {
			accType, err := resolveType(split)
			if err == nil && accType == model.AccountTypeRevenue {
				return split.AccountName, nil
			}
		}

	case "Transfer":
		// Find and return the Asset/Liability account with positive amount (receiving account)
		for _, split := range splits {
			if split.Amount > 0 {
				accType, err := resolveType(split)
				if err == nil && (accType == model.AccountTypeAsset || accType == model.AccountTypeLiability) {
					return split.AccountName, nil
				}
			}
		}

	case "Opening":
		// For opening transactions, return the non-equity account
		for _, split := range splits {
			accType, err := resolveType(split)
			if err == nil && accType != model.AccountTypeEquity {
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

func (ts *TransactionService) GetDisplayAmount(splits []model.SplitDetail) (int64, string) {
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

func (ts *TransactionService) GetDisplayOffsetAccount(ctx context.Context, splits []model.SplitDetail, txType string, primaryAccount string) (string, error) {
	if len(splits) == 0 {
		return "-", nil
	}

	resolveAccountType := func(split model.SplitDetail) (model.AccountType, error) {
		if split.AccountType != "" {
			return split.AccountType, nil
		}
		account, err := ts.accRepo.GetAccountByName(ctx, split.AccountName)
		if err != nil {
			return "", err
		}
		return account.Type, nil
	}

	seen := map[string]struct{}{}

	switch txType {
	case string(model.TxTypeExpense), string(model.TxTypeIncome):
		var primaryType model.AccountType
		if txType == string(model.TxTypeExpense) {
			primaryType = model.AccountTypeExpense
		} else {
			primaryType = model.AccountTypeRevenue
		}

		for _, split := range splits {
			typeVal, err := resolveAccountType(split)
			if err != nil {
				return "", err
			}
			if typeVal != primaryType {
				seen[split.AccountName] = struct{}{}
			}
		}
	default:
		for _, split := range splits {
			if split.AccountName != primaryAccount {
				seen[split.AccountName] = struct{}{}
			}
		}
	}

	switch len(seen) {
	case 0:
		return "-", nil
	case 1:
		for name := range seen {
			return name, nil
		}
	}

	return "(multiple)", nil
}

func (ts *TransactionService) GetAllowedAccounts(txType model.TransactionType, currentAccountType model.AccountType, allAccounts []*model.Account) []*model.Account {
	switch txType {
	case model.TxTypeExpense:
		if currentAccountType == "E" {
			return ts.filterAccountsByTypes(allAccounts, []string{"E"})
		}
		return ts.filterAccountsByTypes(allAccounts, []string{"A", "L"})

	case model.TxTypeIncome:
		if currentAccountType == "R" {
			return ts.filterAccountsByTypes(allAccounts, []string{"R"})
		}
		return ts.filterAccountsByTypes(allAccounts, []string{"A", "L"})

	case model.TxTypeTransfer:
		return ts.filterAccountsByTypes(allAccounts, []string{"A", "L"})

	default:
		return allAccounts
	}
}

func (ts *TransactionService) ValidateSplitsMatchType(ctx context.Context, txType model.TransactionType, splits []model.SplitDetail) error {
	resolveType := func(s model.SplitDetail) (model.AccountType, error) {
		if s.AccountType != "" {
			return s.AccountType, nil
		}
		acc, err := ts.accRepo.GetAccountByName(ctx, s.AccountName)
		if err != nil {
			return "", err
		}
		return acc.Type, nil
	}

	switch txType {
	case model.TxTypeOpening, model.TxTypeOther, model.TxTypeDeposit, model.TxTypeWithdrawal:
		return nil

	case model.TxTypeExpense:
		var hasExpense, hasAssetOrLiab bool
		for _, s := range splits {
			accType, err := resolveType(s)
			if err != nil {
				return err
			}
			if accType == model.AccountTypeExpense {
				hasExpense = true
			}
			if accType == model.AccountTypeAsset || accType == model.AccountTypeLiability {
				hasAssetOrLiab = true
			}
		}
		if !hasExpense {
			return validationErrorf("type", "expense transaction requires at least one Expense account")
		}
		if !hasAssetOrLiab {
			return validationErrorf("type", "expense transaction requires at least one Asset or Liability account")
		}

	case model.TxTypeIncome:
		var hasRevenue, hasAssetOrLiab bool
		for _, s := range splits {
			accType, err := resolveType(s)
			if err != nil {
				return err
			}
			if accType == model.AccountTypeRevenue {
				hasRevenue = true
			}
			if accType == model.AccountTypeAsset || accType == model.AccountTypeLiability {
				hasAssetOrLiab = true
			}
		}
		if !hasRevenue {
			return validationErrorf("type", "income transaction requires at least one Revenue account")
		}
		if !hasAssetOrLiab {
			return validationErrorf("type", "income transaction requires at least one Asset or Liability account")
		}

	case model.TxTypeTransfer:
		for _, s := range splits {
			accType, err := resolveType(s)
			if err != nil {
				return err
			}
			if accType != model.AccountTypeAsset && accType != model.AccountTypeLiability {
				return validationErrorf("type", "transfer transaction must only contain Asset and Liability accounts (found account type %q)", accType)
			}
		}

	default:
		return validationErrorf("type", "unknown transaction type %q", txType)
	}

	return nil
}

func (ts *TransactionService) filterAccountsByTypes(accounts []*model.Account, allowedTypes []string) []*model.Account {
	var filtered []*model.Account

	typeMap := make(map[string]bool)
	for _, t := range allowedTypes {
		typeMap[t] = true
	}

	for _, acc := range accounts {
		if typeMap[string(acc.Type)] {
			filtered = append(filtered, acc)
		}
	}
	return filtered
}
