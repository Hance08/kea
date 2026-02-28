package account

import (
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/ui/prompts"
)

func (r *createRunner) createAccount() (*model.Account, error) {

	return r.accSvc.CreateAccountWithBalance(
		r.fullName,
		r.accountType,
		r.currency,
		r.description,
		r.parentID,
		r.balanceCents,
	)

}

func (r *createRunner) applyTypeSettings(rootName, accType, currencyOverride string) error {
	r.fullName = r.accSvc.FormatAccountName(rootName, r.name)
	r.accountType = model.AccountType(accType)

	if currencyOverride != "" {
		if err := r.validator.ValidateCurrency(currencyOverride); err != nil {
			return err
		}
		r.currency = strings.ToUpper(strings.TrimSpace(currencyOverride))
	} else {
		r.currency = r.defaultCurrency
	}
	return nil
}

func (r *createRunner) applyParentSettings(parent *model.Account, currencyOverride string) {
	r.fullName = r.accSvc.FormatAccountName(parent.Name, r.name)
	r.accountType = parent.Type
	r.parentID = &parent.ID

	if currencyOverride != "" {
		r.currency = currencyOverride
	} else {
		r.currency = parent.Currency
	}
}

func (r *createRunner) buildFromParentName(parentName, currency string) error {
	parentAccount, err := r.accSvc.GetAccountByName(parentName)
	if err != nil {
		return err
	}

	r.applyParentSettings(parentAccount, currency)
	return nil
}

func (r *createRunner) buildFromTypeFlag(accType, currency string) error {
	rootName, err := r.accSvc.GetRootNameByType(accType)
	if err != nil {
		return fmt.Errorf("get root name: %w", err)
	}

	return r.applyTypeSettings(rootName, accType, currency)
}

func (r *createRunner) promptType() (string, error) {
	return prompts.PromptAccountType()
}

func (r *createRunner) promptParent() (*model.Account, error) {
	allAccounts, err := r.accSvc.GetAllAccounts()
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

func (r *createRunner) promptName(prefix string) (string, error) {
	surveyValidator := func(inputStr string) error {
		if err := r.validator.ValidateAccountName(inputStr); err != nil {
			return err
		}

		fullName := r.accSvc.FormatAccountName(prefix, inputStr)

		exists, err := r.accSvc.CheckAccountExists(fullName)
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

func (r *createRunner) promptCurrency() (string, error) {
	defaultCurrency := r.currency

	if defaultCurrency == "" {
		//TODO: Validate the string in the config file
		defaultCurrency = r.defaultCurrency
	}

	isInherited := r.parentID != nil

	return prompts.PromptCurrency(defaultCurrency, isInherited, r.validator.ValidateCurrency)

}

func (r *createRunner) promptBalance() (int64, error) {
	balanceStr, err := prompts.PromptInitialBalance(r.validator.ValidateInitialBalance)
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
