package views

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hance08/kea/internal/model"
)

// JSONReportView renders reports as indented JSON to stdout.
// It satisfies the same ReportView interface as ReportView, making it a
// drop-in replacement for scripting and automation use-cases.
type JSONReportView struct{}

func NewJSONReportView() *JSONReportView {
	return &JSONReportView{}
}

func (v *JSONReportView) RenderIncomeStatement(result *model.ReportResult) error {
	return writeJSON(result)
}

func (v *JSONReportView) RenderExpenseBreakdown(result *model.ReportResult) error {
	return writeJSON(result)
}

func (v *JSONReportView) RenderBalanceSheet(result *model.BalanceSheetResult) error {
	return writeJSON(result)
}

// writeJSON marshals v to indented JSON and writes it to stdout followed by a newline.
func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}
