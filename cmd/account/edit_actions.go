package account

import (
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/ui/prompts"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func (r *editRunner) runFromFlags(acc *model.Account, flags *editFlags, cmd *cobra.Command) (editInput, error) {
	var input editInput

	if flags.Hidden && flags.NoHidden {
		return editInput{}, fmt.Errorf("--hidden and --no-hidden cannot both be set")
	}

	if cmd.Flags().Changed("name") {
		if err := r.svc.ValidateAccountName(flags.Name); err != nil {
			return editInput{}, fmt.Errorf("invalid name: %w", err)
		}
		input.newName = &flags.Name
	}

	if cmd.Flags().Changed("desc") {
		input.description = &flags.Desc
	}

	if cmd.Flags().Changed("hidden") {
		v := true
		input.isHidden = &v
	}

	if cmd.Flags().Changed("no-hidden") {
		v := false
		input.isHidden = &v
	}

	return input, nil
}

func (r *editRunner) runInteractive(acc *model.Account) (editInput, error) {
	var input editInput

	currentSegment := acc.Name
	if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
		currentSegment = acc.Name[idx+1:]
	}

	newSegment, err := r.promptNameSegment(currentSegment, acc.Name)
	if err != nil {
		return editInput{}, err
	}
	if newSegment != currentSegment {
		input.newName = &newSegment
	}

	newDesc, err := prompts.PromptInput(
		fmt.Sprintf("Description (current: %q):", acc.Description),
		acc.Description,
		nil,
	)
	if err != nil {
		return editInput{}, err
	}
	if newDesc != acc.Description {
		input.description = &newDesc
	}

	newHidden, err := prompts.PromptConfirm(
		fmt.Sprintf("Hide account? (current: %v)", acc.IsHidden),
		acc.IsHidden,
	)
	if err != nil {
		return editInput{}, err
	}
	if newHidden != acc.IsHidden {
		input.isHidden = &newHidden
	}

	if input.newName == nil && input.description == nil && input.isHidden == nil {
		return editInput{}, nil
	}

	r.showChangeSummary(acc, input)

	confirmed, err := prompts.PromptConfirm("Apply these changes?", true)
	if err != nil {
		return editInput{}, err
	}
	if !confirmed {
		return editInput{}, fmt.Errorf("edit cancelled")
	}

	return input, nil
}

func (r *editRunner) promptNameSegment(currentSegment, currentFullName string) (string, error) {
	prefix := ""
	if idx := strings.LastIndex(currentFullName, ":"); idx >= 0 {
		prefix = currentFullName[:idx]
	}

	validator := func(s string) error {
		if s == currentSegment {
			return nil
		}
		if err := r.svc.ValidateAccountName(s); err != nil {
			return err
		}
		var newFullName string
		if prefix != "" {
			newFullName = prefix + ":" + s
		} else {
			newFullName = s
		}
		exists, err := r.svc.CheckAccountExists(newFullName)
		if err != nil {
			return fmt.Errorf("failed to check existence: %w", err)
		}
		if exists {
			return fmt.Errorf("account %q already exists", newFullName)
		}
		return nil
	}

	return prompts.PromptInput(
		fmt.Sprintf("Name segment (prefix: %s):", currentFullName),
		currentSegment,
		validator,
	)
}

func (r *editRunner) showChangeSummary(acc *model.Account, input editInput) {
	if input.newName != nil {
		newFullName := *input.newName
		if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
			newFullName = acc.Name[:idx+1] + *input.newName
		}
		pterm.Info.Printf("Name:        %s → %s\n", acc.Name, newFullName)
	}
	if input.description != nil {
		pterm.Info.Printf("Description: %q → %q\n", acc.Description, *input.description)
	}
	if input.isHidden != nil {
		pterm.Info.Printf("Hidden:      %v → %v\n", acc.IsHidden, *input.isHidden)
	}
}

func (r *editRunner) applyChanges(acc *model.Account, input editInput) (string, error) {
	finalName := acc.Name

	if input.newName != nil {
		if err := r.svc.RenameAccount(acc.Name, *input.newName); err != nil {
			return "", fmt.Errorf("failed to rename account: %w", err)
		}
		if idx := strings.LastIndex(acc.Name, ":"); idx >= 0 {
			finalName = acc.Name[:idx+1] + *input.newName
		} else {
			finalName = *input.newName
		}
	}

	if input.description != nil || input.isHidden != nil {
		desc := acc.Description
		hidden := acc.IsHidden
		if input.description != nil {
			desc = *input.description
		}
		if input.isHidden != nil {
			hidden = *input.isHidden
		}
		if err := r.svc.UpdateAccountMetadata(acc.ID, desc, hidden); err != nil {
			return "", fmt.Errorf("failed to update account: %w", err)
		}
	}

	return finalName, nil
}
