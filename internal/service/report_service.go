// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

// DateRangeParams holds the optional date range inputs for report queries.
type DateRangeParams struct {
	Month string
	From  string
	To    string
}

// ResolveDateRange resolves a DateRangeParams into concrete Unix timestamps and a human-readable period string.
// Month takes priority; if neither Month nor From/To are set, defaults to the current calendar month.
func (ts *TransactionService) ResolveDateRange(params DateRangeParams) (startTime, endTime int64, period string, err error) {
	switch {
	case params.Month != "":
		return parseMonth(params.Month)
	case params.From != "" || params.To != "":
		return parseDateRange(params.From, params.To)
	default:
		now := time.Now()
		return parseMonth(now.Format("2006-01"))
	}
}

// GenerateFullIncomeStatement resolves the date range from params, generates an income statement,
// and enriches it with period label, current and previous net worth, and growth percentage.
func (ts *TransactionService) GenerateFullIncomeStatement(ctx context.Context, params DateRangeParams) (*model.ReportResult, error) {
	start, end, period, err := ts.ResolveDateRange(params)
	if err != nil {
		return nil, err
	}

	result, err := ts.GenerateIncomeStatement(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to generate income statement: %w", err)
	}

	result.Period = period

	currentNetWorth, err := ts.GetNetWorthAt(ctx, end)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch net worth for current period: %w", err)
	}
	result.NetWorth = currentNetWorth

	_, prevEnd := previousPeriodRange(start, end)
	previousNetWorth, err := ts.GetNetWorthAt(ctx, prevEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch net worth for previous period: %w", err)
	}
	result.PreviousNetWorth = previousNetWorth
	result.NetWorthGrowthPct = computeNetWorthGrowthPctMap(currentNetWorth, previousNetWorth)

	return result, nil
}

// GenerateFullIncomeBreakdown resolves the date range from params, generates a detailed income breakdown,
// and sets the period label on the result.
func (ts *TransactionService) GenerateFullIncomeBreakdown(ctx context.Context, params DateRangeParams) (*model.ReportResult, error) {
	start, end, period, err := ts.ResolveDateRange(params)
	if err != nil {
		return nil, err
	}

	result, err := ts.GenerateIncomeBreakdown(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to generate income breakdown: %w", err)
	}

	result.Period = period
	return result, nil
}

// GenerateFullExpenseBreakdown resolves the date range from params, generates a detailed expense breakdown,
// and sets the period label on the result.
func (ts *TransactionService) GenerateFullExpenseBreakdown(ctx context.Context, params DateRangeParams) (*model.ReportResult, error) {
	start, end, period, err := ts.ResolveDateRange(params)
	if err != nil {
		return nil, err
	}

	result, err := ts.GenerateExpenseBreakdown(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to generate expense breakdown: %w", err)
	}

	result.Period = period
	return result, nil
}

// GetNetWorthAt returns net worth (assets + liabilities) per currency using all posted splits up to endTime.
// Liability splits are stored as negative values (credit-normal accounts), so adding them to assets
// correctly subtracts the outstanding balance.
func (ts *TransactionService) GetNetWorthAt(ctx context.Context, endTime int64) (map[string]int64, error) {
	txSplitsMap, err := ts.txRepo.GetSplitsWithAccountsByDateRange(ctx, 0, endTime)
	if err != nil {
		return nil, err
	}

	assets := map[string]int64{}
	liabilities := map[string]int64{}
	for _, splits := range txSplitsMap {
		for _, split := range splits {
			switch split.AccountType {
			case model.AccountTypeAsset:
				assets[split.Currency] += split.Amount
			case model.AccountTypeLiability:
				liabilities[split.Currency] += split.Amount
			}
		}
	}

	nw := map[string]int64{}
	for ccy, amt := range assets {
		nw[ccy] = amt + liabilities[ccy]
	}
	for ccy, amt := range liabilities {
		if _, ok := nw[ccy]; !ok {
			nw[ccy] = amt
		}
	}
	return nw, nil
}

// offsetAccountName inspects the splits of a transaction and returns the name of the
// "other side" account relative to the given primary account type filter.
// If exactly one offset account exists it returns its name; if multiple exist it
// returns "(multiple)"; if none exist it returns an empty string.
func offsetAccountName(details []model.SplitDetail, primaryType model.AccountType) string {
	seen := map[string]struct{}{}
	for _, d := range details {
		if d.AccountType != primaryType {
			seen[d.AccountName] = struct{}{}
		}
	}
	switch len(seen) {
	case 0:
		return ""
	case 1:
		for name := range seen {
			return name
		}
	}
	return "(multiple)"
}

