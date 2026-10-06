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

func sampleSavingsReport() *model.SavingsReport {
	return &model.SavingsReport{
		Month: "2026-10",
		Rows: []model.SavingsReportRow{
			{AccountName: "Assets:Savings", Currency: "TWD", Target: 1500000, Saved: 1200000, Remaining: 300000,
				YTDTarget: 15000000, YTDSaved: 15650000, YTDRemaining: -650000,
				Months:           []model.SavingsMonth{{Month: "2026-10", Target: 1500000, Saved: 1200000}},
				ExcludedAccounts: []string{"Assets:Savings:USD"}},
			{AccountName: "Assets:Savings:Travel", Currency: "TWD", Target: 200000, Saved: -50000, Remaining: 250000,
				Months: []model.SavingsMonth{}, ExcludedAccounts: []string{}},
			{AccountName: "Assets:Broker", Currency: "USD", Target: 0, Saved: 0, Remaining: 0,
				Months: []model.SavingsMonth{}, ExcludedAccounts: []string{}},
		},
		TotalTarget:    map[string]int64{"TWD": 1500000, "USD": 0},
		TotalSaved:     map[string]int64{"TWD": 1200000, "USD": 0},
		TotalYTDTarget: map[string]int64{"TWD": 15000000, "USD": 0},
		TotalYTDSaved:  map[string]int64{"TWD": 15650000, "USD": 0},
	}
}

func TestSavingsReportView_Render(t *testing.T) {
	pterm.DisableColor()
	defer pterm.EnableColor()

	var buf bytes.Buffer
	NewSavingsReportView(&buf).Render(sampleSavingsReport())
	out := buf.String()

	assert.Contains(t, out, "Savings report  2026-10")
	var parentLine, childLine string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		switch {
		case strings.HasPrefix(trimmed, "Assets:Savings:Travel"):
			childLine = line
		case strings.HasPrefix(trimmed, "Assets:Savings "):
			parentLine = line
		}
	}
	assert.NotEmpty(t, parentLine)
	assert.NotEmpty(t, childLine)
	indent := func(l string) int { return len(l) - len(strings.TrimLeft(l, " ")) }
	assert.Greater(t, indent(childLine), indent(parentLine), "child row is indented under its targeted parent")
	assert.Contains(t, parentLine, "3,000")
	assert.Contains(t, parentLine, "-6,500")
	assert.Contains(t, childLine, "-500")
	assert.Contains(t, out, "Total (TWD)")
	assert.Contains(t, out, "Total (USD)")
	assert.Contains(t, out, "Assets:Savings: 1 sub-account excluded (different currency): Assets:Savings:USD")
}

func TestSavingsColor(t *testing.T) {
	// Compare against pterm's own functions so the result does not depend on
	// whether another test left color enabled or disabled.
	assert.Equal(t, pterm.Yellow("x"), savingsColor(1)("x"), "still missing")
	assert.Equal(t, pterm.Green("x"), savingsColor(0)("x"), "met")
	assert.Equal(t, pterm.Green("x"), savingsColor(-1)("x"), "exceeded")
}

func TestSavingsReportView_Empty(t *testing.T) {
	var buf bytes.Buffer
	NewSavingsReportView(&buf).Render(&model.SavingsReport{Month: "2026-10", Rows: []model.SavingsReportRow{}})
	assert.Contains(t, buf.String(), "No savings targets in 2026-10")
}

func TestSavingsListView_Render(t *testing.T) {
	pterm.DisableColor()
	defer pterm.EnableColor()

	var buf bytes.Buffer
	NewSavingsListView(&buf).Render([]model.SavingsTarget{
		{ID: 7, AccountName: "Assets:Savings", EffectiveMonth: "2026-01", Amount: 1500000},
		{ID: 9, AccountName: "Assets:Savings", EffectiveMonth: "2026-06", Stopped: true},
	})
	out := buf.String()
	assert.Contains(t, out, "7")
	assert.Contains(t, out, "15,000")
	assert.Contains(t, out, "stopped")

	buf.Reset()
	NewSavingsListView(&buf).Render([]model.SavingsTarget{})
	assert.Contains(t, buf.String(), "No savings targets")
}

func TestToJSONSavingsReport(t *testing.T) {
	j := ToJSONSavingsReport(sampleSavingsReport())
	assert.Equal(t, "2026-10", j.Month)
	assert.InDelta(t, 12000.0, j.Rows[0].Saved, 0.001)
	assert.InDelta(t, -6500.0, j.Rows[0].YTDRemaining, 0.001)
	assert.Equal(t, "2026-10", j.Rows[0].Months[0].Month)
	assert.InDelta(t, 15000.0, j.Rows[0].Months[0].Target, 0.001)
	assert.NotNil(t, j.Rows[1].Months)
	assert.InDelta(t, 156500.0, j.TotalYTDSaved["TWD"], 0.001)
	assert.Equal(t, []string{"Assets:Savings:USD"}, j.Rows[0].ExcludedAccounts)

	empty := ToJSONSavingsReport(&model.SavingsReport{Month: "2026-10"})
	assert.NotNil(t, empty.Rows)
	assert.NotNil(t, empty.TotalTarget)
	assert.NotNil(t, empty.TotalYTDSaved)
}

func TestToJSONSavingsTarget(t *testing.T) {
	j := ToJSONSavingsTarget(model.SavingsTarget{ID: 1, AccountName: "Assets:Savings", EffectiveMonth: "2026-01", Amount: 12345})
	assert.InDelta(t, 123.45, j.Amount, 0.001)
	assert.Equal(t, "Assets:Savings", j.AccountName)
	assert.Len(t, ToJSONSavingsTargets([]model.SavingsTarget{{ID: 1}, {ID: 2}}), 2)
}
