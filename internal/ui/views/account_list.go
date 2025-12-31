package views

import (
	"os"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

type AccountListView struct{}

func NewAccountListView() *AccountListView {
	return &AccountListView{}
}

func (v *AccountListView) Render(accounts []*model.Account, balanceGetter func(int64) (string, error)) error {
	ui.PrintL1Title("Account List (Total: %d accounts)", len(accounts))
	pterm.Println()

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"Name", "Type", "Balance", "Cur"})

	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_CENTER,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_LEFT,
	})

	table.SetBorder(false)
	table.SetHeaderLine(true)
	table.SetAutoWrapText(false)

	for _, acc := range accounts {
		balance, _ := balanceGetter(acc.ID)
		balanceInt, err := strconv.ParseFloat(balance, 64)
		if err != nil {
			return err
		}
		rawStr := humanize.Commaf(balanceInt)
		balanceStr := strings.TrimPrefix(rawStr, "-")

		var coloredAccount, coloredType, coloredBalance string
		switch acc.Type {
		case "A", "R": // Assets, Revenue - Green
			coloredType = pterm.Green(acc.Type)
			coloredBalance = pterm.Green(balanceStr)
			coloredAccount = pterm.Green(acc.Name)
		case "L", "E": // Liabilities, Expenses - Red
			coloredType = pterm.Red(acc.Type)
			coloredBalance = pterm.Red(balanceStr)
			coloredAccount = pterm.Red(acc.Name)
		case "C": // Equity - Gray
			coloredType = pterm.Gray(acc.Type)
			coloredBalance = pterm.Gray(balanceStr)
			coloredAccount = pterm.Gray(acc.Name)
		default:
			coloredType = acc.Type
			coloredBalance = balanceStr
			coloredAccount = acc.Name
		}
		table.Append([]string{
			coloredAccount,
			coloredType,
			coloredBalance,
			acc.Currency,
		})
	}

	table.Render()

	return nil
}