// buildReportMaps fetches all splits in the date range and aggregates them into
// income/expense row maps. Pass includeIncome=false to skip income classification
// (and vice versa) to avoid unnecessary work for breakdown-only queries.
func (ts *TransactionService) buildReportMaps(ctx context.Context, startTime, endTime int64, includeIncome, includeExpense bool) (incomeByAccount, expenseByAccount map[string]*model.ReportRow, err error) {
	txSplitsMap, err := ts.txRepo.GetSplitsWithAccountsByDateRange(ctx, startTime, endTime)
	if err != nil {
		return nil, nil, err
	}

	txs, err := ts.txRepo.GetTransactionsByDateRange(ctx, startTime, endTime)
	if err != nil {
		return nil, nil, err
	}
	txTypeMap := make(map[int64]model.TransactionType, len(txs))
	for _, tx := range txs {
		txTypeMap[tx.ID] = tx.Type
	}

	incomeByAccount = map[string]*model.ReportRow{}
	expenseByAccount = map[string]*model.ReportRow{}

	for txID, details := range txSplitsMap {
		txType := txTypeMap[txID]

		if includeIncome && txType == model.TxTypeIncome {
			offset := offsetAccountName(details, model.AccountTypeRevenue)
			for _, split := range details {
				if split.AccountType == model.AccountTypeRevenue {
					key := split.AccountName + "|" + offset + "|" + split.Currency
					row := getOrCreateRowWithOffset(incomeByAccount, key, split.AccountName, offset, split.Currency)
					row.Amount += utils.AbsInt64(split.Amount)
					row.TxCount++
				}
			}
		}

		if includeExpense && txType == model.TxTypeExpense {
			offset := offsetAccountName(details, model.AccountTypeExpense)
			for _, split := range details {
				if split.AccountType == model.AccountTypeExpense {
					key := split.AccountName + "|" + offset + "|" + split.Currency
					row := getOrCreateRowWithOffset(expenseByAccount, key, split.AccountName, offset, split.Currency)
					row.Amount += utils.AbsInt64(split.Amount)
					row.TxCount++
				}
			}
		}
	}

	return incomeByAccount, expenseByAccount, nil
}

// GenerateIncomeStatement produces an income/expense summary for the given Unix time range.
func (ts *TransactionService) GenerateIncomeStatement(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
	incomeByAccount, expenseByAccount, err := ts.buildReportMaps(ctx, startTime, endTime, true, true)
	if err != nil {
		return nil, err
	}

	result := &model.ReportResult{
		IncomeRows:  rowsFromMap(incomeByAccount),
		ExpenseRows: rowsFromMap(expenseByAccount),
		TotalIncome:  map[string]int64{},
		TotalExpense: map[string]int64{},
		NetAmount:    map[string]int64{},
	}
	for _, r := range result.IncomeRows {
		result.TotalIncome[r.Currency] += r.Amount
	}
	for _, r := range result.ExpenseRows {
		result.TotalExpense[r.Currency] += r.Amount
	}
	for ccy, inc := range result.TotalIncome {
		result.NetAmount[ccy] = inc - result.TotalExpense[ccy]
	}
	for ccy, exp := range result.TotalExpense {
		if _, ok := result.NetAmount[ccy]; !ok {
			result.NetAmount[ccy] = -exp
		}
	}

	return result, nil
}

// GenerateIncomeBreakdown produces a detailed income-only report for the given Unix time range.
func (ts *TransactionService) GenerateIncomeBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
	incomeByAccount, _, err := ts.buildReportMaps(ctx, startTime, endTime, true, false)
	if err != nil {
		return nil, err
	}

	rows := rowsFromMap(incomeByAccount)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Amount > rows[j].Amount })

	total := map[string]int64{}
	for _, r := range rows {
		total[r.Currency] += r.Amount
	}

	return &model.ReportResult{
		IncomeRows:  rows,
		ExpenseRows: []model.ReportRow{},
		TotalIncome: total,
	}, nil
}

// GenerateExpenseBreakdown produces a detailed expense-only report for the given Unix time range.
func (ts *TransactionService) GenerateExpenseBreakdown(ctx context.Context, startTime, endTime int64) (*model.ReportResult, error) {
	_, expenseByAccount, err := ts.buildReportMaps(ctx, startTime, endTime, false, true)
	if err != nil {
		return nil, err
	}

	rows := rowsFromMap(expenseByAccount)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Amount > rows[j].Amount })

	total := map[string]int64{}
	for _, r := range rows {
		total[r.Currency] += r.Amount
	}

	return &model.ReportResult{
		IncomeRows:   []model.ReportRow{},
		ExpenseRows:  rows,
		TotalExpense: total,
	}, nil
}

