package prompts

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/hance08/kea/internal/model"
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

	// Reuse our new PromptSelect
	selected, err := PromptSelect("Account Types:", options, "A - Assets")
	if err != nil {
		return "", fmt.Errorf("input cancelled: %w", err)
	}

	// Extract type code
	selectedType := strings.Split(selected, " ")[0]
	return selectedType, nil
}

// PromptParentAccount prompts for parent account with autocomplete
func PromptParentAccount(accounts []*model.Account) (string, *model.Account, error) {
	accountMap := make(map[string]*model.Account)
	var options []huh.Option[string]

	for _, acc := range accounts {
		accountMap[acc.Name] = acc
		options = append(options, huh.NewOption(acc.Name, acc.Name))
	}

	var selected string

	err := huh.NewSelect[string]().
		Title("Parent account FULL NAME:").
		Options(options...).
		Value(&selected).
		Height(10). // Show more options
		Run()

	if err != nil {
		return "", nil, fmt.Errorf("input cancelled: %w", err)
	}

	account, exists := accountMap[selected]
	if !exists {
		// Should not happen with Select, but good for safety
		return selected, nil, nil
	}

	return selected, account, nil
}

// PromptIsSubAccount asks if creating a subaccount
func PromptIsSubAccount() (bool, error) {
	return PromptConfirm("Is this a subaccount?", false)
}

// PromptAccountName prompts for account name with validation
func PromptAccountName(validator func(string) error) (string, error) {
	return PromptInput("Account Name:", "", validator)
}

// PromptCurrency prompts for currency selection with common options
func PromptCurrency(defaultCurrency string, isInherited bool, customValidator func(string) error) (string, error) {
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

	var message string
	if isInherited {
		message = fmt.Sprintf("Currency (inherited: %s):", defaultCurrency)
	} else {
		message = fmt.Sprintf("Currency (default: %s):", defaultCurrency)
	}

	selected, err := PromptSelect(message, commonCurrencies, defaultCurrency)
	if err != nil {
		return "", fmt.Errorf("input cancelled: %w", err)
	}

	if selected == "Other (Custom)" {
		customCurrency, err := PromptInput("Enter currency code:", "", customValidator)
		if err != nil {
			return "", fmt.Errorf("input cancelled: %w", err)
		}
		return strings.ToUpper(strings.TrimSpace(customCurrency)), nil
	}

	currencyCode := strings.Split(selected, " ")[0]
	return currencyCode, nil
}

// PromptInitialBalance prompts for initial balance with validation
func PromptInitialBalance(validator func(string) error) (string, error) {
	return PromptInput("Initial Balance (press Enter for 0):", "0", validator)
}
