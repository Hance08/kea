package views

import (
	"fmt"
	"os"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/hance08/kea/internal/service"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

func GetSplitRoleLabels(splits []service.SplitDetail, txType service.TransactionType) []string {
	labels := make([]string, len(splits))

	if len(splits) != 2 {
		for i := range labels {
			labels[i] = "account"
		}
		return labels
	}

	switch txType {
	case service.TxTypeExpense:

		if splits[0].Amount > 0 {
			labels[0] = "expense category"
			labels[1] = "payment account"
		} else {
			labels[0] = "payment account"
			labels[1] = "expense category"
		}

	case service.TxTypeIncome:

		if splits[0].Amount < 0 {
			labels[0] = "income source"
			labels[1] = "receiving account"
		} else {
			labels[0] = "receiving account"
			labels[1] = "income source"
		}

	case service.TxTypeTransfer:

		if splits[0].Amount > 0 {
			labels[0] = "receiving account"
			labels[1] = "source account"
		} else {
			labels[0] = "source account"
			labels[1] = "receiving account"
		}

	case service.TxTypeOpening:
		labels[0] = "account"
		labels[1] = "opening balance"

	default:
		labels[0] = "account"
		labels[1] = "account"
	}

	return labels
}

func RenderSimpleSplitList(splits []service.SplitDetail) {

	table := tablewriter.NewWriter(os.Stdout)

	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_LEFT,
	})

	table.SetHeader([]string{})
	table.SetBorder(false)
	table.SetColumnSeparator("|")
	table.SetAutoWrapText(false)

	for _, split := range splits {
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
	}

	table.Render()
	pterm.Println()
}
