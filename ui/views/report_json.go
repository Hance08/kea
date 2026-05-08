// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
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
	return WriteJSON(toJSONReportResult(result))
}

func (v *JSONReportView) RenderIncomeBreakdown(result *model.ReportResult) error {
	return WriteJSON(toJSONReportResult(result))
}

func (v *JSONReportView) RenderExpenseBreakdown(result *model.ReportResult) error {
	return WriteJSON(toJSONReportResult(result))
}

func (v *JSONReportView) RenderBalanceSheet(result *model.BalanceSheetResult) error {
	return WriteJSON(toJSONBalanceSheetResult(result))
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
	Period            string             `json:"period"`
	TotalIncome       map[string]float64 `json:"total_income"`
	TotalExpense      map[string]float64 `json:"total_expense"`
	NetAmount         map[string]float64 `json:"net_amount"`
	NetWorth          map[string]float64 `json:"net_worth"`
	PreviousNetWorth  map[string]float64 `json:"previous_net_worth"`
	NetWorthGrowthPct map[string]float64 `json:"net_worth_growth_pct"`
	IncomeRows        []jsonReportRow    `json:"income_rows"`
	ExpenseRows       []jsonReportRow    `json:"expense_rows"`
}

type jsonBalanceSheetResult struct {
	Assets           []jsonReportRow    `json:"assets"`
	Liabilities      []jsonReportRow    `json:"liabilities"`
	Equity           []jsonReportRow    `json:"equity"`
	TotalAssets      map[string]float64 `json:"total_assets"`
	TotalLiabilities map[string]float64 `json:"total_liabilities"`
	TotalEquity      map[string]float64 `json:"total_equity"`
	NetWorth         map[string]float64 `json:"net_worth"`
	AsOf             int64              `json:"as_of"`
}

func toJSONRow(r model.ReportRow) jsonReportRow {
	return jsonReportRow{
		AccountName:   r.AccountName,
		OffsetAccount: r.OffsetAccount,
		Amount:        CentsToUnit(r.Amount),
		Currency:      r.Currency,
		TxCount:       r.TxCount,
	}
}

func centsMapToUnitMap(m map[string]int64) map[string]float64 {
	if m == nil {
		return nil
	}
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = CentsToUnit(v)
	}
	return out
}

func toJSONRows(rows []model.ReportRow) []jsonReportRow {
	out := make([]jsonReportRow, len(rows))
	for i, r := range rows {
		out[i] = toJSONRow(r)
	}
	return out
}

func toJSONReportResult(r *model.ReportResult) jsonReportResult {
	return jsonReportResult{
		Period:            r.Period,
		TotalIncome:       centsMapToUnitMap(r.TotalIncome),
		TotalExpense:      centsMapToUnitMap(r.TotalExpense),
		NetAmount:         centsMapToUnitMap(r.NetAmount),
		NetWorth:          centsMapToUnitMap(r.NetWorth),
		PreviousNetWorth:  centsMapToUnitMap(r.PreviousNetWorth),
		NetWorthGrowthPct: r.NetWorthGrowthPct,
		IncomeRows:        toJSONRows(r.IncomeRows),
		ExpenseRows:       toJSONRows(r.ExpenseRows),
	}
}

func toJSONBalanceSheetResult(r *model.BalanceSheetResult) jsonBalanceSheetResult {
	return jsonBalanceSheetResult{
		Assets:           toJSONRows(r.Assets),
		Liabilities:      toJSONRows(r.Liabilities),
		Equity:           toJSONRows(r.Equity),
		TotalAssets:      centsMapToUnitMap(r.TotalAssets),
		TotalLiabilities: centsMapToUnitMap(r.TotalLiabilities),
		TotalEquity:      centsMapToUnitMap(r.TotalEquity),
		NetWorth:         centsMapToUnitMap(r.NetWorth),
		AsOf:             r.AsOf,
	}
}