// GenerateBalanceSheet produces a snapshot of all asset, liability, and equity account balances
// as of the given Unix timestamp.
func (ts *TransactionService) GenerateBalanceSheet(ctx context.Context, asOf int64) (*model.BalanceSheetResult, error) {
	allAccounts, err := ts.accRepo.GetAllAccounts(ctx)
	if err != nil {
		return nil, err
	}

	balances, err := ts.accRepo.GetAllAccountBalances(ctx, asOf)
	if err != nil {
		return nil, err
	}

	result := &model.BalanceSheetResult{
		AsOf:             asOf,
		Assets:           []model.ReportRow{},
		Liabilities:      []model.ReportRow{},
		Equity:           []model.ReportRow{},
		TotalAssets:      map[string]int64{},
		TotalLiabilities: map[string]int64{},
		TotalEquity:      map[string]int64{},
		NetWorth:         map[string]int64{},
	}

	for _, acc := range allAccounts {
		balance := balances[acc.ID]

		currency := acc.Currency
		if currency == "" {
			currency = ts.config.Defaults.Currency
		}

		row := model.ReportRow{
			AccountName: acc.Name,
			Amount:      balance,
			Currency:    currency,
		}

		switch acc.Type {
		case model.AccountTypeAsset:
			result.Assets = append(result.Assets, row)
			result.TotalAssets[currency] += balance
		case model.AccountTypeLiability:
			displayBalance := -balance
			row := model.ReportRow{
				AccountName: acc.Name,
				Amount:      displayBalance,
				Currency:    currency,
			}
			result.Liabilities = append(result.Liabilities, row)
			result.TotalLiabilities[currency] += displayBalance
		case model.AccountTypeEquity:
			result.Equity = append(result.Equity, row)
			result.TotalEquity[currency] += balance
		}
	}

	for ccy, assets := range result.TotalAssets {
		result.NetWorth[ccy] = assets - result.TotalLiabilities[ccy]
	}
	for ccy, liabs := range result.TotalLiabilities {
		if _, ok := result.NetWorth[ccy]; !ok {
			result.NetWorth[ccy] = -liabs
		}
	}

	sort.Slice(result.Assets, func(i, j int) bool {
		return result.Assets[i].Amount > result.Assets[j].Amount
	})
	sort.Slice(result.Liabilities, func(i, j int) bool {
		return result.Liabilities[i].Amount > result.Liabilities[j].Amount
	})
	sort.Slice(result.Equity, func(i, j int) bool {
		return result.Equity[i].Amount > result.Equity[j].Amount
	})

	return result, nil
}

// getOrCreateRowWithOffset is like getOrCreateRow but also sets OffsetAccount.
// The map key is expected to already encode both dimensions (e.g. "accName|offsetName").
func getOrCreateRowWithOffset(m map[string]*model.ReportRow, key, name, offset, currency string) *model.ReportRow {
	if _, ok := m[key]; !ok {
		m[key] = &model.ReportRow{AccountName: name, OffsetAccount: offset, Currency: currency}
	}
	return m[key]
}

// rowsFromMap converts a map of ReportRows to a slice sorted by amount descending.
func rowsFromMap(m map[string]*model.ReportRow) []model.ReportRow {
	rows := make([]model.ReportRow, 0, len(m))
	for _, r := range m {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Amount > rows[j].Amount
	})
	return rows
}

func parseMonth(month string) (startTime, endTime int64, period string, err error) {
	loc := time.Local
	t, parseErr := time.ParseInLocation("2006-01", month, loc)
	if parseErr != nil {
		err = validationErrorf("month", "invalid month format %q, expected YYYY-MM", month)
		return
	}

	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0).Add(-time.Second)

	startTime = start.Unix()
	endTime = end.Unix()
	period = start.Format("January 2006")
	return
}

