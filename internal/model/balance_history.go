// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// MonthlyBalancePoint is one end-of-month balance for an account, in cents,
// expressed in the account's natural-amount direction.
type MonthlyBalancePoint struct {
	Month   string `json:"month"`   // "YYYY-MM" (UTC year-month)
	Balance int64  `json:"balance"` // cents, natural-amount
}

// AccountMonthlySeries is the full monthly balance history for one account.
// The points slice is ordered ascending by month and starts at the account's
// earliest month with activity (no front-fill of zero months).
type AccountMonthlySeries struct {
	AccountID int64                 `json:"account_id"`
	Currency  string                `json:"currency"`
	Points    []MonthlyBalancePoint `json:"points"`
}
