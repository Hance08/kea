package transaction

import (
	"fmt"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func NewDeleteCmd(svc *service.AccountingService) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <transaction-id>",
		Short: "Delete a transaction",
		Long:  `Delete a transaction and all its associated splits. This action cannot be undone.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTransactionDelete(svc, args)
		},
	}
}

func runTransactionDelete(svc *service.AccountingService, args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	// Get transaction details first to show what will be deleted
	detail, err := svc.GetTransactionByID(txID)
	if err != nil {
		pterm.Error.Printf("Failed to delete transaction: %v\n", err)
		return nil
	}
	if detail.ID == 1 {
		pterm.Error.Println("Can't delete the opening transaction")
		return nil
	}

	// Show transaction summary
	date := time.Unix(detail.Timestamp, 0).Format("2006-01-02")
	pterm.Warning.Printf("About to delete transaction #%d:\n", detail.ID)
	deletionInfo := pterm.TableData{
		{"Date", date},
		{"Description", detail.Description},
		{"Splits", fmt.Sprint(len(detail.Splits))},
	}

	pterm.DefaultTable.WithData(deletionInfo).Render()

	// Confirm deletion
	pterm.Warning.Println("This action cannot be undone!")

	var confirmation bool
	confirmPrompt := &survey.Confirm{
		Message: "Do you want to delete this transaction?",
		Default: false,
	}
	if err := survey.AskOne(confirmPrompt, &confirmation, ui.IconOption()); err != nil {
		return err
	}

	if !confirmation {
		pterm.Info.Println("Deletion cancelled")
		return nil
	}

	// Delete transaction
	if err := svc.DeleteTransaction(txID); err != nil {
		pterm.Error.Printf("Failed to delete transaction: %v\n", err)
		return nil
	}

	pterm.Success.Printf("Transaction #%d deleted successfully\n", txID)
	ui.Separator()
	return nil
}
