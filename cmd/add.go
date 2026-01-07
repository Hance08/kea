package cmd

import (
	"fmt"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/spf13/cobra"
)

type addRunner struct {
	accSvc  AddProvider
	txSvc   TransactionProvider
	addView AddView
	flags   *addFlags
	cmd     *cobra.Command
}

func NewAddCmd(svc *service.Service) *cobra.Command {
	flags := &addFlags{}

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new transaction",
		Long: `Add a new transaction to your accounting system.

	This command allows you to record financial transactions using double-entry bookkeeping.
	You can use flags for quick entry or interactive mode for guided input.

	Examples:
	# Interactive mode (recommended for beginners)
	kea add

	# Quick mode with flags
	kea add --description "Buy Coffee" --amount 150 --from "Assets:Cash" --to "Expenses:Food:Coffee"
	
	# With pending status (default is cleared)
	kea add --description "Pending cost" --amount 500 --from "Assets:Bank" --to "Expenses:Shopping" --status pending`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &addRunner{
				accSvc:  svc.Account,
				txSvc:   svc.Transaction,
				addView: views.NewTransactionDetailView(),
				flags:   flags,
				cmd:     cmd,
			}
			return runner.Run()
		},
	}
	cmd.Flags().StringVarP(&flags.Description, "desc", "d", "", "Transaction description")
	cmd.Flags().StringVarP(&flags.Amount, "amount", "a", "", "Transaction amount (e.g., 150 or 150.50)")
	cmd.Flags().StringVarP(&flags.From, "from", "f", "", "Source account (where money comes from)")
	cmd.Flags().StringVarP(&flags.To, "to", "t", "", "Destination account (where money goes to)")
	cmd.Flags().StringVarP(&flags.Status, "status", "s", "cleared", "Transaction status: pending or cleared")
	cmd.Flags().StringVar(&flags.Timestamp, "date", "", "Transaction date (YYYY-MM-DD), default is today")

	return cmd
}

func (r *addRunner) Run() error {
	var input addTransactionInput
	var err error

	// Check if using flag mode or interactive mode
	hasFlags := r.cmd.Flags().Changed("desc") || r.cmd.Flags().Changed("amount") ||
		r.cmd.Flags().Changed("from") || r.cmd.Flags().Changed("to")

	if hasFlags {
		// Flag mode: validate all required flags
		input, err = r.runFromFlags()
	} else {
		// Interactive mode
		input, err = r.runInteractive()
	}
	if err != nil {
		return err
	}

	result, err := r.txSvc.CreateSimpleTransaction(
		input.FromAccountID,
		input.ToAccountID,
		input.AmountCents,
		input.Description,
		input.Timestamp,
		input.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// Display transaction summary
	if err := r.addView.Render(&result, true); err != nil {
		return err
	}

	return nil
}
