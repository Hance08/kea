// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package account

import (
	"context"
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/ui/prompts"
)

func (r *createRunner) createAccount(ctx context.Context, input createInput) (*model.Account, error) {
	return r.accSvc.CreateAccountWithBalance(
		ctx,
		input.fullName,
		input.accountType,
		input.currency,
		input.description,
		input.parentID,
		input.balanceCents,
	)
}

func (r *createRunner) applyTypeSettings(accType, currencyOverride string, input *createInput) error {
	input.accountType = model.AccountType(accType)
	if currencyOverride != "" {
		if err := r.accSvc.ValidateCurrency(currencyOverride); err != nil {
			return err
		}
		input.currency = strings.ToUpper(strings.TrimSpace(currencyOverride))
	} else {
		input.currency = r.defaultCurrency
	}
	return nil
}

func (r *createRunner) applyParentSettings(parent *model.Account, currencyOverride string, input *createInput) {
	input.accountType = parent.Type
	input.parentID = &parent.ID
	if currencyOverride != "" {
		input.currency = currencyOverride
	} else {
		input.currency = parent.Currency
	}
}

func (r *createRunner) buildFromParentName(ctx context.Context, parentName, currency string, input *createInput) error {
	parentAccount, err := r.accSvc.GetAccountByName(ctx, parentName)
	if err != nil {
		return err
	}
	r.applyParentSettings(parentAccount, currency, input)
	input.fullName = parentAccount.Name // prefix for FormatAccountName in runFromFlags
	return nil
}

func (r *createRunner) buildFromTypeFlag(accType, currency string, input *createInput) error {
	rootName, err := r.accSvc.GetRootNameByType(accType)
	if err != nil {
		return fmt.Errorf("get root name: %w", err)
	}
	if err := r.applyTypeSettings(accType, currency, input); err != nil {
		return err
	}
	input.fullName = rootName // prefix for FormatAccountName in runFromFlags
	return nil
}

func (r *createRunner) promptType() (string, error) {
	return prompts.PromptAccountType()
}

func (r *createRunner) promptParent(ctx context.Context) (*model.Account, error) {
	allAccounts, err := r.accSvc.GetAllAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve accounts: %w", err)
	}

	_, selectedAccount, err := prompts.PromptParentAccount(allAccounts)
	if err != nil {
		return nil, err
	}

	if selectedAccount == nil {
		return nil, fmt.Errorf("no account selected")
	}

	return selectedAccount, nil
}

func (r *createRunner) promptName(ctx context.Context, prefix string) (string, error) {
	surveyValidator := func(inputStr string) error {
		if err := r.accSvc.ValidateAccountName(inputStr); err != nil {
			return err
		}

		fullName := r.accSvc.FormatAccountName(prefix, inputStr)

		exists, err := r.accSvc.CheckAccountExists(ctx, fullName)
		if err != nil {
			return fmt.Errorf("failed to validate: %w", err)
		}
		if exists {
			return fmt.Errorf("account '%s' already exists", fullName)
		}
		return nil
	}
	return prompts.PromptAccountName(surveyValidator)
}

func (r *createRunner) promptCurrency(input createInput) (string, error) {
	defaultCurrency := input.currency
	if defaultCurrency == "" {
		defaultCurrency = r.defaultCurrency
	}
	isInherited := input.parentID != nil
	return prompts.PromptCurrency(defaultCurrency, isInherited, r.accSvc.ValidateCurrency)
}

func (r *createRunner) promptBalance() (int64, error) {
	balanceStr, err := prompts.PromptInitialBalance(prompts.ValidateInitialBalance)
	if err != nil {
		return 0, err
	}

	return utils.ParseAmount(balanceStr)
}

func (r *createRunner) promptDescription() (string, error) {
	return prompts.PromptDescription("Description (optional):", false)
}

func (r *createRunner) confirm() error {
	confirm, err := prompts.PromptConfirm("Proceed with account creation?", true)
	if err != nil {
		return err
	}

	if !confirm {
		return fmt.Errorf("account creation cancelled")
	}

	return nil
}
