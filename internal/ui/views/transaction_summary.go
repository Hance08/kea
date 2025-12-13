package views

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/service"
	"github.com/pterm/pterm"
)

func RenderTransactionSummary(input service.TransactionInput) {
	pterm.DefaultSection.Println("Transaction Summary")

	date := time.Unix(input.Timestamp, 0).Format("2006-01-02")

	status := "Cleared"
	if input.Status == 0 {
		status = "Pending"
	}

	tableData := pterm.TableData{
		{"Field", "Value"},
		{"Date", date},
		{"Description", input.Description},
		{"Status", status},
	}

	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

	pterm.DefaultSection.Println("Splits (Double-Entry)")

	splitsData := pterm.TableData{
		{"Account", "Amount", "Type"},
	}

	var total int64
	for _, split := range input.Splits {
		// TODO: Change to currency package
		amountStr := fmt.Sprintf("%.2f", float64(split.Amount)/100.0)
		typeStr := "Debit"
		if split.Amount < 0 {
			typeStr = "Credit"
			amountStr = fmt.Sprintf("%.2f", float64(-split.Amount)/100.0)
		}
		splitsData = append(splitsData, []string{split.AccountName, amountStr, typeStr})
		total += split.Amount
	}

	pterm.DefaultTable.WithHasHeader().WithData(splitsData).Render()

	if total == 0 {
		pterm.Success.Println("✓ Splits balance verified (total = 0)")
	} else {
		pterm.Warning.Printf("⚠ Warning: Splits do not balance (total = %d)\n", total)
	}
}
