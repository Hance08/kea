// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/ui/prompts"
)

func (r *addRunner) runFromFlags(ctx context.Context, flags *addFlags) (addTransactionInput, error) {
	if len(flags.Splits) > 0 {
		return r.runFromSplitFlags(flags)
	}

	// Flag mode: validate all required flags
	if flags.Amount == "" || flags.From == "" || flags.To == "" {
		return addTransactionInput{}, fmt.Errorf("when using flags, --amount, --from, and --to are all required")
	}

	description := flags.Description
	if description == "" {
		description = "-"
	}

	// Parse amount
	amountCents, err := utils.ParseAmount(flags.Amount)
	if err != nil {
		return addTransactionInput{}, fmt.Errorf("invalid amount: %w", err)
	}

	// Parse status (default to Cleared when not specified)
	status := model.StatusCleared
	if flags.Status != "" {
		status, err = model.ParseTransactionStatus(flags.Status)
		if err != nil {
			return addTransactionInput{}, err
		}
	}

	// Parse timestamp
	timestamp, err := r.txSvc.ParseTransactionDate(flags.Timestamp)
	if err != nil {
		return addTransactionInput{}, err
	}

	var txType model.TransactionType
	if flags.Type != "" {
		txType, err = model.ParseTransactionType(flags.Type)
		if err != nil {
			return addTransactionInput{}, err
		}
	}

	if err := r.validateAccountSelectable(ctx, flags.From, nil, "--from"); err != nil {
		return addTransactionInput{}, err
	}

	if err := r.validateAccountSelectable(ctx, flags.To, nil, "--to"); err != nil {
		return addTransactionInput{}, err
	}

	return addTransactionInput{
		FromAccountID: flags.From,
		ToAccountID:   flags.To,
		AmountCents:   amountCents,
		Description:   description,
		Timestamp:     timestamp,
		Status:        status,
		Type:          txType,
	}, nil
}

