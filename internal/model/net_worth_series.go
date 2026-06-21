// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// DailyBalancePoint is a single end-of-day net-worth value, in cents.
type DailyBalancePoint struct {
	Date    string `json:"date"`    // "YYYY-MM-DD" (UTC)
	Balance int64  `json:"balance"` // cents
}

// CurrencyDailySeries is the full daily net-worth history for one currency.
// Points are ordered ascending by date and dense (one per day from the first
// activity day through today) — gap days are front-filled from the prior day.
type CurrencyDailySeries struct {
	Currency string              `json:"currency"`
	Points   []DailyBalancePoint `json:"points"`
}
