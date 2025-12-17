package views

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/utils"
	"github.com/pterm/pterm"
)

func RenderTransactionDetail(detail *service.TransactionDetail) {
	date := time.Unix(detail.Timestamp, 0).Format("2005-01-02")
	status := "Pending"
	if detail.Status == 1 {
		status = "Cleared"
	}

	pterm.DefaultSection.Println("Transaction Info")
	infoData := pterm.TableData{
		{"Field", "Value"},
		{"ID", fmt.Sprintf("%d", detail.ID)},
		{"Date", date},
		{"Description", detail.Description},
		{"Status", status},
	}
	pterm.DefaultTable.WithHasHeader().WithData(infoData).Render()

	pterm.DefaultSection.Println("Splits (Double-Entry)")
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

	pterm.DefaultTable.WithHasHeader().WithData(splitsData).Render()
}
