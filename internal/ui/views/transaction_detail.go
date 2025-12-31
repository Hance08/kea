package views

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

func RenderTransactionDetail(input *service.TransactionDetail, isCreate bool) error {
	ui.PrintL1Title("Transaction Summary (ID: %d)", input.ID)

	date := time.Unix(input.Timestamp, 0).Format("2006-01-02")

	status := "Cleared"
	if input.Status == 0 {
		status = "Pending"
	}

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
		{headerStyle.Sprint("Description"), input.Description},
		{headerStyle.Sprint("Status"), status},
	}

	table.AppendBulk(rows)
	table.Render()
	pterm.Println()

	ui.PrintL1Title("Splits")
	table = tablewriter.NewWriter(os.Stdout)

	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_LEFT,
	})

	table.SetHeader([]string{})
	table.SetBorder(false)
	table.SetColumnSeparator("|")
	table.SetAutoWrapText(false)

	var total int64
	for _, split := range input.Splits {
		// TODO: Change to currency package
		amountVal := float64(split.Amount) / 100.0
		rawStr := humanize.Commaf(amountVal)
		amountStr := strings.TrimPrefix(rawStr, "-")

		var displayAccount string
		var displayAmount string

		if split.Amount < 0 {
			displayAccount = fmt.Sprintf("    %s", split.AccountName)
			displayAmount = fmt.Sprintf("%s", amountStr)
		} else {
			displayAccount = split.AccountName
			displayAmount = fmt.Sprintf("%s", amountStr)
		}

		table.Append([]string{
			displayAccount,
			displayAmount,
		})

		total += split.Amount
	}

	table.Render()
	pterm.Println()

	if isCreate {
		if total == 0 {
			pterm.Success.Printf("Transaction created successfully! (ID: %d)\n", input.ID)
		} else {
			pterm.Warning.Printf("Warning: Splits do not balance (total = %d)\n", total)
		}
	}

	return nil
}
