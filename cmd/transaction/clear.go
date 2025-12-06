package transaction

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// clearCmd marks a transaction as cleared
var clearCmd = &cobra.Command{
	Use:   "clear <transaction-id>",
	Short: "Mark transaction as cleared",
	Long:  `Mark a pending transaction as cleared (confirmed).`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTransactionClear,
}

func runTransactionClear(cmd *cobra.Command, args []string) error {
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
	printSeparator()
	return nil
}
