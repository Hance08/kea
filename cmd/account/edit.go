// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package account

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func NewEditCmd(svc *service.Service) *cobra.Command {
	flags := &editFlags{}

	cmd := &cobra.Command{
		Use:     "edit <account-name>",
		Aliases: []string{"e"},
		Short:   "Edit an account's name, description, or hidden status.",
		Long: `Edit an account's name segment, description, or hidden status.

When renaming, only the last segment of the name changes — the parent path is preserved.
Renaming a parent account cascades to all its descendants automatically.

Example:
  kea account edit Assets:Bank --name Savings
  kea account edit Assets:Bank --desc "Main savings account"
  kea account edit Assets:Bank --hidden
  kea account edit Assets:Bank --no-hidden`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &editRunner{
				svc:  svc.Account(),
				view: views.NewCommonView(),
			}
			return runner.Run(cmd.Context(), args[0], flags, cmd)
		},
	}

	cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "New name segment (last part only)")
	cmd.Flags().StringVarP(&flags.Desc, "desc", "d", "", "New description")
	cmd.Flags().BoolVar(&flags.Hidden, "hidden", false, "Hide the account")
	cmd.Flags().BoolVar(&flags.NoHidden, "no-hidden", false, "Unhide the account")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "Output updated account as JSON")

	cmd.MarkFlagsMutuallyExclusive("hidden", "no-hidden")

	return cmd
}

func (r *editRunner) Run(ctx context.Context, accName string, flags *editFlags, cmd *cobra.Command) error {
	acc, err := r.svc.GetAccountByName(ctx, accName)
	if err != nil {
		return fmt.Errorf("account not found: %w", err)
	}

	if model.IsOpeningBalancesAccount(acc.Name) {
		return fmt.Errorf("account %q is a system account and cannot be edited: %w", acc.Name, service.ErrNotEditable)
	}

	hasFlags := cmd.Flags().Changed("name") || cmd.Flags().Changed("desc") ||
		cmd.Flags().Changed("hidden") || cmd.Flags().Changed("no-hidden")

	if flags.JSON && !hasFlags {
		return fmt.Errorf("--json requires at least one of: --name, --desc, --hidden, --no-hidden")
	}

	var input editInput
	if hasFlags {
		input, err = r.runFromFlags(flags, cmd)
	} else {
		input, err = r.runInteractive(ctx, acc)
	}
	if err != nil {
		return err
	}

	if input.newName == nil && input.description == nil && input.isHidden == nil {
		pterm.Info.Println("No changes made")
		return nil
	}

	finalName, err := r.applyChanges(ctx, acc, input)
	if err != nil {
		return err
	}

	if flags.JSON {
		updatedAcc, err := r.svc.GetAccountByName(ctx, finalName)
		if err != nil {
			return err
		}
		bal, err := r.svc.GetAccountBalance(ctx, updatedAcc.ID)
		if err != nil {
			return err
		}
		return views.WriteJSON(views.ToJSONAccount(updatedAcc, bal))
	}

	r.view.ShowSuccess("Account updated successfully")
	return nil
}
