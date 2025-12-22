package views

import (
	"fmt"

	"github.com/pterm/pterm"
)

type TransactionListItem struct {
	ID          int64
	Date        string
	Type        string
	Account     string
	Description string
	Amount      string
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

	pterm.DefaultSection.Printf("Showing recent transactions (limit: %d)", limit)

	tableData := pterm.TableData{
		{"ID", "Date", "Type", "Account", "Description", "Amount", "Status"},
	}

	for _, item := range items {
		var coloredType, coloredAccount, coloredAmount string

		switch item.Type {
		case "Expense":
			coloredType = pterm.Red(item.Type)
			coloredAccount = pterm.Red(item.Account)
			coloredAmount = pterm.Red(item.Amount)
		case "Income":
			coloredType = pterm.Green(item.Type)
			coloredAccount = pterm.Green(item.Account)
			coloredAmount = pterm.Green(item.Amount)
		case "Transfer":
			coloredType = pterm.Blue(item.Type)
			coloredAccount = pterm.Blue(item.Account)
			coloredAmount = pterm.Blue(item.Amount)
		default: // Other or Opening
			coloredType = item.Type
			coloredAccount = item.Account
			coloredAmount = item.Amount
		}

		tableData = append(tableData, []string{
			fmt.Sprintf("%d", item.ID),
			item.Date,
			coloredType,
			coloredAccount,
			item.Description,
			coloredAmount,
			item.Status,
		})
	}

	if err := pterm.DefaultTable.WithHasHeader().WithData(tableData).Render(); err != nil {
		return err
	}
	pterm.Info.Printf("Total: %d transactions\n", len(items))
	return nil
}
