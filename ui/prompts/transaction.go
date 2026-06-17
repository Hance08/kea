// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package prompts

import (
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/hance08/kea/internal/model"
)

// PromptTransactionType prompts for transaction type selection
func PromptTransactionType() (string, error) {
	options := []string{
		"Record Expense",
		"Record Income",
		"Transfer",
		"Record Investment",
	}

	selected, err := PromptSelect("Choose the transaction type:", options, "Record Expense")
	if err != nil {
		return "", err
	}

	return selected, nil
}

// PromptTransactionStatus prompts for transaction status
func PromptTransactionStatus(defaultStatus string) (string, error) {
	options := []string{"Cleared", "Pending"}

	if defaultStatus == "" {
		defaultStatus = "Cleared"
	}

	selected, err := PromptSelect("Transaction status:", options, defaultStatus)
	if err != nil {
		return "", err
	}

	return selected, nil
}

// PromptTransactionDate prompts for transaction date
func PromptTransactionDate() (string, error) {
	defaultDate := time.Now().Format("2006-01-02")

	date, err := PromptDate(
		"Transaction Date (YYYY-MM-DD):",
		defaultDate,
		"Press Enter for today",
		ValidateDateFormat(true),
	)
	if err != nil {
		return "", err
	}

	return date, nil
}

// filterAccounts returns accounts matching the given types and currency.
// allowedCurrency="" means no currency filter (all currencies pass).
// A non-empty allowedCurrency filters to exact-match only.
func filterAccounts(accounts []*model.Account, allowedTypes []string, allowedCurrency string) []*model.Account {
	parentIDs := make(map[int64]bool)
	for _, acc := range accounts {
		if acc.ParentID != nil {
			parentIDs[*acc.ParentID] = true
		}
	}

	typeMap := make(map[string]bool)
	for _, t := range allowedTypes {
		typeMap[t] = true
	}

	var result []*model.Account
	for _, acc := range accounts {
		if !typeMap[string(acc.Type)] || acc.IsHidden || parentIDs[acc.ID] {
			continue
		}
		if allowedCurrency != "" && acc.Currency != allowedCurrency {
			continue
		}
		result = append(result, acc)
	}
	return result
}

// PromptAccountSelection prompts for account selection with optional balance display.
// allowedCurrency filters to accounts with that exact currency ("" means system-default accounts).
func PromptAccountSelection(
	accounts []*model.Account,
	allowedTypes []string,
	message string,
	showBalance bool,
	balanceGetter func(int64) (string, error),
	allowedCurrency string,
) (string, error) {
	filteredAccounts := filterAccounts(accounts, allowedTypes, allowedCurrency)

	if len(filteredAccounts) == 0 {
		return "", fmt.Errorf("no available accounts (Type: %v)", allowedTypes)
	}

	var opts []huh.Option[string]
	accountMap := make(map[string]string) // display -> actual name

	for _, acc := range filteredAccounts {
		displayName := acc.Name

		if showBalance && balanceGetter != nil {
			balance, err := balanceGetter(acc.ID)
			if err == nil {
				displayName = fmt.Sprintf("%s (Balance: %s %s)", acc.Name, balance, acc.Currency)
			}
		}

		// key: displayName, value: displayName (we map back later)
		opts = append(opts, huh.NewOption(displayName, displayName))
		accountMap[displayName] = acc.Name
	}

	// Show selection prompt
	var selectedDisplay string

	err := huh.NewSelect[string]().
		Title(message).
		Options(opts...).
		Value(&selectedDisplay).
		Height(15). // Give it plenty of height for transactions
		Run()

	if err != nil {
		return "", err
	}

	return accountMap[selectedDisplay], nil
}
