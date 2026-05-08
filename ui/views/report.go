// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
	"fmt"
	"os"
	"sort"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/ui"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

// ReportView renders the three report types to stdout.
type ReportView struct{}

func NewReportView() *ReportView {
	return &ReportView{}
}

// RenderIncomeStatement prints an income vs expense table and a net summary.
func (v *ReportView) RenderIncomeStatement(result *model.ReportResult) error {
	ui.PrintL1Title("Income Statement  —  %s", result.Period)
	pterm.Println()

	// Income section
	if len(result.IncomeRows) > 0 {
		ui.PrintL2Title("Income")
		pterm.Println()

		t := newReportTable([]string{"Account", "Offset Account", "Transactions", "Amount", "Currency"})
		t.SetColumnAlignment([]int{
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_RIGHT,
			tablewriter.ALIGN_RIGHT,
			tablewriter.ALIGN_LEFT,
		})
		for _, row := range result.IncomeRows {
			t.Append([]string{
				row.AccountName,
				row.OffsetAccount,
				fmt.Sprintf("%d", row.TxCount),
				pterm.Green(utils.FormatAmount(row.Amount)),
				row.Currency,
			})
		}
		t.Render()
		pterm.Println()
	}

	// Expense section
	if len(result.ExpenseRows) > 0 {
		ui.PrintL2Title("Expenses")
		pterm.Println()

		t := newReportTable([]string{"Account", "Offset Account", "Transactions", "Amount", "Currency"})
		t.SetColumnAlignment([]int{
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_RIGHT,
			tablewriter.ALIGN_RIGHT,
			tablewriter.ALIGN_LEFT,
		})
		for _, row := range result.ExpenseRows {
			t.Append([]string{
				row.AccountName,
				row.OffsetAccount,
				fmt.Sprintf("%d", row.TxCount),
				pterm.Red(utils.FormatAmount(row.Amount)),
				row.Currency,
			})
		}
		t.Render()
		pterm.Println()
	}

	// Summary
	v.renderPerCurrencyTotals("Total Income", result.TotalIncome, pterm.Green)
	v.renderPerCurrencyTotals("Total Expenses", result.TotalExpense, pterm.Red)

	for _, ccy := range sortedKeys(result.NetAmount) {
		net := result.NetAmount[ccy]
		netStr := utils.FormatAmount(utils.AbsInt64(net))
		if net >= 0 {
			v.renderSummaryLine("Net", pterm.Green(netStr), ccy)
		} else {
			v.renderSummaryLine("Net", pterm.Red("-"+netStr), ccy)
		}
	}

	for _, ccy := range sortedKeys(result.NetWorth) {
		nw := result.NetWorth[ccy]
		nwStr := utils.FormatAmount(utils.AbsInt64(nw))
		if nw >= 0 {
			v.renderSummaryLine("Net Worth", pterm.Green(nwStr), ccy)
		} else {
			v.renderSummaryLine("Net Worth", pterm.Red("-"+nwStr), ccy)
		}
	}

	growthCurrencies := make([]string, 0, len(result.NetWorthGrowthPct))
	for ccy := range result.NetWorthGrowthPct {
		growthCurrencies = append(growthCurrencies, ccy)
	}
	sort.Strings(growthCurrencies)

	if len(growthCurrencies) == 0 {
		v.renderSummaryLineNoCurrency("Net Worth Growth", "N/A")
	} else {
		for _, ccy := range growthCurrencies {
			pct := result.NetWorthGrowthPct[ccy]
			pctText := fmt.Sprintf("%+.2f%%", pct)
			switch {
			case pct > 0:
				v.renderSummaryLine("Net Worth Growth", pterm.Green(pctText), ccy)
			case pct < 0:
				v.renderSummaryLine("Net Worth Growth", pterm.Red(pctText), ccy)
			default:
				v.renderSummaryLine("Net Worth Growth", pctText, ccy)
			}
		}
	}

	pterm.Println()
	return nil
}

// RenderExpenseBreakdown prints a ranked expense table.
func (v *ReportView) RenderExpenseBreakdown(result *model.ReportResult) error {
	ui.PrintL1Title("Expense Breakdown  —  %s", result.Period)
	pterm.Println()

	if len(result.ExpenseRows) == 0 {
		pterm.Warning.Println("No expenses found in this period.")
		return nil
	}

	t := newReportTable([]string{"Rank", "Account", "Offset Account", "Transactions", "Amount", "Currency"})
	t.SetColumnAlignment([]int{
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_LEFT,
	})

	for i, row := range result.ExpenseRows {
		t.Append([]string{
			fmt.Sprintf("%d", i+1),
			row.AccountName,
			row.OffsetAccount,
			fmt.Sprintf("%d", row.TxCount),
			pterm.Red(utils.FormatAmount(row.Amount)),
			row.Currency,
		})
	}
	t.Render()
	pterm.Println()

	v.renderPerCurrencyTotals("Total Expenses", result.TotalExpense, pterm.Red)
	pterm.Println()

	return nil
}

