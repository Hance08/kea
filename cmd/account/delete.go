package account

import (
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/prompts"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type deleteRunner struct {
	svc *service.Service
	yes bool
}

func NewDeleteCmd(svc *service.Service) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "delete <account-name>",
		Aliases: []string{"del", "d"},
		Short:   "Delete an account with no transactions",
		Long:    "Delete an account that has no transactions, no child accounts, and is not the system opening balance account.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &deleteRunner{svc: svc, yes: yes}
			return runner.Run(args[0])
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm deletion without interactive prompt")

	return cmd
}

func (r *deleteRunner) Run(name string) error {
	acc, err := r.svc.Account().GetAccountByName(name)
	if err != nil {
		pterm.Error.Printf("Failed to delete account: %v\n", err)
		return nil
	}

	pterm.Info.Printf("Account: %s | Type: %s | Currency: %s | Hidden: %t\n", acc.Name, acc.Type, acc.Currency, acc.IsHidden)

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

	pterm.Success.Printf("Account %q deleted\n", acc.Name)
	return nil
}
