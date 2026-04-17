package cmd

import (
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/spf13/cobra"
)

// reconcileAccountProvider is the subset of AccountService used by the runner.
type reconcileAccountProvider interface {
	GetAccountByName(name string) (*model.Account, error)
}

// reconcileTxProvider is the subset of TransactionService used by the runner.
type reconcileTxProvider interface {
	GetUnreconciledByAccount(accountID int64) ([]*model.ReconcileEntry, int64, error)
	ReconcileTransactions(accountID int64, statementBalance int64, txIDs []int64) (int64, error)
}

type reconcileFlags struct {
	Balance string
	IDs     string
	Force   bool
	JSON    bool
}

type reconcileRunner struct {
	accSvc reconcileAccountProvider
	txSvc  reconcileTxProvider
	flags  *reconcileFlags
}

// NewReconcileCmd wires up the `kea reconcile` command.
func NewReconcileCmd(svc *service.Service) *cobra.Command {
	flags := &reconcileFlags{}

	cmd := &cobra.Command{
		Use:   "reconcile <account-name>",
		Short: "Reconcile an account against a statement balance",
		Long: `Compare your records against an external statement and mark
matching transactions as reconciled.

Interactive mode (default):
  kea reconcile "Assets:Checking"

Non-interactive / agent mode (all flags required):
  kea reconcile "Assets:Checking" --balance 2450.00 --ids 12,15,18 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &reconcileRunner{
				accSvc: svc.Account(),
				txSvc:  svc.Transaction(),
				flags:  flags,
			}
			return runner.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&flags.Balance, "balance", "", "statement ending balance (e.g. 2450.00)")
	cmd.Flags().StringVar(&flags.IDs, "ids", "", "comma-separated transaction IDs to reconcile (non-interactive)")
	cmd.Flags().BoolVar(&flags.Force, "force", false, "skip balance-mismatch warning (non-interactive mode)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")

	return cmd
}
