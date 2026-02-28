package prompts

import (
	"errors"
	"strings"

	"github.com/charmbracelet/huh"
)

func PromptInitCurrency(currDefault string) (string, error) {
	selection := currDefault

	err := huh.NewSelect[string]().
		Title("Welcome to Kea! This is the first execute, please set the default currency:").
		Description("This default currency will help you to initialize the system account (Opening Balances)").
		Options(
			huh.NewOption("USD", "USD"),
			huh.NewOption("TWD", "TWD"),
			huh.NewOption("JPY", "JPY"),
			huh.NewOption("EUR", "EUR"),
			huh.NewOption("CNY", "CNY"),
			huh.NewOption("Other", "Other"),
		).
		Value(&selection).
		Run()

	if err != nil {
		return "", err
	}

	finalCurrency := selection
	if selection == "Other" {
		var customInput string
		err := huh.NewInput().
			Title("Please enter the currency code:").
			Description("Please use the ISO 4217 standard 3-letter currency code.").
			Value(&customInput).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("currency code is required") // 修正這裡
				}
				return nil
			}).
			Run()

		if err != nil {
			return "", err
		}

		finalCurrency = strings.ToUpper(strings.TrimSpace(customInput))
	}

	return finalCurrency, nil
}
