package account

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/spf13/cobra"
)

type listFlags struct {
	Type       string
	ShowHidden bool
}

type listRunner struct {
	svc   *service.Service
	flags *listFlags
}

func NewListCmd(svc *service.Service) *cobra.Command {
	flags := &listFlags{}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "List all accounts with their balances.",
		Long: `List all accounts in the system with their current balances.
You can filter by account type or show hidden accounts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &listRunner{
				svc:   svc,
				flags: flags,
			}
			return runner.Run()
		},
	}

	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "Filter accounts by type (A, L, C, R, E)")
	cmd.Flags().BoolVar(&flags.ShowHidden, "show-hidden", false, "Show hidden accounts")

	return cmd
}

func (r *listRunner) Run() error {

	var accounts []*model.Account
	var err error

	if r.flags.Type != "" {
		accounts, err = r.svc.Account.GetAccountsByType(r.flags.Type)
	} else {
		accounts, err = r.svc.Account.GetAllAccounts()
	}

	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	if !r.flags.ShowHidden {
		accounts = r.filterHiddenAccounts(accounts)
	}

	if err := views.NewAccountListView().Render(accounts, r.svc.Account.GetAccountBalanceFormatted); err != nil {
		return err
	}

	return nil
}

func (r *listRunner) filterHiddenAccounts(accounts []*model.Account) []*model.Account {
	var filtered []*model.Account
	for _, acc := range accounts {
		if !acc.IsHidden {
			filtered = append(filtered, acc)
		}
	}
	return filtered
}