func (r *addRunner) runInteractive(ctx context.Context) (addTransactionInput, error) {
	// Get all accounts
	accounts, err := r.accSvc.GetAllAccounts(ctx)
	if err != nil {
		return addTransactionInput{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	// Step 1: Select transaction type
	rawType, err := prompts.PromptTransactionType()
	if err != nil {
		return addTransactionInput{}, err
	}

	mode := model.ParseTransactionTypeLabel(rawType)

	// Step 2: Get description (optional)
	description, err := prompts.PromptDescription("Transaction description (optional):", false)
	if err != nil {
		return addTransactionInput{}, err
	}
	if description == "" {
		description = "-"
	}

	// Step 3: Get amount
	amountStr, err := prompts.PromptAmount(
		"Amount:",
		"Enter the amount, no need currency symbol(e.g. 150 or 150.50)",
		prompts.ValidateAmountFormat(false),
	)
	if err != nil {
		return addTransactionInput{}, err
	}

	amountCents, err := utils.ParseAmount(amountStr)
	if err != nil {
		return addTransactionInput{}, fmt.Errorf("invalid amount: %w", err)
	}

	// Step 4 & 5: Select accounts based on mode
	rule, err := r.txSvc.GetTransactionRule(mode)
	if err != nil {
		return addTransactionInput{}, err
	}

	uiConf, ok := modeUIConfigs[mode]
	if !ok {
		return addTransactionInput{}, fmt.Errorf("UI config missing for mode: %s", mode)
	}

	fromAccount, err := r.selectAccount(ctx, accounts, rule.SourceTypes, uiConf.Src, true, "")
	if err != nil {
		return addTransactionInput{}, err
	}

	// Resolve the selected account's currency so the offset account list
	// is filtered to the same currency, preventing mixed-currency splits.
	fromAcc, err := r.accSvc.GetAccountByName(ctx, fromAccount)
	if err != nil {
		return addTransactionInput{}, fmt.Errorf("failed to load account %q: %w", fromAccount, err)
	}

	toAccount, err := r.selectAccount(ctx, accounts, rule.DestTypes, uiConf.Dst, mode != model.TxTypeExpense, fromAcc.Currency)
	if err != nil {
		return addTransactionInput{}, err
	}

	// Step 6: Transaction status
	statusStr, err := prompts.PromptTransactionStatus("Cleared")
	if err != nil {
		return addTransactionInput{}, err
	}

	status := model.StatusCleared
	if statusStr == "Pending" {
		status = model.StatusPending
	}

	// Step 7: Transaction date
	dateStr, err := prompts.PromptTransactionDate()
	if err != nil {
		return addTransactionInput{}, err
	}

	timestamp, err := r.txSvc.ParseTransactionDate(dateStr)
	if err != nil {
		return addTransactionInput{}, err
	}

	return addTransactionInput{
		FromAccountID: fromAccount,
		ToAccountID:   toAccount,
		AmountCents:   amountCents,
		Description:   description,
		Timestamp:     timestamp,
		Status:        status,
		Type:          mode,
	}, nil
}

func (r *addRunner) selectAccount(ctx context.Context, accounts []*model.Account, allowedTypes []string, message string, showBalance bool, allowedCurrency string) (string, error) {
	var balanceGetter func(int64) (string, error)
	if showBalance {
		balanceGetter = func(id int64) (string, error) {
			return r.accSvc.GetAccountBalanceFormatted(ctx, id)
		}
	}

	return prompts.PromptAccountSelection(accounts, allowedTypes, message, showBalance, balanceGetter, allowedCurrency)
}

func (r *addRunner) validateAccountSelectable(ctx context.Context, accountName string, allowedTypes []string, flagName string) error {
	if err := r.accSvc.ValidateSelectableAccount(ctx, accountName, allowedTypes); err != nil {
		return fmt.Errorf("%s: %w", flagName, err)
	}
	return nil
}


var modeUIConfigs = map[model.TransactionType]struct{ Src, Dst string }{
	model.TxTypeExpense:  {"Payment Source:", "Expense Type:"},
	model.TxTypeIncome:   {"Revenue Type:", "Deposit To:"},
	model.TxTypeTransfer: {"From Account:", "To Account:"},
}

func (r *addRunner) runFromSplitFlags(flags *addFlags) (addTransactionInput, error) {
	if flags.Type == "" {
		return addTransactionInput{}, fmt.Errorf("--type is required when using --split")
	}
	if len(flags.Splits) < 2 {
		return addTransactionInput{}, fmt.Errorf("--split requires at least 2 splits, got %d", len(flags.Splits))
	}

	txType, err := model.ParseTransactionType(flags.Type)
	if err != nil {
		return addTransactionInput{}, err
	}

	description := flags.Description
	if description == "" {
		description = "-"
	}

	status := model.StatusCleared
	if flags.Status != "" {
		status, err = model.ParseTransactionStatus(flags.Status)
		if err != nil {
			return addTransactionInput{}, err
		}
	}

	timestamp, err := r.txSvc.ParseTransactionDate(flags.Timestamp)
	if err != nil {
		return addTransactionInput{}, err
	}

	splits := make([]model.SplitDetail, 0, len(flags.Splits))
	for _, s := range flags.Splits {
		split, err := parseSplitFlag(s)
		if err != nil {
			return addTransactionInput{}, err
		}
		splits = append(splits, split)
	}

	return addTransactionInput{
		Splits:      splits,
		Description: description,
		Timestamp:   timestamp,
		Status:      status,
		Type:        txType,
	}, nil
}

func parseSplitFlag(s string) (model.SplitDetail, error) {
	i := strings.LastIndex(s, "=")
	if i < 0 {
		return model.SplitDetail{}, fmt.Errorf("invalid split %q: expected format AccountName=amount", s)
	}
	accountName := s[:i]
	amountStr := s[i+1:]
	if accountName == "" {
		return model.SplitDetail{}, fmt.Errorf("invalid split %q: account name cannot be empty", s)
	}
	cents, err := utils.ParseAmount(amountStr)
	if err != nil {
		return model.SplitDetail{}, fmt.Errorf("invalid split %q: %w", s, err)
	}
	return model.SplitDetail{AccountName: accountName, Amount: cents}, nil
}
