package model

// ReportRow represents a single account line in a report.
type ReportRow struct {
	AccountName string
	Amount      int64
	Currency    string
	TxCount     int
}

// ReportResult holds the result of an income statement or expense breakdown report.
type ReportResult struct {
	Period       string
	TotalIncome  int64
	TotalExpense int64
	NetAmount    int64
	NetWorth     int64 // cumulative net worth (assets - liabilities) at report time
	Currency     string
	IncomeRows   []ReportRow
	ExpenseRows  []ReportRow
}

// BalanceSheetResult holds the result of a balance sheet report.
type BalanceSheetResult struct {
	Assets           []ReportRow
	Liabilities      []ReportRow
	Equity           []ReportRow
	TotalAssets      int64
	TotalLiabilities int64
	TotalEquity      int64
	NetWorth         int64
	Currency         string
}
