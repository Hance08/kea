package views

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/ui"
	"github.com/pterm/pterm"
)

type TransactionDeletePreviewItem struct {
	ID          int64
	Timestamp   int64
	Description string
	SplitCount  int
}

func RenderTransactionDeletePreview(data TransactionDeletePreviewItem) error {
	date := time.Unix(data.Timestamp, 0).Format("2006-01-02")

	pterm.Warning.Printf("About to delete transaction #%d:\n", data.ID)

	deletionInfo := pterm.TableData{
		{"Date", date},
		{"Description", data.Description},
		{"Splits", fmt.Sprint(data.SplitCount)},
	}

	if err := pterm.DefaultTable.WithData(deletionInfo).Render(); err != nil {
		return err
	}
	pterm.Warning.Println("This action cannot be undone!")
	return nil
}

func RenderTransactionDeleteSuccess(id int64) {
	pterm.Success.Printf("Transaction #%d deleted successfully\n", id)
	ui.Separator()
}
