package account

import (
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/prompts"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type deleteRunner struct {
	svc     *service.Service
	yes     bool
	jsonOut bool
}

func NewDeleteCmd(svc *service.Service) *cobra.Command {
	var yes bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "delete <account-name>",
		Aliases: []string{"del", "d"},
		Short:   "Delete an account with no transactions",
		Long:    "Delete an account that has no transactions, no child accounts, and is not the system opening balance account.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &deleteRunner{svc: svc, yes: yes || jsonOut, jsonOut: jsonOut}
			return runner.Run(args[0])
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm deletion without interactive prompt")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "output result as JSON")

	return cmd
}

func (r *deleteRunner) Run(name string) error {
	acc, err := r.svc.Account().GetAccountByName(name)
	if err != nil {
		pterm.Error.Printf("Failed to delete account: %v\n", err)
		return nil
	}

	if !r.jsonOut {
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

	if err := r.svc.Account().DeleteAccountByName(acc.Name); err != nil {
		pterm.Error.Printf("Failed to delete account: %v\n", err)
		return nil
	}

	if r.jsonOut {
		return views.WriteJSON(map[string]any{"name": acc.Name, "deleted": true})
	}
	pterm.Success.Printf("Account %q deleted\n", acc.Name)
	return nil
}
