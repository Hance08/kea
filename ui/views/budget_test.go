// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/pterm/pterm"
	"github.com/stretchr/testify/assert"
)

func TestBudgetUsedLabel(t *testing.T) {
	assert.Equal(t, "73%", BudgetUsedLabel(10000, 7320))
	assert.Equal(t, "107%", BudgetUsedLabel(6000, 6410))
	assert.Equal(t, "", BudgetUsedLabel(0, 0))
	assert.Equal(t, "over", BudgetUsedLabel(0, 1))
	assert.Equal(t, "0%", BudgetUsedLabel(0, -500)) // refund only, zero budget
	assert.Equal(t, "-5%", BudgetUsedLabel(10000, -500))
}

func sampleBudgetReport() *model.BudgetReport {
	return &model.BudgetReport{
		Month: "2026-10",
		Rows: []model.BudgetReportRow{
			{AccountName: "Expenses:Food", Currency: "TWD", Budget: 1000000, Actual: 732050, ActualIrregular: 732050, Remaining: 267950, ExcludedAccounts: []string{"Expenses:Food:Japan"}},
			{AccountName: "Expenses:Food:Dining", Currency: "TWD", Budget: 600000, Actual: 641000, ActualIrregular: 641000, Remaining: -41000, ExcludedAccounts: []string{}},
			{AccountName: "Expenses:Travel", Currency: "USD", Budget: 50000, Actual: 0, Remaining: 50000, ExcludedAccounts: []string{}},
		},
		TotalBudget: map[string]int64{"TWD": 1000000, "USD": 50000},
		TotalActual: map[string]int64{"TWD": 732050, "USD": 0},
	}
}

func TestBudgetReportView_Render(t *testing.T) {
	pterm.DisableColor()
	defer pterm.EnableColor()

	var buf bytes.Buffer
	NewBudgetReportView(&buf).Render(sampleBudgetReport())
	out := buf.String()

	assert.Contains(t, out, "2026-10")
	assert.Contains(t, out, "  Expenses:Food:Dining", "child row is indented under its budgeted parent")
	assert.Contains(t, out, "107%")
	assert.Contains(t, out, "-410")
	assert.Contains(t, out, "Total (TWD)")
	assert.Contains(t, out, "Total (USD)")
	assert.Contains(t, out, "Expenses:Food: 1 sub-account excluded (different currency): Expenses:Food:Japan")
}

func TestBudgetReportView_Empty(t *testing.T) {
	var buf bytes.Buffer
	NewBudgetReportView(&buf).Render(&model.BudgetReport{Month: "2026-10", Rows: []model.BudgetReportRow{}})
	assert.Contains(t, buf.String(), "No budgets in 2026-10")
}

func TestBudgetListView_Render(t *testing.T) {
	pterm.DisableColor()
	defer pterm.EnableColor()

	var buf bytes.Buffer
	NewBudgetListView(&buf).Render([]model.Budget{
		{ID: 7, AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 800000},
		{ID: 9, AccountName: "Expenses:Food", EffectiveMonth: "2026-06", Stopped: true},
	})
	out := buf.String()
	assert.Contains(t, out, "7")
	assert.Contains(t, out, "8,000")
	assert.Contains(t, out, "stopped")
	assert.Equal(t, 1, strings.Count(out, "2026-06"))
}

func TestToJSONBudgetReport(t *testing.T) {
	j := ToJSONBudgetReport(sampleBudgetReport())
	assert.Equal(t, "2026-10", j.Month)
	assert.InDelta(t, 7320.5, j.Rows[0].Actual, 0.001)
	assert.InDelta(t, -410.0, j.Rows[1].Remaining, 0.001)
	assert.InDelta(t, 10000.0, j.TotalBudget["TWD"], 0.001)
	assert.Equal(t, []string{"Expenses:Food:Japan"}, j.Rows[0].ExcludedAccounts)
}

func TestToJSONBudget(t *testing.T) {
	j := ToJSONBudget(model.Budget{ID: 1, AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 12345})
	assert.InDelta(t, 123.45, j.Amount, 0.001)
	assert.Equal(t, "Expenses:Food", j.AccountName)
}
