package transaction

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type clearRunner struct {
	svc     *service.Service
	jsonOut bool
}

func NewClearCmd(svc *service.Service) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "clear <transaction-id>",
		Short: "Mark transaction as cleared",
		Long:  `Mark a pending transaction as cleared (confirmed).`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &clearRunner{svc: svc, jsonOut: jsonOut}
			return runner.Run(args)
		},
	}
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *clearRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	// Update status to cleared (1)
	if err := r.svc.Transaction().UpdateTransactionStatus(txID, 1); err != nil {
		if r.jsonOut {
			return err
		}
		pterm.Error.Printf("Failed to update transaction status: %v\n", err)
		return nil
	}

	if r.jsonOut {
		return views.WriteJSON(map[string]any{"id": txID, "status": model.StatusCleared.String()})
	}
	pterm.Success.Printf("Transaction (ID: %d) marked as cleared\n", txID)
	return nil
}
