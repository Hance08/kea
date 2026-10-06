// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
	"strconv"
	"strings"
	"time"

	"github.com/hance08/kea/internal/model"
)

// ── JSON DTOs ─────────────────────────────────────────────────────────────────
// Amounts are float64 currency units (not cents). Dates are YYYY-MM-DD strings.
// TransactionStatus is serialized via .String(). AccountType as single-letter code.

type JSONAccount struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	ParentID    *int64  `json:"parent_id"`
	Currency    string  `json:"currency"`
	Description string  `json:"description"`
	IsHidden    bool    `json:"is_hidden"`
	Balance     float64 `json:"balance"`
}

type JSONSplitDetail struct {
	ID          int64   `json:"id"`
	AccountID   int64   `json:"account_id"`
	AccountName string  `json:"account_name"`
	AccountType string  `json:"account_type"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Memo        string  `json:"memo"`
}

type JSONTxDetail struct {
	ID          int64             `json:"id"`
	Date        string            `json:"date"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Status      string            `json:"status"`
	Splits      []JSONSplitDetail `json:"splits"`
}

type JSONTxListItem struct {
	ID          int64   `json:"id"`
	Date        string  `json:"date"`
	Type        string  `json:"type"`
	Account     string  `json:"account"`
	Offset      string  `json:"offset"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	Regular     *bool   `json:"regular,omitempty"`
}

type JSONSystemInfo struct {
	ConfigPath      string `json:"config_path"`
	ActiveLedger    string `json:"active_ledger"`
	DBPath          string `json:"db_path"`
	// false means DB does not yet exist; it will be created on first use.
	DBExists        bool   `json:"db_exists"`
	DefaultCurrency string `json:"default_currency"`
	AppDataDir      string `json:"app_data_dir"`
}

// ── Converters ────────────────────────────────────────────────────────────────

func ToJSONAccount(acc *model.Account, balanceCents int64) JSONAccount {
	return JSONAccount{
		ID:          acc.ID,
		Name:        acc.Name,
		Type:        string(acc.Type),
		ParentID:    acc.ParentID,
		Currency:    acc.Currency,
		Description: acc.Description,
		IsHidden:    acc.IsHidden,
		Balance:     CentsToUnit(balanceCents),
	}
}

func toJSONSplit(s model.SplitDetail) JSONSplitDetail {
	return JSONSplitDetail{
		ID:          s.ID,
		AccountID:   s.AccountID,
		AccountName: s.AccountName,
		AccountType: string(s.AccountType),
		Amount:      CentsToUnit(s.Amount),
		Currency:    s.Currency,
		Memo:        s.Memo,
	}
}

func ToJSONTxDetail(d *model.TransactionDetail) JSONTxDetail {
	splits := make([]JSONSplitDetail, len(d.Splits))
	for i, s := range d.Splits {
		splits[i] = toJSONSplit(s)
	}
	return JSONTxDetail{
		ID:          d.ID,
		Date:        time.Unix(d.Timestamp, 0).Format(model.DateFormat),
		Description: d.Description,
		Type:        string(d.Type),
		Status:      d.Status.String(),
		Splits:      splits,
	}
}

func ToJSONTxListItem(item TransactionListItem) JSONTxListItem {
	amount, _ := strconv.ParseFloat(strings.ReplaceAll(item.Amount, ",", ""), 64)
	return JSONTxListItem{
		ID:          item.ID,
		Date:        item.Date,
		Type:        item.Type,
		Account:     item.Account,
		Offset:      item.Offset,
		Description: item.Description,
		Amount:      amount,
		Currency:    item.Currency,
		Status:      item.Status,
		Regular:     item.Regular,
	}
}

func ToJSONSystemInfo(info SystemInfo) JSONSystemInfo {
	return JSONSystemInfo{
		ConfigPath:      info.ConfigPath,
		ActiveLedger:    info.ActiveLedger,
		DBPath:          info.DBPath,
		DBExists:        info.DBExists,
		DefaultCurrency: info.DefaultCurrency,
		AppDataDir:      info.AppDataDir,
	}
}

// JSONBudget is the CLI JSON shape of a budget version (amount in currency units).
type JSONBudget struct {
	ID             int64   `json:"id"`
	AccountName    string  `json:"account_name"`
	EffectiveMonth string  `json:"effective_month"`
	Amount         float64 `json:"amount"`
	Stopped        bool    `json:"stopped"`
}

func ToJSONBudget(b model.Budget) JSONBudget {
	return JSONBudget{
		ID:             b.ID,
		AccountName:    b.AccountName,
		EffectiveMonth: b.EffectiveMonth,
		Amount:         CentsToUnit(b.Amount),
		Stopped:        b.Stopped,
	}
}

func ToJSONBudgets(bs []model.Budget) []JSONBudget {
	out := make([]JSONBudget, len(bs))
	for i, b := range bs {
		out[i] = ToJSONBudget(b)
	}
	return out
}

// JSONBudgetReportRow mirrors model.BudgetReportRow in currency units.
type JSONBudgetReportRow struct {
	AccountName      string   `json:"account_name"`
	Currency         string   `json:"currency"`
	EffectiveMonth   string   `json:"effective_month"`
	Budget           float64  `json:"budget"`
	Actual           float64  `json:"actual"`
	ActualRegular    float64  `json:"actual_regular"`
	ActualIrregular  float64  `json:"actual_irregular"`
	Remaining        float64  `json:"remaining"`
	ExcludedAccounts []string `json:"excluded_accounts"`
}

// JSONBudgetReport mirrors model.BudgetReport in currency units.
type JSONBudgetReport struct {
	Month       string                `json:"month"`
	Rows        []JSONBudgetReportRow `json:"rows"`
	TotalBudget map[string]float64    `json:"total_budget"`
	TotalActual map[string]float64    `json:"total_actual"`
}

func ToJSONBudgetReport(r *model.BudgetReport) JSONBudgetReport {
	rows := make([]JSONBudgetReportRow, len(r.Rows))
	for i, row := range r.Rows {
		excluded := row.ExcludedAccounts
		if excluded == nil {
			excluded = []string{}
		}
		rows[i] = JSONBudgetReportRow{
			AccountName:      row.AccountName,
			Currency:         row.Currency,
			EffectiveMonth:   row.EffectiveMonth,
			Budget:           CentsToUnit(row.Budget),
			Actual:           CentsToUnit(row.Actual),
			ActualRegular:    CentsToUnit(row.ActualRegular),
			ActualIrregular:  CentsToUnit(row.ActualIrregular),
			Remaining:        CentsToUnit(row.Remaining),
			ExcludedAccounts: excluded,
		}
	}
	totalBudget := centsMapToUnitMap(r.TotalBudget)
	if totalBudget == nil {
		totalBudget = map[string]float64{}
	}
	totalActual := centsMapToUnitMap(r.TotalActual)
	if totalActual == nil {
		totalActual = map[string]float64{}
	}
	return JSONBudgetReport{Month: r.Month, Rows: rows, TotalBudget: totalBudget, TotalActual: totalActual}
}

// JSONSavingsTarget is the CLI JSON shape of a savings target version (amount in currency units).
type JSONSavingsTarget struct {
	ID             int64   `json:"id"`
	AccountName    string  `json:"account_name"`
	EffectiveMonth string  `json:"effective_month"`
	Amount         float64 `json:"amount"`
	Stopped        bool    `json:"stopped"`
}

func ToJSONSavingsTarget(s model.SavingsTarget) JSONSavingsTarget {
	return JSONSavingsTarget{
		ID:             s.ID,
		AccountName:    s.AccountName,
		EffectiveMonth: s.EffectiveMonth,
		Amount:         CentsToUnit(s.Amount),
		Stopped:        s.Stopped,
	}
}

func ToJSONSavingsTargets(ss []model.SavingsTarget) []JSONSavingsTarget {
	out := make([]JSONSavingsTarget, len(ss))
	for i, s := range ss {
		out[i] = ToJSONSavingsTarget(s)
	}
	return out
}

// JSONSavingsMonth mirrors model.SavingsMonth in currency units.
type JSONSavingsMonth struct {
	Month  string  `json:"month"`
	Target float64 `json:"target"`
	Saved  float64 `json:"saved"`
}

// JSONSavingsReportRow mirrors model.SavingsReportRow in currency units.
type JSONSavingsReportRow struct {
	AccountName      string             `json:"account_name"`
	Currency         string             `json:"currency"`
	EffectiveMonth   string             `json:"effective_month"`
	Target           float64            `json:"target"`
	Saved            float64            `json:"saved"`
	Remaining        float64            `json:"remaining"`
	YTDTarget        float64            `json:"ytd_target"`
	YTDSaved         float64            `json:"ytd_saved"`
	YTDRemaining     float64            `json:"ytd_remaining"`
	Months           []JSONSavingsMonth `json:"months"`
	ExcludedAccounts []string           `json:"excluded_accounts"`
}

// JSONSavingsReport mirrors model.SavingsReport in currency units.
type JSONSavingsReport struct {
	Month          string                 `json:"month"`
	Rows           []JSONSavingsReportRow `json:"rows"`
	TotalTarget    map[string]float64     `json:"total_target"`
	TotalSaved     map[string]float64     `json:"total_saved"`
	TotalYTDTarget map[string]float64     `json:"total_ytd_target"`
	TotalYTDSaved  map[string]float64     `json:"total_ytd_saved"`
}

func ToJSONSavingsReport(r *model.SavingsReport) JSONSavingsReport {
	rows := make([]JSONSavingsReportRow, len(r.Rows))
	for i, row := range r.Rows {
		months := make([]JSONSavingsMonth, len(row.Months))
		for j, m := range row.Months {
			months[j] = JSONSavingsMonth{Month: m.Month, Target: CentsToUnit(m.Target), Saved: CentsToUnit(m.Saved)}
		}
		excluded := row.ExcludedAccounts
		if excluded == nil {
			excluded = []string{}
		}
		rows[i] = JSONSavingsReportRow{
			AccountName:      row.AccountName,
			Currency:         row.Currency,
			EffectiveMonth:   row.EffectiveMonth,
			Target:           CentsToUnit(row.Target),
			Saved:            CentsToUnit(row.Saved),
			Remaining:        CentsToUnit(row.Remaining),
			YTDTarget:        CentsToUnit(row.YTDTarget),
			YTDSaved:         CentsToUnit(row.YTDSaved),
			YTDRemaining:     CentsToUnit(row.YTDRemaining),
			Months:           months,
			ExcludedAccounts: excluded,
		}
	}
	unitMap := func(m map[string]int64) map[string]float64 {
		if out := centsMapToUnitMap(m); out != nil {
			return out
		}
		return map[string]float64{}
	}
	return JSONSavingsReport{
		Month:          r.Month,
		Rows:           rows,
		TotalTarget:    unitMap(r.TotalTarget),
		TotalSaved:     unitMap(r.TotalSaved),
		TotalYTDTarget: unitMap(r.TotalYTDTarget),
		TotalYTDSaved:  unitMap(r.TotalYTDSaved),
	}
}