func parseDateRange(from, to string) (startTime, endTime int64, period string, err error) {
	loc := time.Local

	var startDate, endDate time.Time

	if from == "" {
		startDate = time.Unix(0, 0)
	} else {
		startDate, err = time.ParseInLocation(model.DateFormat, from, loc)
		if err != nil {
			err = validationErrorf("from", "invalid from-date format %q, expected YYYY-MM-DD", from)
			return
		}
	}

	if to == "" {
		endDate = time.Now()
	} else {
		endDate, err = time.ParseInLocation(model.DateFormat, to, loc)
		if err != nil {
			err = validationErrorf("to", "invalid to-date format %q, expected YYYY-MM-DD", to)
			return
		}
		endDate = endDate.Add(24*time.Hour - time.Second)
	}

	if endDate.Before(startDate) {
		err = validationErrorf("to", "end date must be on or after start date")
		return
	}

	startTime = startDate.Unix()
	endTime = endDate.Unix()
	period = fmt.Sprintf("%s – %s", startDate.Format(model.DateFormat), endDate.Format(model.DateFormat))
	return
}

func previousPeriodRange(startTime, endTime int64) (prevStart, prevEnd int64) {
	duration := endTime - startTime + 1
	prevEnd = startTime - 1
	prevStart = prevEnd - duration + 1
	return
}

func computeNetWorthGrowthPctMap(current, previous map[string]int64) map[string]float64 {
	result := map[string]float64{}
	allCurrencies := map[string]struct{}{}
	for ccy := range current {
		allCurrencies[ccy] = struct{}{}
	}
	for ccy := range previous {
		allCurrencies[ccy] = struct{}{}
	}
	for ccy := range allCurrencies {
		prev := previous[ccy]
		if prev == 0 {
			continue
		}
		cur := current[ccy]
		result[ccy] = (float64(cur-prev) / float64(utils.AbsInt64(prev))) * 100
	}
	return result
}

// GetDailyNetWorthSeries returns daily net-worth points across all currencies,
// dense from the earliest transaction day through today (UTC), front-filled
// over days without activity. Income/expense splits are excluded.
func (ts *TransactionService) GetDailyNetWorthSeries(ctx context.Context) ([]model.CurrencyDailySeries, error) {
	return ts.GetDailyNetWorthSeriesUntil(ctx, time.Now().Unix())
}

// GetDailyNetWorthSeriesUntil is the deterministic variant used by tests; it
// pins the upper bound of the series to a fixed Unix timestamp.
func (ts *TransactionService) GetDailyNetWorthSeriesUntil(ctx context.Context, until int64) ([]model.CurrencyDailySeries, error) {
	txs, err := ts.txRepo.GetTransactionsByDateRange(ctx, 0, until)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}
	if len(txs) == 0 {
		return []model.CurrencyDailySeries{}, nil
	}

	splitsByTx, err := ts.txRepo.GetSplitsWithAccountsByDateRange(ctx, 0, until)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch splits: %w", err)
	}

	// Bucket day deltas per currency: map[currency]map[YYYY-MM-DD]int64.
	deltas := map[string]map[string]int64{}
	firstDay := ""
	for _, tx := range txs {
		day := time.Unix(tx.Timestamp, 0).UTC().Format("2006-01-02")
		if firstDay == "" || day < firstDay {
			firstDay = day
		}
		for _, sp := range splitsByTx[tx.ID] {
			if sp.AccountType != model.AccountTypeAsset && sp.AccountType != model.AccountTypeLiability {
				continue
			}
			byDay, ok := deltas[sp.Currency]
			if !ok {
				byDay = map[string]int64{}
				deltas[sp.Currency] = byDay
			}
			byDay[day] += sp.Amount
		}
	}
	if len(deltas) == 0 {
		return []model.CurrencyDailySeries{}, nil
	}

	lastDay := time.Unix(until, 0).UTC().Format("2006-01-02")
	currencies := make([]string, 0, len(deltas))
	for c := range deltas {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)

	out := make([]model.CurrencyDailySeries, 0, len(currencies))
	for _, ccy := range currencies {
		byDay := deltas[ccy]
		var points []model.DailyBalancePoint
		var running int64
		day, _ := time.Parse("2006-01-02", firstDay)
		end, _ := time.Parse("2006-01-02", lastDay)
		for !day.After(end) {
			d := day.Format("2006-01-02")
			running += byDay[d]
			points = append(points, model.DailyBalancePoint{Date: d, Balance: running})
			day = day.Add(24 * time.Hour)
		}
		// Skip series that never moved off zero.
		nonZero := false
		for _, p := range points {
			if p.Balance != 0 {
				nonZero = true
				break
			}
		}
		if !nonZero {
			continue
		}
		out = append(out, model.CurrencyDailySeries{Currency: ccy, Points: points})
	}
	return out, nil
}
