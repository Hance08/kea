// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// ReportRow represents a single account line in a report.
type ReportRow struct {
	AccountName   string `json:"account_name"`
	OffsetAccount string `json:"offset_account"` // the counter-entry account (e.g. asset/liability that funds an expense)
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	TxCount       int    `json:"tx_count"`
}

// ReportResult holds the result of an income statement or expense breakdown report.
type ReportResult struct {
	Period            string             `json:"period"`
	TotalIncome       map[string]int64   `json:"total_income"`
	TotalExpense      map[string]int64   `json:"total_expense"`
	NetAmount         map[string]int64   `json:"net_amount"`
	NetWorth          map[string]int64   `json:"net_worth"`
	PreviousNetWorth  map[string]int64   `json:"previous_net_worth"`
	NetWorthGrowthPct map[string]float64 `json:"net_worth_growth_pct"`
	IncomeRows        []ReportRow        `json:"income_rows"`
	ExpenseRows       []ReportRow        `json:"expense_rows"`
}

// BalanceSheetResult holds the result of a balance sheet report.
type BalanceSheetResult struct {
	Assets           []ReportRow      `json:"assets"`
	Liabilities      []ReportRow      `json:"liabilities"`
	Equity           []ReportRow      `json:"equity"`
	TotalAssets      map[string]int64 `json:"total_assets"`
	TotalLiabilities map[string]int64 `json:"total_liabilities"`
	TotalEquity      map[string]int64 `json:"total_equity"`
	NetWorth         map[string]int64 `json:"net_worth"`
	AsOf             int64            `json:"as_of"`
}
