package cmd

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type addRunner struct {
	accSvc AddProvider
	txSvc  TransactionProvider
	view   AddView
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
  kea add (recommended for beginners)

  kea add --desc "Buy Coffee" --amount 150 --from "Assets:Cash" --to "Expenses:Food:Coffee"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &addRunner{
				accSvc: svc.Account(),
				txSvc:  svc.Transaction(),
				view:   views.NewTransactionDetailView(),
			}

			return runner.Run(cmd.Context(), flags, cmd)
		},
	}
	cmd.Flags().StringVarP(&flags.Description, "desc", "d", "", "Transaction description")
	cmd.Flags().StringVarP(&flags.Amount, "amount", "a", "", "Transaction amount (e.g., 150 or 150.50)")
	cmd.Flags().StringVarP(&flags.From, "from", "f", "", "Source account (where money comes from)")
	cmd.Flags().StringVarP(&flags.To, "to", "t", "", "Destination account (where money goes to)")
	cmd.Flags().StringVarP(&flags.Status, "status", "s", "cleared", "Transaction status: pending or cleared")
	cmd.Flags().StringVar(&flags.Timestamp, "date", "", "Transaction date (YYYY-MM-DD), default is today")
	cmd.Flags().StringVar(&flags.Type, "type", "", "Transaction type: expense, income, transfer")
	cmd.Flags().StringArrayVar(&flags.Splits, "split", nil, "Split as AccountName=amount, e.g. Assets:Bank=-1000 (repeatable)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output created transaction as JSON")

	cmd.MarkFlagsMutuallyExclusive("split", "from")
	cmd.MarkFlagsMutuallyExclusive("split", "to")
	cmd.MarkFlagsMutuallyExclusive("split", "amount")

	return cmd
}

func (r *addRunner) Run(ctx context.Context, flags *addFlags, cmd *cobra.Command) error {
	var input addTransactionInput
	var err error

	// Check if using flag mode or interactive mode
	hasFlags := cmd.Flags().Changed("desc") || cmd.Flags().Changed("amount") ||
		cmd.Flags().Changed("from") || cmd.Flags().Changed("to") ||
		cmd.Flags().Changed("type") || cmd.Flags().Changed("split")

	if flags.JSON && !hasFlags {
		return fmt.Errorf("--json requires flag mode: use --amount/--from/--to or --split")
	}

	if hasFlags {
		// Flag mode: validate all required flags
		input, err = r.runFromFlags(ctx, flags)
	} else {
		// Interactive mode
		input, err = r.runInteractive(ctx)
	}
	if err != nil {
		return err
	}

	var result model.TransactionDetail
	if len(input.Splits) > 0 {
		result, err = r.txSvc.CreateTransactionFromSplits(
			ctx, input.Splits, input.Description, input.Timestamp, input.Status, input.Type,
		)
	} else {
		result, err = r.txSvc.CreateSimpleTransaction(
			ctx,
			input.FromAccountID,
			input.ToAccountID,
			input.AmountCents,
			input.Description,
			input.Timestamp,
			input.Status,
			input.Type,
		)
	}
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// Display transaction summary
	if flags.JSON {
		return views.WriteJSON(views.ToJSONTxDetail(&result))
	}
	return r.view.Render(&result, true)
}
