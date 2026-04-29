package transaction

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type ClearProvider interface {
	UpdateTransactionStatus(ctx context.Context, txID int64, status model.TransactionStatus) error
}

type clearFlags struct {
	JSON bool
}

type clearRunner struct {
	svc  ClearProvider
	json bool
}

func NewClearCmd(svc *service.Service) *cobra.Command {
	flags := &clearFlags{}
	cmd := &cobra.Command{
		Use:   "clear <transaction-id>",
		Short: "Mark transaction as cleared",
		Long:  `Mark a pending transaction as cleared (confirmed).`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &clearRunner{svc: svc.Transaction(), json: flags.JSON}
			return runner.Run(cmd.Context(), args)
		},
	}
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *clearRunner) Run(ctx context.Context, args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	if err := r.svc.UpdateTransactionStatus(ctx, txID, model.StatusCleared); err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	if r.json {
		return views.WriteJSON(map[string]any{"id": txID, "status": model.StatusCleared.String()})
	}
	pterm.Success.Printf("Transaction (ID: %d) marked as cleared\n", txID)
	return nil
}
