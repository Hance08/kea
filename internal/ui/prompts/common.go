package prompts

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

// PromptDescription prompts for a description text
// Can be used for transactions, accounts, or any other entity
func PromptDescription(message string, required bool) (string, error) {
	var desc string

	input := huh.NewInput().
		Title(message).
		Value(&desc)

	if required {
		input.Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("description is required")
			}
			return nil
		})
	}

	err := input.Run()
	return desc, err
}

// PromptAmount prompts for an amount with custom validation
func PromptAmount(message string, helpText string, validator func(string) error) (string, error) {
	var amount string

	input := huh.NewInput().
		Title(message).
		Description(helpText).
		Value(&amount)

	if validator != nil {
		input.Validate(validator)
	}

	err := input.Run()
	return amount, err
}

// PromptConfirm prompts for yes/no confirmation
func PromptConfirm(message string, defaultValue bool) (bool, error) {
	confirm := defaultValue

	err := huh.NewConfirm().
		Title(message).
		Value(&confirm).
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()

	return confirm, err
}

// PromptDate prompts for a date in YYYY-MM-DD format
func PromptDate(message string, defaultDate string, helpText string) (string, error) {
	var date string

	// Use Input for date for now (huh has no specialized date picker yet, simpler to stick to input)
	err := huh.NewInput().
		Title(message).
		Description(helpText).
		Placeholder(defaultDate). // Placeholder shows the default hint
		Value(&date).
		Run()

	if err != nil {
		return "", err
	}

	// If user pressed enter without typing, use the placeholder/default
	if date == "" {
		return defaultDate, nil
	}
	return date, nil
}

// PromptInput prompts for a generic text input with optional default and validator
func PromptInput(message string, defaultValue string, validator func(string) error) (string, error) {
	var inputVal string

	input := huh.NewInput().
		Title(message).
		Value(&inputVal)

	if defaultValue != "" {
		input.Placeholder(defaultValue)
	}

	if validator != nil {
		input.Validate(validator)
	}

	err := input.Run()
	if err != nil {
		return "", err
	}

	if inputVal == "" && defaultValue != "" {
		return defaultValue, nil
	}

	return inputVal, nil
}

// PromptSelect prompts for a selection from a list of options
func PromptSelect(message string, options []string, defaultOption string) (string, error) {
	realDefault := defaultOption
	matchFound := false

	for _, o := range options {
		if o == defaultOption {
			realDefault = o
			matchFound = true
			break
		}
	}

	if !matchFound && defaultOption != "" {
		for _, o := range options {
			if strings.HasPrefix(o, defaultOption+" ") {
				realDefault = o
				break
			}
		}
	}
	selected := realDefault

	// Create options for huh
	var opts []huh.Option[string]
	for _, o := range options {
		opts = append(opts, huh.NewOption(o, o))
	}

	selectField := huh.NewSelect[string]().
		Title(message).
		Options(opts...).
		Value(&selected)

	err := selectField.Run()
	return selected, err
}
