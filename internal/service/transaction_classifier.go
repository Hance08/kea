// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"fmt"
	"time"

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
		hasExpense                  bool
		hasRevenue                  bool
		hasEquity                   bool
		assetOrLiabCnt              int
		isOpening                   bool
		isAssetIncrease             bool
		hasInvestmentAccount        bool
		nonInvestmentAssetOrLiabCnt int
	)

	for _, split := range splits {
		accType, err := ts.resolveAccountType(ctx, split)
		if err != nil {
			return model.TxTypeOther, err
		}

		if split.Memo == model.OpeningAccountMemo {
			isOpening = true
		}

		switch accType {
		case model.AccountTypeExpense:
			hasExpense = true
			totalExpenseAmount += utils.AbsInt64(split.Amount)
		case model.AccountTypeRevenue:
			hasRevenue = true
			totalRevenueAmount += utils.AbsInt64(split.Amount)
		case model.AccountTypeAsset:
			assetOrLiabCnt++
			if model.IsInvestmentAccount(split.AccountName) {
				hasInvestmentAccount = true
			} else {
				nonInvestmentAssetOrLiabCnt++
			}
			if split.Amount > 0 {
				isAssetIncrease = true
				totalPositiveAssetLiabAmount += split.Amount
			}
		case model.AccountTypeLiability:
			assetOrLiabCnt++
			nonInvestmentAssetOrLiabCnt++
			if split.Amount > 0 {
				totalPositiveAssetLiabAmount += split.Amount
			}
		case model.AccountTypeEquity:
			hasEquity = true
		}
	}

	if isOpening {
		return model.TxTypeOpening, nil
	}

	if hasInvestmentAccount && nonInvestmentAssetOrLiabCnt >= 1 {
		return model.TxTypeInvestment, nil
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

	switch txType {
	case "Expense":
		// Find and return the Expense account (E type)
		for _, split := range splits {
			accType, err := ts.resolveAccountType(ctx, split)
			if err == nil && accType == model.AccountTypeExpense {
				return split.AccountName, nil
			}
		}

	case "Income":
		// Find and return the Revenue account (R type)
		for _, split := range splits {
			accType, err := ts.resolveAccountType(ctx, split)
			if err == nil && accType == model.AccountTypeRevenue {
				return split.AccountName, nil
			}
		}

	case "Transfer":
		// Find and return the Asset/Liability account with positive amount (receiving account)
		for _, split := range splits {
			if split.Amount > 0 {
				accType, err := ts.resolveAccountType(ctx, split)
				if err == nil && (accType == model.AccountTypeAsset || accType == model.AccountTypeLiability) {
					return split.AccountName, nil
				}
			}
		}

	case "Opening":
		// For opening transactions, return the non-equity account
		for _, split := range splits {
			accType, err := ts.resolveAccountType(ctx, split)
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
			typeVal, err := ts.resolveAccountType(ctx, split)
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

// BuildTransactionListItems assembles display-ready list items from transactions and their details.
func (ts *TransactionService) BuildTransactionListItems(ctx context.Context, txs []*model.Transaction, details map[int64]*model.TransactionDetail) []model.TransactionListItem {
	items := make([]model.TransactionListItem, 0, len(txs))
	for _, tx := range txs {
		detail, ok := details[tx.ID]
		if !ok {
			continue
		}

		txType := string(detail.Type)

		accountName, err := ts.GetDisplayAccount(ctx, detail.Splits, txType)
		if err != nil {
			accountName = "-"
		}

		offsetAccount, err := ts.GetDisplayOffsetAccount(ctx, detail.Splits, txType, accountName)
		if err != nil {
			offsetAccount = "-"
		}

		amountCents, currency := ts.GetDisplayAmount(detail.Splits)

		items = append(items, model.TransactionListItem{
			ID:            tx.ID,
			Date:          time.Unix(tx.Timestamp, 0).Format(model.DateFormat),
			Type:          txType,
			Account:       accountName,
			OffsetAccount: offsetAccount,
			Description:   tx.Description,
			Amount:        amountCents,
			Currency:      currency,
			Status:        tx.Status.String(),
		})
	}
	return items
}

func (ts *TransactionService) GetAllowedAccounts(txType model.TransactionType, currentAccountType model.AccountType, allAccounts []*model.Account) []*model.Account {
	switch txType {
	case model.TxTypeExpense:
		if currentAccountType == model.AccountTypeExpense {
			return ts.filterAccountsByTypes(allAccounts, []model.AccountType{model.AccountTypeExpense})
		}
		return ts.filterAccountsByTypes(allAccounts, []model.AccountType{model.AccountTypeAsset, model.AccountTypeLiability})

	case model.TxTypeIncome:
		if currentAccountType == model.AccountTypeRevenue {
			return ts.filterAccountsByTypes(allAccounts, []model.AccountType{model.AccountTypeRevenue})
		}
		return ts.filterAccountsByTypes(allAccounts, []model.AccountType{model.AccountTypeAsset, model.AccountTypeLiability})

	case model.TxTypeTransfer:
		return ts.filterAccountsByTypes(allAccounts, []model.AccountType{model.AccountTypeAsset, model.AccountTypeLiability})

	default:
		return allAccounts
	}
}

func (ts *TransactionService) resolveAccountType(ctx context.Context, s model.SplitDetail) (model.AccountType, error) {
	if s.AccountID > 0 {
		acc, err := ts.accRepo.GetAccountByID(ctx, s.AccountID)
		if err != nil {
			return "", err
		}
		return acc.Type, nil
	}
	if s.AccountName != "" {
		acc, err := ts.accRepo.GetAccountByName(ctx, s.AccountName)
		if err != nil {
			return "", err
		}
		return acc.Type, nil
	}
	return "", fmt.Errorf("split has neither AccountID nor AccountName")
}

func (ts *TransactionService) ValidateSplitsMatchType(ctx context.Context, txType model.TransactionType, splits []model.SplitDetail) error {
	switch txType {
	case model.TxTypeOpening, model.TxTypeOther, model.TxTypeDeposit, model.TxTypeWithdrawal:
		return nil

	case model.TxTypeExpense:
		var hasExpense, hasAssetOrLiab bool
		for _, s := range splits {
			accType, err := ts.resolveAccountType(ctx, s)
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
			accType, err := ts.resolveAccountType(ctx, s)
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
			accType, err := ts.resolveAccountType(ctx, s)
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

func (ts *TransactionService) filterAccountsByTypes(accounts []*model.Account, allowedTypes []model.AccountType) []*model.Account {
	var filtered []*model.Account

	typeMap := make(map[model.AccountType]bool)
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
