package views

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

type AccountSummaryItem struct {
	FullName    string
	Type        string
	Currency    string
	Balance     int64
	Description string
}

func RenderAccountSummary(data AccountSummaryItem) error {
	balanceStr := fmt.Sprintf("%.2f", float64(data.Balance)/100)

	descStr := data.Description
	if descStr == "" {
		descStr = "-"
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
		{headerStyle.Sprint("Full Name"), data.FullName},
		{headerStyle.Sprint("Type"), data.Type},
		{headerStyle.Sprint("Currency"), data.Currency},
		{headerStyle.Sprint("Balance"), balanceStr},
		{headerStyle.Sprint("Description"), descStr},
	}

	table.AppendBulk(rows)
	pterm.Println()
	table.Render()
	pterm.Println()

	return nil
}

func RenderAccountSuccess(id int64, fullName string) error {
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
		{headerStyle.Sprint("Account ID "), fmt.Sprintf("%d", id)},
		{headerStyle.Sprint("Full Name"), fullName},
	}

	table.AppendBulk(rows)
	table.Render()
	pterm.Println()
	pterm.Success.Print("Account created successfully!\n")

	return nil
}
