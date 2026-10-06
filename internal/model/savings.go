// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// SavingsTarget is one version of a monthly savings target on an Asset
// account. It applies from EffectiveMonth (YYYY-MM) until a later version for
// the same account. A Stopped version ends the target; its Amount is always 0.
type SavingsTarget struct {
	ID             int64  `json:"id"`
	AccountID      int64  `json:"account_id"`
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
	Amount         int64  `json:"amount"` // cents
	Stopped        bool   `json:"stopped"`
}

// SetSavingsTargetInput sets (or replaces) the target version starting at EffectiveMonth.
type SetSavingsTargetInput struct {
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
	Amount         int64  `json:"amount"` // cents, >= 0
}

// StopSavingsTargetInput ends an account's savings target from EffectiveMonth on.
type StopSavingsTargetInput struct {
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
}

// SavingsReport compares each target active in Month with what was saved,
// for the month and year to date. Totals are per currency and only include
// rows with no targeted ancestor in the same currency.
type SavingsReport struct {
	Month          string             `json:"month"`
	Rows           []SavingsReportRow `json:"rows"`
	TotalTarget    map[string]int64   `json:"total_target"`
	TotalSaved     map[string]int64   `json:"total_saved"`
	TotalYTDTarget map[string]int64   `json:"total_ytd_target"`
	TotalYTDSaved  map[string]int64   `json:"total_ytd_saved"`
}

// SavingsReportRow is one targeted account. Saved is the signed sum of the
// splits on the account and its descendants in Currency, excluding Opening
// transactions; it may be negative. Remaining is Target - Saved (negative
// means the target was exceeded). YTD fields cover January of Month's year
// through Month, counting only months in which the target was active.
type SavingsReportRow struct {
	AccountID        int64          `json:"account_id"`
	AccountName      string         `json:"account_name"`
	Currency         string         `json:"currency"`
	EffectiveMonth   string         `json:"effective_month"`
	Target           int64          `json:"target"`
	Saved            int64          `json:"saved"`
	Remaining        int64          `json:"remaining"`
	YTDTarget        int64          `json:"ytd_target"`
	YTDSaved         int64          `json:"ytd_saved"`
	YTDRemaining     int64          `json:"ytd_remaining"`
	Months           []SavingsMonth `json:"months"`            // active months, ascending
	ExcludedAccounts []string       `json:"excluded_accounts"` // descendants skipped for a different currency in Month
}

// SavingsMonth is one active month in a row's year-to-date breakdown.
type SavingsMonth struct {
	Month  string `json:"month"`
	Target int64  `json:"target"`
	Saved  int64  `json:"saved"`
}
