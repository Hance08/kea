package views

import (
	"fmt"

	"github.com/hance08/kea/internal/store"
	"github.com/pterm/pterm"
)

type AccountListView struct{}

func NewAccountListView() *AccountListView {
	return &AccountListView{}
}

func (v *AccountListView) Render(accounts []*store.Account, balanceGetter func(int64) (string, error)) error {
	headers := []string{"Name", "Type", "Balance"}
	tableData := pterm.TableData{headers}

	for _, acc := range accounts {
		balance, _ := balanceGetter(acc.ID)
		balanceWithCurrency := fmt.Sprintf("%s %s", balance, acc.Currency)

		var coloredAccount, coloredType, coloredBalance string
		switch acc.Type {
		case "A", "R": // Assets, Revenue - Green
			coloredType = pterm.Green(acc.Type)
			coloredBalance = pterm.Green(balanceWithCurrency)
			coloredAccount = pterm.Green(acc.Name)
		case "L", "E": // Liabilities, Expenses - Red
			coloredType = pterm.Red(acc.Type)
			coloredBalance = pterm.Red(balanceWithCurrency)
			coloredAccount = pterm.Red(acc.Name)
		case "C": // Equity - Gray
			coloredType = pterm.Gray(acc.Type)
			coloredBalance = pterm.Gray(balanceWithCurrency)
			coloredAccount = pterm.Gray(acc.Name)
		default:
			coloredType = acc.Type
			coloredBalance = balanceWithCurrency
			coloredAccount = acc.Name
		}
		tableData = append(tableData, []string{coloredAccount, coloredType, coloredBalance})
	}

	pterm.DefaultSection.Printf("Account List")
	if err := pterm.DefaultTable.WithHasHeader().WithData(tableData).Render(); err != nil {
		return err
	}

	pterm.Info.Printf("Total: %d accounts\n", len(accounts))

	return nil
}
