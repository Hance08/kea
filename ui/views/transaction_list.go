package views

import (
	"fmt"
	"os"

	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

type TransactionListItem struct {
	ID          int64
	Date        string
	Type        string
	Account     string
	Offset      string
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

	table.SetHeader([]string{"ID", "Date", "Type", "Account", "Offset Account", "Description", "Amount", "Status"})

	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
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
		var coloredType, coloredAccount, coloredOffset, coloredAmount, coloredDescription string

		amountVal, err := utils.ParseAmount(item.Amount)
		if err != nil {
			return err
		}
		amountStr := utils.FormatAmount(amountVal)

		switch item.Type {
		case "Expense":
			coloredType = pterm.Red(item.Type)
			coloredAccount = pterm.Red(item.Account)
			coloredOffset = pterm.Red(item.Offset)
			coloredAmount = pterm.Red(amountStr)
			coloredDescription = item.Description
		case "Income":
			coloredType = pterm.Green(item.Type)
			coloredAccount = pterm.Green(item.Account)
			coloredOffset = pterm.Green(item.Offset)
			coloredAmount = pterm.Green(amountStr)
			coloredDescription = item.Description
		case "Transfer":
			coloredType = pterm.Blue(item.Type)
			coloredAccount = pterm.Blue(item.Account)
			coloredOffset = pterm.Blue(item.Offset)
			coloredAmount = pterm.Blue(amountStr)
			coloredDescription = item.Description
		default: // Other or Opening
			coloredType = pterm.Gray(item.Type)
			coloredAccount = pterm.Gray(item.Account)
			coloredOffset = pterm.Gray(item.Offset)
			coloredAmount = pterm.Gray(amountStr)
			coloredDescription = pterm.Gray(item.Description)
		}

		table.Append([]string{
			fmt.Sprintf("%d", item.ID),
			item.Date,
			coloredType,
			coloredAccount,
			coloredOffset,
			coloredDescription,
			coloredAmount,
			item.Status,
		})
	}

	table.Render()

	return nil
}

func (v *TransactionListView) ShowWarning(format string, a ...any) {
	pterm.Warning.Printf(format+"\n", a...)
}
