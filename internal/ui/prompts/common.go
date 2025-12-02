package prompts

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/hance08/kea/internal/ui"
)

// PromptDescription prompts for a description text
// Can be used for transactions, accounts, or any other entity
func PromptDescription(message string, required bool) (string, error) {
	var desc string
	prompt := &survey.Input{
		Message: message,
	}

	opts := []survey.AskOpt{ui.IconOption()}
	if required {
		opts = append(opts, survey.WithValidator(survey.Required))
	}

	err := survey.AskOne(prompt, &desc, opts...)
	return desc, err
}

// PromptAmount prompts for an amount with custom validation
func PromptAmount(message string, helpText string, validator func(any) error) (string, error) {
	var amount string
	prompt := &survey.Input{
		Message: message,
		Help:    helpText,
	}

	opts := []survey.AskOpt{ui.IconOption()}
	if validator != nil {
		opts = append(opts, survey.WithValidator(validator))
	}

	err := survey.AskOne(prompt, &amount, opts...)
	return amount, err
}

// PromptConfirm prompts for yes/no confirmation
func PromptConfirm(message string, defaultValue bool) (bool, error) {
	var confirm bool
	prompt := &survey.Confirm{
		Message: message,
		Default: defaultValue,
	}

	err := survey.AskOne(prompt, &confirm, ui.IconOption())
	return confirm, err
}

// PromptDate prompts for a date in YYYY-MM-DD format
func PromptDate(message string, defaultDate string, helpText string) (string, error) {
	var date string
	prompt := &survey.Input{
		Message: message,
		Default: defaultDate,
		Help:    helpText,
	}

	err := survey.AskOne(prompt, &date, ui.IconOption())
	return date, err
}

// PromptInput prompts for a generic text input with optional default and validator
func PromptInput(message string, defaultValue string, validator func(any) error) (string, error) {
	var input string
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
	}

	opts := []survey.AskOpt{ui.IconOption()}
	if validator != nil {
		opts = append(opts, survey.WithValidator(validator))
	}

	err := survey.AskOne(prompt, &input, opts...)
	return input, err
}

// PromptSelect prompts for a selection from a list of options
func PromptSelect(message string, options []string, defaultOption string) (string, error) {
	var selected string
	prompt := &survey.Select{
		Message: message,
		Options: options,
		Default: defaultOption,
	}

	err := survey.AskOne(prompt, &selected, ui.IconOption())
	return selected, err
}
