package views

import (
	"os"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

type TransactionDetailView struct {
	splitsView *TransactionSplitsView
}

func NewTransactionDetailView() *TransactionDetailView {
	return &TransactionDetailView{
		splitsView: NewTransactionSplitsView(),
	}
}

func (v *TransactionDetailView) Render(input *model.TransactionDetail, isCreate bool) error {
	ui.PrintL1Title("Transaction Summary (ID: %d)", input.ID)

	date := time.Unix(input.Timestamp, 0).Format("2006-01-02")

	status := input.Status.String()

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
	v.splitsView.Render(input.Splits)

	if isCreate {
		var total int64
		for _, split := range input.Splits {
			total += split.Amount
		}

		if total == 0 {
			pterm.Success.Printf("Transaction created successfully! (ID: %d)\n", input.ID)
		} else {
			pterm.Warning.Printf("Warning: Splits do not balance (total = %d)\n", total)
		}
	}

	return nil
}
