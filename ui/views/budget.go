// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

// Usage thresholds shared by the CLI color coding.
const (
	budgetWarnPct = 80
	budgetOverPct = 100
)

// BudgetUsedLabel formats actual/budget as an integer percent. A zero budget
// shows "over" when anything was spent and "" when nothing was.
func BudgetUsedLabel(budget, actual int64) string {
	if budget == 0 {
		switch {
		case actual > 0:
			return "over"
		case actual < 0:
			return "0%"
		default:
			return ""
		}
	}
	return fmt.Sprintf("%d%%", int64(math.Round(float64(actual)*100/float64(budget))))
}

// budgetColor picks the color for a row: red over budget, yellow from 80%.
func budgetColor(budget, actual int64) func(...any) string {
	switch {
	case budget == 0 && actual > 0, budget > 0 && actual*100 > budget*budgetOverPct:
		return pterm.Red
	case budget > 0 && actual*100 >= budget*budgetWarnPct:
		return pterm.Yellow
	default:
		return fmt.Sprint
	}
}

// BudgetReportView renders a budget vs actual table.
type BudgetReportView struct{ w io.Writer }

func NewBudgetReportView(w io.Writer) *BudgetReportView { return &BudgetReportView{w: w} }

func (v *BudgetReportView) Render(r *model.BudgetReport) {
	if len(r.Rows) == 0 {
		fmt.Fprintf(v.w, "No budgets in %s. Set one with: kea budget set <account> <amount>\n", r.Month)
		return
	}
	fmt.Fprintf(v.w, "Budget report  %s\n\n", r.Month)

	t := tablewriter.NewWriter(v.w)
	t.SetHeader([]string{"Account", "Budget", "Actual", "Regular", "Irregular", "Remaining", "Used", "Currency"})
	t.SetBorder(false)
	t.SetHeaderLine(true)
	t.SetAutoWrapText(false)
	t.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT,
	})

	for _, row := range r.Rows {
		color := budgetColor(row.Budget, row.Actual)
		t.Append([]string{
			strings.Repeat("  ", budgetDepth(row.AccountName, r.Rows)) + row.AccountName,
			utils.FormatAmount(row.Budget),
			utils.FormatAmount(row.Actual),
			utils.FormatAmount(row.ActualRegular),
			utils.FormatAmount(row.ActualIrregular),
			color(utils.FormatAmount(row.Remaining)),
			color(BudgetUsedLabel(row.Budget, row.Actual)),
			row.Currency,
		})
	}
	for _, ccy := range sortedKeys(r.TotalBudget) {
		budget, actual := r.TotalBudget[ccy], r.TotalActual[ccy]
		color := budgetColor(budget, actual)
		t.Append([]string{
			fmt.Sprintf("Total (%s)", ccy),
			utils.FormatAmount(budget),
			utils.FormatAmount(actual),
			"", "",
			color(utils.FormatAmount(budget - actual)),
			color(BudgetUsedLabel(budget, actual)),
			ccy,
		})
	}
	t.Render()

	for _, row := range r.Rows {
		if n := len(row.ExcludedAccounts); n > 0 {
			fmt.Fprintf(v.w, "! %s: %d sub-account excluded (different currency): %s\n",
				row.AccountName, n, strings.Join(row.ExcludedAccounts, ", "))
		}
	}
}

// budgetDepth counts the other budgeted rows that are ancestors of name.
func budgetDepth(name string, rows []model.BudgetReportRow) int {
	depth := 0
	for _, r := range rows {
		if r.AccountName != name && strings.HasPrefix(name, r.AccountName+":") {
			depth++
		}
	}
	return depth
}

// BudgetListView renders every budget version.
type BudgetListView struct{ w io.Writer }

func NewBudgetListView(w io.Writer) *BudgetListView { return &BudgetListView{w: w} }

func (v *BudgetListView) Render(items []model.Budget) {
	if len(items) == 0 {
		fmt.Fprintln(v.w, "No budgets. Set one with: kea budget set <account> <amount>")
		return
	}
	t := tablewriter.NewWriter(v.w)
	t.SetHeader([]string{"ID", "Account", "From", "Amount"})
	t.SetBorder(false)
	t.SetHeaderLine(true)
	t.SetAutoWrapText(false)
	t.SetColumnAlignment([]int{tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT})
	for _, b := range items {
		amount := utils.FormatAmount(b.Amount)
		if b.Stopped {
			amount = "stopped"
		}
		t.Append([]string{fmt.Sprintf("%d", b.ID), b.AccountName, b.EffectiveMonth, amount})
	}
	t.Render()
}
