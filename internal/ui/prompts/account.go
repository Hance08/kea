package prompts

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/ui"
)

// PromptAccountType prompts for account type selection
func PromptAccountType() (string, error) {
	options := []string{
		"A - Assets",
		"L - Liabilities",
		"R - Revenue",
		"E - Expenses",
		"C - Equity (Advanced)",
	}

	selected, err := PromptSelect("Account Types:", options, "A - Assets")
	if err != nil {
		return "", fmt.Errorf("input cancelled: %w", err)
	}

	// Extract type code (first character before " - ")
	selectedType := strings.Split(selected, " ")[0]
	return selectedType, nil
}

// PromptParentAccount prompts for parent account with autocomplete
func PromptParentAccount(accounts []*model.Account) (string, *model.Account, error) {
	var accountNames []string
	accountMap := make(map[string]*model.Account)

	for _, acc := range accounts {
		accountNames = append(accountNames, acc.Name)
		accountMap[acc.Name] = acc
	}

	var selected string
	prompt := &survey.Input{
		Message: "Parent account FULL NAME:",
		Suggest: func(toComplete string) []string {
			var filtered []string
			for _, name := range accountNames {
				if strings.Contains(strings.ToLower(name), strings.ToLower(toComplete)) {
					filtered = append(filtered, name)
				}
			}
			return filtered
		},
	}

	err := survey.AskOne(prompt, &selected, ui.IconOption())
	if err != nil {
		return "", nil, fmt.Errorf("input cancelled: %w", err)
	}

	account, exists := accountMap[selected]
	if !exists {
		return selected, nil, nil
	}

	return selected, account, nil
}

// PromptIsSubAccount asks if creating a subaccount
func PromptIsSubAccount() (bool, error) {
	confirm, err := PromptConfirm("Is this a subaccount?", false)
	if err != nil {
		return false, fmt.Errorf("input cancelled: %w", err)
	}
	return confirm, nil
}

// PromptAccountName prompts for account name with validation
func PromptAccountName(validator func(any) error) (string, error) {
	return PromptInput("Account Name:", "", validator)
}

// PromptCurrency prompts for currency selection with common options
func PromptCurrency(defaultCurrency string, isInherited bool, customValidator func(any) error) (string, error) {
	commonCurrencies := []string{
		"USD - US Dollar",
		"EUR - Euro",
		"GBP - British Pound",
		"JPY - Japanese Yen",
		"CNY - Chinese Yuan",
		"TWD - Taiwan Dollar",
		"HKD - Hong Kong Dollar",
		"SGD - Singapore Dollar",
		"Other (Custom)",
	}

	// Find default option in the list
	var defaultOption string
	for _, curr := range commonCurrencies {
		if strings.HasPrefix(curr, defaultCurrency) {
			defaultOption = curr
			break
		}
	}
	if defaultOption == "" {
		defaultOption = commonCurrencies[0]
	}

	var message string
	if isInherited {
		message = fmt.Sprintf("Currency (inherited: %s):", defaultCurrency)
	} else {
		message = fmt.Sprintf("Currency (default: %s):", defaultCurrency)
	}

	selected, err := PromptSelect(message, commonCurrencies, defaultOption)
	if err != nil {
		return "", fmt.Errorf("input cancelled: %w", err)
	}

	// If "Other (Custom)" is selected, ask for custom input
	if selected == "Other (Custom)" {
		customCurrency, err := PromptInput("Enter currency code:", "", customValidator)
		if err != nil {
			return "", fmt.Errorf("input cancelled: %w", err)
		}
		return strings.ToUpper(strings.TrimSpace(customCurrency)), nil
	}

	// Extract currency code from selection (first 3 characters)
	currencyCode := strings.Split(selected, " ")[0]
	return currencyCode, nil
}

// PromptInitialBalance prompts for initial balance with validation
func PromptInitialBalance(validator func(any) error) (string, error) {
	var balanceInput string
	prompt := &survey.Input{
		Message: "Initial Balance (press Enter for 0):",
		Default: "0",
	}

	err := survey.AskOne(prompt, &balanceInput, survey.WithValidator(validator), ui.IconOption())
	if err != nil {
		return "", fmt.Errorf("input cancelled: %w", err)
	}

	return balanceInput, nil
}
