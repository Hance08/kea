package views

import (
	"fmt"
	"os"

	"github.com/hance08/kea/ui"
	"github.com/hance08/kea/internal/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

type TransactionListItem struct {
	ID          int64
	Date        string
	Type        string
	Account     string
	Description string
	Amount      string
	Currency    string
	Status      string
}

type TransactionListView struct{}

func NewTransactionListView() *TransactionListView {
	return &TransactionListView{}
}

func (v *TransactionListView) Render(items []TransactionListItem, limit int) error {
	if len(items) == 0 {
		pterm.Warning.Println("No transactions found")
		return nil
	}

	ui.PrintL1Title("Showing recent transactions (limit: %d)", limit)
	pterm.Println()

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"ID", "Date", "Type", "Account", "Description", "Amount", "Status"})

	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_LEFT,
	})

	table.SetBorder(false)
	table.SetHeaderLine(true)
	table.SetAutoWrapText(false)

	for _, item := range items {
		var coloredType, coloredAccount, coloredAmount, coloredDescription string

		amountVal, err := utils.ParseAmount(item.Amount)
		if err != nil {
			return err
		}
		amountStr := utils.FormatAmount(amountVal)

		switch item.Type {
		case "Expense":
			coloredType = pterm.Red(item.Type)
			coloredAccount = pterm.Red(item.Account)
			coloredAmount = pterm.Red(amountStr)
			coloredDescription = item.Description
		case "Income":
			coloredType = pterm.Green(item.Type)
			coloredAccount = pterm.Green(item.Account)
			coloredAmount = pterm.Green(amountStr)
			coloredDescription = item.Description
		case "Transfer":
			coloredType = pterm.Blue(item.Type)
			coloredAccount = pterm.Blue(item.Account)
			coloredAmount = pterm.Blue(amountStr)
			coloredDescription = item.Description
		default: // Other or Opening
			coloredType = pterm.Gray(item.Type)
			coloredAccount = pterm.Gray(item.Account)
			coloredAmount = pterm.Gray(amountStr)
			coloredDescription = pterm.Gray(item.Description)
		}

		table.Append([]string{
			fmt.Sprintf("%d", item.ID),
			item.Date,
			coloredType,
			coloredAccount,
			coloredDescription,
			coloredAmount,
			item.Status,
		})
	}

	table.Render()

	return nil
}

func (v *TransactionListView) ShowWarning(format string, a ...interface{}) {
	pterm.Warning.Printf(format+"\n", a...)
}
