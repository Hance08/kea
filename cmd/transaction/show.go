package transaction

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// showCmd shows details of a specific transaction
var showCmd = &cobra.Command{
	Use:   "show <transaction-id>",
	Short: "Show transaction details",
	Long:  `Display detailed information about a specific transaction including all splits.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTransactionShow,
}

func runTransactionShow(cmd *cobra.Command, args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	// Get transaction details
	detail, err := logic.GetTransactionByID(txID)
	if err != nil {
		pterm.Error.Printf("Failed to get transaction: %v\n", err)
		return nil
	}

	// Basic info table
	date := time.Unix(detail.Timestamp, 0).Format("2006-01-02")
	status := "Cleared"
	if detail.Status == 0 {
		status = "Pending"
	}

	pterm.DefaultSection.Println("Transaction Info")
	infoData := pterm.TableData{
		{"Field", "Value"},
		{"ID", fmt.Sprintf("%d", detail.ID)},
		{"Date", date},
		{"Description", detail.Description},
		{"Status", status},
	}

	pterm.DefaultTable.WithHasHeader().WithData(infoData).Render()

	// Splits table
	pterm.DefaultSection.Println("Splits (Double-Entry)")

	splitsData := pterm.TableData{
		{"Account", "Amount", "Type", "Memo"},
	}

	var total int64
	for _, split := range detail.Splits {
		amountStr := fmt.Sprintf("%.2f %s", float64(split.Amount)/100.0, split.Currency)
		typeStr := "Debit +"
		if split.Amount < 0 {
			typeStr = "Credit -"
			amountStr = fmt.Sprintf("%.2f %s", float64(-split.Amount)/100.0, split.Currency)
		}

		accountName := split.AccountName
		if accountName == "" {
			accountName = fmt.Sprintf("[ID: %d]", split.AccountID)
		}

		memo := split.Memo
		if memo == "" {
			memo = "-"
		}

		splitsData = append(splitsData, []string{
			accountName,
			amountStr,
			typeStr,
			memo,
		})

		total += split.Amount
	}

	pterm.DefaultTable.WithHasHeader().WithData(splitsData).Render()

	return nil
}
