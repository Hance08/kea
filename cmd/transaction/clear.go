package transaction

import (
	"fmt"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func NewClearCmd(svc *service.AccountingService) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <transaction-id>",
		Short: "Mark transaction as cleared",
		Long:  `Mark a pending transaction as cleared (confirmed).`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTransactionClear(svc, args)
		},
	}
}

func runTransactionClear(svc *service.AccountingService, args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	// Update status to cleared (1)
	if err := svc.UpdateTransactionStatus(txID, 1); err != nil {
		pterm.Error.Printf("Failed to update transaction status: %v\n", err)
		return nil
	}

	pterm.Success.Printf("Transaction #%d marked as cleared\n", txID)
	ui.Separator()
	return nil
}
