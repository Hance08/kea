package views

import (
	"fmt"
	"os"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

type AccountSummaryItem struct {
	FullName    string
	Type        model.AccountType
	Currency    string
	Balance     int64
	Description string
}

type AccountCreateView struct{}

func NewAccountCreateView() *AccountCreateView {
	return &AccountCreateView{}
}

func (v *AccountCreateView) RenderSummary(data AccountSummaryItem) error {
	balanceStr := utils.FormatAmount(data.Balance)

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
		{headerStyle.Sprint("Type"), string(data.Type)},
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

func (v *AccountCreateView) ShowSuccess(id int64, fullName string) error {
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
