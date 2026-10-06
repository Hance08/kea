// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
	"fmt"
	"io"
	"strings"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

// savingsColor is yellow while part of the target is still missing and green
// once it is met or exceeded.
func savingsColor(remaining int64) func(...any) string {
	if remaining > 0 {
		return pterm.Yellow
	}
	return pterm.Green
}

// SavingsReportView renders a target vs saved table.
type SavingsReportView struct{ w io.Writer }

func NewSavingsReportView(w io.Writer) *SavingsReportView { return &SavingsReportView{w: w} }

func (v *SavingsReportView) Render(r *model.SavingsReport) {
	if len(r.Rows) == 0 {
		fmt.Fprintf(v.w, "No savings targets in %s. Set one with: kea savings set <account> <amount>\n", r.Month)
		return
	}
	fmt.Fprintf(v.w, "Savings report  %s\n\n", r.Month)

	t := tablewriter.NewWriter(v.w)
	t.SetHeader([]string{"Account", "Target", "Saved", "Remaining", "YTD Target", "YTD Saved", "YTD Remaining", "Currency"})
	t.SetBorder(false)
	t.SetHeaderLine(true)
	t.SetAutoWrapText(false)
	t.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT,
	})

	names := make([]string, len(r.Rows))
	for i, row := range r.Rows {
		names[i] = row.AccountName
	}
	for _, row := range r.Rows {
		t.Append([]string{
			strings.Repeat("  ", ancestorDepth(row.AccountName, names)) + row.AccountName,
			utils.FormatAmount(row.Target),
			utils.FormatAmount(row.Saved),
			savingsColor(row.Remaining)(utils.FormatAmount(row.Remaining)),
			utils.FormatAmount(row.YTDTarget),
			utils.FormatAmount(row.YTDSaved),
			savingsColor(row.YTDRemaining)(utils.FormatAmount(row.YTDRemaining)),
			row.Currency,
		})
	}
	for _, ccy := range sortedKeys(r.TotalTarget) {
		target, saved := r.TotalTarget[ccy], r.TotalSaved[ccy]
		ytdTarget, ytdSaved := r.TotalYTDTarget[ccy], r.TotalYTDSaved[ccy]
		t.Append([]string{
			fmt.Sprintf("Total (%s)", ccy),
			utils.FormatAmount(target),
			utils.FormatAmount(saved),
			savingsColor(target - saved)(utils.FormatAmount(target - saved)),
			utils.FormatAmount(ytdTarget),
			utils.FormatAmount(ytdSaved),
			savingsColor(ytdTarget - ytdSaved)(utils.FormatAmount(ytdTarget - ytdSaved)),
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

// ancestorDepth counts the other names that are ancestors of name.
func ancestorDepth(name string, names []string) int {
	depth := 0
	for _, n := range names {
		if n != name && strings.HasPrefix(name, n+":") {
			depth++
		}
	}
	return depth
}

// SavingsListView renders every savings target version.
type SavingsListView struct{ w io.Writer }

func NewSavingsListView(w io.Writer) *SavingsListView { return &SavingsListView{w: w} }

func (v *SavingsListView) Render(items []model.SavingsTarget) {
	if len(items) == 0 {
		fmt.Fprintln(v.w, "No savings targets. Set one with: kea savings set <account> <amount>")
		return
	}
	t := tablewriter.NewWriter(v.w)
	t.SetHeader([]string{"ID", "Account", "From", "Amount"})
	t.SetBorder(false)
	t.SetHeaderLine(true)
	t.SetAutoWrapText(false)
	t.SetColumnAlignment([]int{tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT})
	for _, s := range items {
		amount := utils.FormatAmount(s.Amount)
		if s.Stopped {
			amount = "stopped"
		}
		t.Append([]string{fmt.Sprintf("%d", s.ID), s.AccountName, s.EffectiveMonth, amount})
	}
	t.Render()
}