// RenderIncomeBreakdown prints a ranked income table.
func (v *ReportView) RenderIncomeBreakdown(result *model.ReportResult) error {
	ui.PrintL1Title("Income Breakdown  —  %s", result.Period)
	pterm.Println()

	if len(result.IncomeRows) == 0 {
		pterm.Warning.Println("No income found in this period.")
		return nil
	}

	t := newReportTable([]string{"Rank", "Account", "Offset Account", "Transactions", "Amount", "Currency"})
	t.SetColumnAlignment([]int{
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_LEFT,
	})

	for i, row := range result.IncomeRows {
		t.Append([]string{
			fmt.Sprintf("%d", i+1),
			row.AccountName,
			row.OffsetAccount,
			fmt.Sprintf("%d", row.TxCount),
			pterm.Green(utils.FormatAmount(row.Amount)),
			row.Currency,
		})
	}
	t.Render()
	pterm.Println()

	v.renderPerCurrencyTotals("Total Income", result.TotalIncome, pterm.Green)
	pterm.Println()

	return nil
}

// RenderBalanceSheet prints asset, liability, and equity tables with a net-worth summary.
func (v *ReportView) RenderBalanceSheet(result *model.BalanceSheetResult) error {
	ui.PrintL1Title("Balance Sheet")
	pterm.Println()

	// Assets
	ui.PrintL2Title("Assets")
	pterm.Println()
	if len(result.Assets) == 0 {
		pterm.Println(pterm.Red("(none)"))
	} else {
		t := newReportTable([]string{"Account", "Balance", "Currency"})
		t.SetColumnAlignment([]int{
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_RIGHT,
			tablewriter.ALIGN_LEFT,
		})
		for _, row := range result.Assets {
			t.Append([]string{
				row.AccountName,
				pterm.Green(utils.FormatAmount(row.Amount)),
				row.Currency,
			})
		}
		t.Render()
	}
	pterm.Println()

	// Liabilities
	ui.PrintL2Title("Liabilities")
	pterm.Println()
	if len(result.Liabilities) == 0 {
		pterm.Println(pterm.Red("(none)"))
	} else {
		t := newReportTable([]string{"Account", "Balance", "Currency"})
		t.SetColumnAlignment([]int{
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_RIGHT,
			tablewriter.ALIGN_LEFT,
		})
		for _, row := range result.Liabilities {
			t.Append([]string{
				row.AccountName,
				pterm.Red(utils.FormatAmount(row.Amount)),
				row.Currency,
			})
		}
		t.Render()
	}
	pterm.Println()

	// Equity
	ui.PrintL2Title("Equity")
	pterm.Println()
	if len(result.Equity) == 0 {
		pterm.Println(pterm.Red("(none)"))
	} else {
		t := newReportTable([]string{"Account", "Balance", "Currency"})
		t.SetColumnAlignment([]int{
			tablewriter.ALIGN_LEFT,
			tablewriter.ALIGN_RIGHT,
			tablewriter.ALIGN_LEFT,
		})
		for _, row := range result.Equity {
			t.Append([]string{
				row.AccountName,
				utils.FormatAmount(row.Amount),
				row.Currency,
			})
		}
		t.Render()
	}
	pterm.Println()

	// Summary
	v.renderPerCurrencyTotals("Total Assets", result.TotalAssets, pterm.Green)
	v.renderPerCurrencyTotals("Total Liabilities", result.TotalLiabilities, pterm.Red)

	for _, ccy := range sortedKeys(result.NetWorth) {
		nw := result.NetWorth[ccy]
		nwStr := utils.FormatAmount(utils.AbsInt64(nw))
		if nw >= 0 {
			v.renderSummaryLine("Net Worth", pterm.Green(nwStr), ccy)
		} else {
			v.renderSummaryLine("Net Worth", pterm.Red("-"+nwStr), ccy)
		}
	}

	pterm.Println()
	return nil
}

// newReportTable creates a consistently-styled tablewriter table.
func newReportTable(headers []string) *tablewriter.Table {
	t := tablewriter.NewWriter(os.Stdout)
	t.SetHeader(headers)
	t.SetBorder(false)
	t.SetHeaderLine(true)
	t.SetAutoWrapText(false)
	return t
}

// renderSummaryLine prints a key–value summary row with aligned formatting.
func (v *ReportView) renderSummaryLine(label, value, currency string) {
	pterm.Printf("  %-20s  %s  %s\n", label, value, currency)
}

func (v *ReportView) renderSummaryLineNoCurrency(label, value string) {
	pterm.Printf("  %-20s  %s\n", label, value)
}

func (v *ReportView) renderPerCurrencyTotals(label string, totals map[string]int64, colorFn func(string) string) {
	currencies := sortedKeys(totals)
	for _, ccy := range currencies {
		amt := totals[ccy]
		v.renderSummaryLine(label, colorFn(utils.FormatAmount(amt)), ccy)
	}
}

func sortedKeys(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
