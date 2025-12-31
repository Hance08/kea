package views

import (
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

type TransactionDeletePreviewItem struct {
	ID          int64
	Timestamp   int64
	Description string
	SplitCount  int
}

func RenderTransactionDeletePreview(data TransactionDeletePreviewItem) error {
	date := time.Unix(data.Timestamp, 0).Format("2006-01-02")

	pterm.Warning.Printf("About to delete transaction (ID: %d)\n", data.ID)

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{})
	table.SetBorder(false)
	table.SetColumnSeparator(":")
	table.SetAutoWrapText(false)

	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
	})

	headerStyle := pterm.NewStyle(pterm.FgCyan, pterm.Bold)
	rows := [][]string{
		{headerStyle.Sprint("Date"), date},
		{headerStyle.Sprint("Description"), data.Description},
		{headerStyle.Sprint("Splits"), fmt.Sprint(data.SplitCount)},
	}

	table.AppendBulk(rows)
	table.Render()
	pterm.Println()
	pterm.Warning.Println("This action cannot be undone!")

	return nil
}

func RenderTransactionDeleteSuccess(id int64) {
	pterm.Success.Printf("Transaction (ID: %d) deleted successfully\n", id)
}
