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
	return writeJSON(toJSONReportResult(result))
}

func (v *JSONReportView) RenderExpenseBreakdown(result *model.ReportResult) error {
	return writeJSON(toJSONReportResult(result))
}

func (v *JSONReportView) RenderBalanceSheet(result *model.BalanceSheetResult) error {
	return writeJSON(toJSONBalanceSheetResult(result))
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

// ── JSON-specific DTOs ────────────────────────────────────────────────────────
// Amounts are converted from cents (int64) to the base currency unit (float64)
// by dividing by model.CentsPerUnit (100), so consumers receive human-readable
// values without needing to know the internal storage format.

type jsonReportRow struct {
	AccountName   string  `json:"account_name"`
	OffsetAccount string  `json:"offset_account"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	TxCount       int     `json:"tx_count"`
}

type jsonReportResult struct {
	Period            string          `json:"period"`
	TotalIncome       float64         `json:"total_income"`
	TotalExpense      float64         `json:"total_expense"`
	NetAmount         float64         `json:"net_amount"`
	NetWorth          float64         `json:"net_worth"`
	PreviousNetWorth  *float64        `json:"previous_net_worth"`
	NetWorthGrowthPct *float64        `json:"net_worth_growth_pct"`
	Currency          string          `json:"currency"`
	IncomeRows        []jsonReportRow `json:"income_rows"`
	ExpenseRows       []jsonReportRow `json:"expense_rows"`
}

type jsonBalanceSheetResult struct {
	Assets            []jsonReportRow `json:"assets"`
	Liabilities       []jsonReportRow `json:"liabilities"`
	Equity            []jsonReportRow `json:"equity"`
	TotalAssets       float64         `json:"total_assets"`
	TotalLiabilities  float64         `json:"total_liabilities"`
	TotalEquity       float64         `json:"total_equity"`
	NetWorth          float64         `json:"net_worth"`
	PreviousNetWorth  *float64        `json:"previous_net_worth"`
	NetWorthGrowthPct *float64        `json:"net_worth_growth_pct"`
	Currency          string          `json:"currency"`
}

func centsToUnit(cents int64) float64 {
	return float64(cents) / float64(model.CentsPerUnit)
}

func toJSONRow(r model.ReportRow) jsonReportRow {
	return jsonReportRow{
		AccountName:   r.AccountName,
		OffsetAccount: r.OffsetAccount,
		Amount:        centsToUnit(r.Amount),
		Currency:      r.Currency,
		TxCount:       r.TxCount,
	}
}

func toJSONRows(rows []model.ReportRow) []jsonReportRow {
	out := make([]jsonReportRow, len(rows))
	for i, r := range rows {
		out[i] = toJSONRow(r)
	}
	return out
}

func toJSONReportResult(r *model.ReportResult) jsonReportResult {
	var previousNetWorth *float64
	if r.PreviousNetWorth != nil {
		v := centsToUnit(*r.PreviousNetWorth)
		previousNetWorth = &v
	}

	return jsonReportResult{
		Period:            r.Period,
		TotalIncome:       centsToUnit(r.TotalIncome),
		TotalExpense:      centsToUnit(r.TotalExpense),
		NetAmount:         centsToUnit(r.NetAmount),
		NetWorth:          centsToUnit(r.NetWorth),
		PreviousNetWorth:  previousNetWorth,
		NetWorthGrowthPct: r.NetWorthGrowthPct,
		Currency:          r.Currency,
		IncomeRows:        toJSONRows(r.IncomeRows),
		ExpenseRows:       toJSONRows(r.ExpenseRows),
	}
}

func toJSONBalanceSheetResult(r *model.BalanceSheetResult) jsonBalanceSheetResult {
	var previousNetWorth *float64
	if r.PreviousNetWorth != nil {
		v := centsToUnit(*r.PreviousNetWorth)
		previousNetWorth = &v
	}

	return jsonBalanceSheetResult{
		Assets:            toJSONRows(r.Assets),
		Liabilities:       toJSONRows(r.Liabilities),
		Equity:            toJSONRows(r.Equity),
		TotalAssets:       centsToUnit(r.TotalAssets),
		TotalLiabilities:  centsToUnit(r.TotalLiabilities),
		TotalEquity:       centsToUnit(r.TotalEquity),
		NetWorth:          centsToUnit(r.NetWorth),
		PreviousNetWorth:  previousNetWorth,
		NetWorthGrowthPct: r.NetWorthGrowthPct,
		Currency:          r.Currency,
	}
}
