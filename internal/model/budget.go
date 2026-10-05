// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// Budget is one version of a monthly budget on an Expense account. It applies
// from EffectiveMonth (YYYY-MM) until a later version for the same account.
// A Stopped version ends the budget; its Amount is always 0.
type Budget struct {
	ID             int64  `json:"id"`
	AccountID      int64  `json:"account_id"`
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
	Amount         int64  `json:"amount"` // cents
	Stopped        bool   `json:"stopped"`
}

// SetBudgetInput sets (or replaces) the budget version starting at EffectiveMonth.
type SetBudgetInput struct {
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
	Amount         int64  `json:"amount"` // cents, >= 0
}

// StopBudgetInput ends an account's budget from EffectiveMonth on.
type StopBudgetInput struct {
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
}

// BudgetReport compares each active budget in Month with actual spending.
// Totals are per currency and only include budgeted rows with no budgeted
// ancestor in the same currency, so parent and child budgets are not double
// counted while other-currency children still count in their own currency.
type BudgetReport struct {
	Month       string            `json:"month"`
	Rows        []BudgetReportRow `json:"rows"`
	TotalBudget map[string]int64  `json:"total_budget"`
	TotalActual map[string]int64  `json:"total_actual"`
}

// BudgetReportRow is one budgeted account. Actual is the signed sum of Expense
// splits of Expense-typed transactions on the account and its descendants in
// Currency; refunds reduce it and it may be negative.
type BudgetReportRow struct {
	AccountID        int64    `json:"account_id"`
	AccountName      string   `json:"account_name"`
	Currency         string   `json:"currency"`
	EffectiveMonth   string   `json:"effective_month"`
	Budget           int64    `json:"budget"`
	Actual           int64    `json:"actual"`
	ActualRegular    int64    `json:"actual_regular"`
	ActualIrregular  int64    `json:"actual_irregular"`
	Remaining        int64    `json:"remaining"`
	ExcludedAccounts []string `json:"excluded_accounts"` // descendants skipped for a different currency
}
