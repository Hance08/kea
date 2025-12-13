package views

import (
	"fmt"

	"github.com/hance08/kea/internal/ui"
	"github.com/pterm/pterm"
)

type AccountSummaryItem struct {
	FullName    string
	Type        string
	Currency    string
	Balance     int64
	Description string
}

func RenderAccountSummary(data AccountSummaryItem) {
	ui.Separator()

	balanceStr := fmt.Sprintf("%.2f", float64(data.Balance)/100)

	descStr := data.Description
	if descStr == "" {
		descStr = "None"
	}

	tableData := pterm.TableData{
		{pterm.Blue("Full Name"), data.FullName},
		{pterm.Blue("Type"), data.Type},
		{pterm.Blue("Currency"), data.Currency},
		{pterm.Blue("Balance"), balanceStr},
		{pterm.Blue("Description"), descStr},
	}

	pterm.DefaultTable.WithData(tableData).Render()
}

func RenderAccountSuccess(id int64, fullName string) {
	ui.Separator()

	tableData := pterm.TableData{
		{pterm.Blue("Account ID"), fmt.Sprintf("%d", id)},
		{pterm.Blue("Full Name"), fullName},
	}

	pterm.DefaultTable.WithData(tableData).Render()
	pterm.Success.Print("Account created successfully!\n")
}
