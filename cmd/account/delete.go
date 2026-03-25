package account

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/prompts"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type AccountDeleteProvider interface {
	GetAccountByName(name string) (*model.Account, error)
	DeleteAccountByName(name string) error
}

type deleteFlags struct {
	Yes     bool
	JSONOut bool
}

type deleteRunner struct {
	svc  AccountDeleteProvider
	yes  bool
	json bool
}

func NewDeleteCmd(svc *service.Service) *cobra.Command {
	flags := &deleteFlags{}

	cmd := &cobra.Command{
		Use:     "delete <account-name>",
		Aliases: []string{"del", "d"},
		Short:   "Delete an account with no transactions",
		Long:    "Delete an account that has no transactions, no child accounts, and is not the system opening balance account.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &deleteRunner{svc: svc.Account(), yes: flags.Yes || flags.JSONOut, json: flags.JSONOut}
			return runner.Run(args[0])
		},
	}

	cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "confirm deletion without interactive prompt")
	cmd.Flags().BoolVarP(&flags.JSONOut, "json", "j", false, "output result as JSON")

	return cmd
}

func (r *deleteRunner) Run(name string) error {
	acc, err := r.svc.GetAccountByName(name)
	if err != nil {
		return fmt.Errorf("failed to find account: %w", err)
	}

	if !r.json {
		pterm.Info.Printf("Account: %s | Type: %s | Currency: %s | Hidden: %t\n", acc.Name, acc.Type, acc.Currency, acc.IsHidden)
	}

	if !r.yes {
		confirm, err := prompts.PromptConfirm("This will permanently delete the account. Continue?", false)
		if err != nil {
			return err
		}
		if !confirm {
			pterm.Info.Println("Deletion cancelled")
			return nil
		}
	}

	if err := r.svc.DeleteAccountByName(acc.Name); err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	if r.json {
		return views.WriteJSON(map[string]any{"name": acc.Name, "deleted": true})
	}
	pterm.Success.Printf("Account %q deleted\n", acc.Name)
	return nil
}
