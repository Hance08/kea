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
	Assets           []jsonReportRow `json:"assets"`
	Liabilities      []jsonReportRow `json:"liabilities"`
	Equity           []jsonReportRow `json:"equity"`
	TotalAssets      float64         `json:"total_assets"`
	TotalLiabilities float64         `json:"total_liabilities"`
	TotalEquity      float64         `json:"total_equity"`
	NetWorth         float64         `json:"net_worth"`
	Currency         string          `json:"currency"`
	AsOf             int64           `json:"as_of"`
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
		v := CentsToUnit(*r.PreviousNetWorth)
		previousNetWorth = &v
	}

	return jsonReportResult{
		Period:            r.Period,
		TotalIncome:       CentsToUnit(r.TotalIncome),
		TotalExpense:      CentsToUnit(r.TotalExpense),
		NetAmount:         CentsToUnit(r.NetAmount),
		NetWorth:          CentsToUnit(r.NetWorth),
		PreviousNetWorth:  previousNetWorth,
		NetWorthGrowthPct: r.NetWorthGrowthPct,
		Currency:          r.Currency,
		IncomeRows:        toJSONRows(r.IncomeRows),
		ExpenseRows:       toJSONRows(r.ExpenseRows),
	}
}

func toJSONBalanceSheetResult(r *model.BalanceSheetResult) jsonBalanceSheetResult {
	return jsonBalanceSheetResult{
		Assets:           toJSONRows(r.Assets),
		Liabilities:      toJSONRows(r.Liabilities),
		Equity:           toJSONRows(r.Equity),
		TotalAssets:      CentsToUnit(r.TotalAssets),
		TotalLiabilities: CentsToUnit(r.TotalLiabilities),
		TotalEquity:      CentsToUnit(r.TotalEquity),
		NetWorth:         CentsToUnit(r.NetWorth),
		Currency:         r.Currency,
		AsOf:             r.AsOf,
	}
}
