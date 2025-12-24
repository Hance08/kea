package views

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui"
	"github.com/hance08/kea/internal/utils"
	"github.com/pterm/pterm"
)

func RenderTransactionDetail(detail *service.TransactionDetail) error {
	date := time.Unix(detail.Timestamp, 0).Format("2005-01-02")
	status := "Pending"
	if detail.Status == 1 {
		status = "Cleared"
	}

	pterm.Println()
	ui.PrintL2Title("Transaction Info")
	infoData := pterm.TableData{
		{"Field", "Value"},
		{"ID", fmt.Sprintf("%d", detail.ID)},
		{"Date", date},
		{"Description", detail.Description},
		{"Status", status},
	}
	if err := pterm.DefaultTable.
		WithHasHeader().
		WithHeaderStyle(pterm.NewStyle(pterm.FgGray)).
		WithData(infoData).
		Render(); err != nil {
		return err
	}

	pterm.Println()
	ui.PrintL2Title("Splits")
	splitsData := pterm.TableData{
		{"Account", "Amount", "Type", "Memo"},
	}

	for _, split := range detail.Splits {
		amountStr := utils.FormatFromCents(split.Amount)
		fullAmountStr := fmt.Sprintf("%s %s", amountStr, split.Currency)

		typeStr := "Debit +"
		if split.Amount < 0 {
			typeStr = "Credit -"

			absAmount := -split.Amount
			fullAmountStr = fmt.Sprintf("%s %s", utils.FormatFromCents(absAmount), split.Currency)
		}

		accountName := split.AccountName
		if accountName == "" {
			accountName = fmt.Sprintf("[ID: %d]", split.AccountID)
		}

		memo := split.Memo
		if memo == "" {
			memo = "-"
		}

		splitsData = append(splitsData, []string{
			accountName,
			fullAmountStr,
			typeStr,
			memo,
		})
	}

	if err := pterm.DefaultTable.
		WithHasHeader().
		WithHeaderStyle(pterm.NewStyle(pterm.FgGray)).
		WithData(splitsData).
		Render(); err != nil {
		return err
	}

	return nil
}
