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
	Period            string      `json:"period"`
	TotalIncome       int64       `json:"total_income"`
	TotalExpense      int64       `json:"total_expense"`
	NetAmount         int64       `json:"net_amount"`
	NetWorth          int64       `json:"net_worth"` // cumulative net worth (assets - liabilities) at report time
	PreviousNetWorth  *int64      `json:"previous_net_worth"`
	NetWorthGrowthPct *float64    `json:"net_worth_growth_pct"`
	Currency          string      `json:"currency"`
	IncomeRows        []ReportRow `json:"income_rows"`
	ExpenseRows       []ReportRow `json:"expense_rows"`
}

// BalanceSheetResult holds the result of a balance sheet report.
type BalanceSheetResult struct {
	Assets           []ReportRow `json:"assets"`
	Liabilities      []ReportRow `json:"liabilities"`
	Equity           []ReportRow `json:"equity"`
	TotalAssets      int64       `json:"total_assets"`
	TotalLiabilities int64       `json:"total_liabilities"`
	TotalEquity      int64       `json:"total_equity"`
	NetWorth         int64       `json:"net_worth"`
	Currency         string      `json:"currency"`
}
