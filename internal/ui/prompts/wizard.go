package prompts

import (
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

func PromptInitCurrency(currDefault string) (string, error) {
	selectPrompt := &survey.Select{
		Message: "Welcome to Kea! This is the first execute, please set the default currency:",
		Options: []string{"USD", "TWD", "JPY", "EUR", "CNY", "Other"},
		Default: currDefault,
		Help:    "This default currency will help you to initialize the system account (Opening Balances)",
	}

	var selection string

	err := survey.AskOne(selectPrompt, &selection)
	if err != nil {
		return "", err
	}

	finalCurrency := selection
	if selection == "Other" {
		inputPrompt := &survey.Input{
			Message: "Please enter the currency code:",
			Help:    "Please use the ISO 4217 standard 3-letter currency code.",
		}

		var customInput string
		//TODO: improved the validator
		err := survey.AskOne(inputPrompt, &customInput, survey.WithValidator(survey.Required))
		if err != nil {
			return "", err
		}

		finalCurrency = strings.ToUpper(strings.TrimSpace(customInput))
	}

	return finalCurrency, nil
}
