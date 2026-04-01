package cmd

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

// run is the main entry point called by the cobra command.
func (r *reportRunner) run() error {
	reportType := r.flags.Type
	if reportType == "" {
		reportType = "is"
	}

	switch reportType {
	case "is":
		return r.runIncomeStatement()
	case "ib":
		return r.runIncomeBreakdown()
	case "eb":
		return r.runExpenseBreakdown()
	case "bs":
		return r.runBalanceSheet()
	default:
		return fmt.Errorf("unknown report type %q — use: is, ib, eb, bs", reportType)
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

	currentNetWorth, err := r.provider.GetNetWorthAt(end)
	if err != nil {
		return fmt.Errorf("failed to fetch net worth for current period: %w", err)
	}
	result.NetWorth = currentNetWorth

	_, prevEnd := previousPeriodRange(start, end)
	previousNetWorth, err := r.provider.GetNetWorthAt(prevEnd)
	if err != nil {
		return fmt.Errorf("failed to fetch net worth for previous period: %w", err)
	}
	result.PreviousNetWorth = &previousNetWorth
	result.NetWorthGrowthPct = computeNetWorthGrowthPct(currentNetWorth, previousNetWorth)

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

func (r *reportRunner) runIncomeBreakdown() error {
	start, end, period, err := r.resolveDateRange()
	if err != nil {
		return err
	}

	result, err := r.provider.GenerateIncomeBreakdown(start, end)
	if err != nil {
		return fmt.Errorf("failed to generate income breakdown: %w", err)
	}

	result.Period = period
	return r.view.RenderIncomeBreakdown(result)
}

func (r *reportRunner) runBalanceSheet() error {
	_, end, _, err := r.resolveDateRange()
	if err != nil {
		return err
	}

	result, err := r.provider.GenerateBalanceSheet(end)
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

// previousPeriodRange returns the immediate prior period with the same inclusive duration.
func previousPeriodRange(startTime, endTime int64) (prevStart, prevEnd int64) {
	duration := endTime - startTime + 1
	prevEnd = startTime - 1
	prevStart = prevEnd - duration + 1
	return
}

// computeNetWorthGrowthPct returns growth percentage; nil means N/A (previous is zero).
func computeNetWorthGrowthPct(current, previous int64) *float64 {
	if previous == 0 {
		return nil
	}

	growth := (float64(current-previous) / float64(utils.AbsInt64(previous))) * 100
	return &growth
}
