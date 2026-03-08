package cmd

import "github.com/hance08/kea/internal/model"

// reportFlags holds the parsed CLI flags for the report command.
type reportFlags struct {
	Type  string // income | balance | expense
	Month string // YYYY-MM  (shorthand for a full calendar month)
	From  string // YYYY-MM-DD  (explicit range start)
	To    string // YYYY-MM-DD  (explicit range end)
	JSON  bool   // output raw JSON instead of formatted tables
}

// ReportProvider is the service interface required by the report command.
type ReportProvider interface {
	GenerateIncomeStatement(startTime, endTime int64) (*model.ReportResult, error)
	GenerateExpenseBreakdown(startTime, endTime int64) (*model.ReportResult, error)
	GenerateBalanceSheet() (*model.BalanceSheetResult, error)
}

// ReportView is the view interface used to render report output.
type ReportView interface {
	RenderIncomeStatement(result *model.ReportResult) error
	RenderExpenseBreakdown(result *model.ReportResult) error
	RenderBalanceSheet(result *model.BalanceSheetResult) error
}

// reportRunner wires together flags, provider and view.
type reportRunner struct {
	flags    reportFlags
	provider ReportProvider
	view     ReportView
}
