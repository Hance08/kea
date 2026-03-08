package cmd

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/model"
)

// run is the main entry point called by the cobra command.
func (r *reportRunner) run() error {
	reportType := r.flags.Type
	if reportType == "" {
		reportType = "income"
	}

	switch reportType {
	case "income":
		return r.runIncomeStatement()
	case "expense":
		return r.runExpenseBreakdown()
	case "balance":
		return r.runBalanceSheet()
	default:
		return fmt.Errorf("unknown report type %q — use: income, expense, balance", reportType)
	}
}

func (r *reportRunner) runIncomeStatement() error {
	start, end, period, err := r.resolveDateRange()
	if err != nil {
		return err
	}

	result, err := r.provider.GenerateIncomeStatement(start, end)
	if err != nil {
		return fmt.Errorf("failed to generate income statement: %w", err)
	}

	result.Period = period

	// Fetch current net worth (assets - liabilities) to show as the bottom line.
	bs, err := r.provider.GenerateBalanceSheet()
	if err != nil {
		return fmt.Errorf("failed to fetch net worth: %w", err)
	}
	result.NetWorth = bs.NetWorth

	return r.view.RenderIncomeStatement(result)
}

func (r *reportRunner) runExpenseBreakdown() error {
	start, end, period, err := r.resolveDateRange()
	if err != nil {
		return err
	}

	result, err := r.provider.GenerateExpenseBreakdown(start, end)
	if err != nil {
		return fmt.Errorf("failed to generate expense breakdown: %w", err)
	}

	result.Period = period
	return r.view.RenderExpenseBreakdown(result)
}

func (r *reportRunner) runBalanceSheet() error {

	result, err := r.provider.GenerateBalanceSheet()
	if err != nil {
		return fmt.Errorf("failed to generate balance sheet: %w", err)
	}

	return r.view.RenderBalanceSheet(result)
}

// resolveDateRange converts the --month / --from / --to flags into Unix timestamps and a
// human-readable period label. Defaults to the current calendar month.
func (r *reportRunner) resolveDateRange() (startTime, endTime int64, period string, err error) {
	switch {
	case r.flags.Month != "":
		return parseMonth(r.flags.Month)

	case r.flags.From != "" || r.flags.To != "":
		return parseDateRange(r.flags.From, r.flags.To)

	default:
		// Default: current calendar month
		now := time.Now()
		return parseMonth(now.Format("2006-01"))
	}
}

// parseMonth converts "YYYY-MM" into the start/end Unix timestamps of that month.
func parseMonth(month string) (startTime, endTime int64, period string, err error) {
	loc := time.Local
	t, parseErr := time.ParseInLocation("2006-01", month, loc)
	if parseErr != nil {
		err = fmt.Errorf("invalid --month format %q, expected YYYY-MM", month)
		return
	}

	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0).Add(-time.Second) // last second of the month

	startTime = start.Unix()
	endTime = end.Unix()
	period = start.Format("January 2006")
	return
}

// parseDateRange converts "YYYY-MM-DD" strings into Unix timestamps.
// Either value may be empty — if From is empty it defaults to Unix epoch,
// if To is empty it defaults to now.
func parseDateRange(from, to string) (startTime, endTime int64, period string, err error) {
	loc := time.Local

	var startDate, endDate time.Time

	if from == "" {
		startDate = time.Unix(0, 0)
	} else {
		startDate, err = time.ParseInLocation(model.DateFormat, from, loc)
		if err != nil {
			err = fmt.Errorf("invalid --from format %q, expected YYYY-MM-DD", from)
			return
		}
	}

	if to == "" {
		endDate = time.Now()
	} else {
		endDate, err = time.ParseInLocation(model.DateFormat, to, loc)
		if err != nil {
			err = fmt.Errorf("invalid --to format %q, expected YYYY-MM-DD", to)
			return
		}
		// Include the entire end day.
		endDate = endDate.Add(24*time.Hour - time.Second)
	}

	if endDate.Before(startDate) {
		err = fmt.Errorf("--to date must be on or after --from date")
		return
	}

	startTime = startDate.Unix()
	endTime = endDate.Unix()
	period = fmt.Sprintf("%s – %s", startDate.Format(model.DateFormat), endDate.Format(model.DateFormat))
	return
}
