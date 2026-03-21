package views

import (
	"fmt"
	"os"

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
	v.renderSummaryLine("Total Income", pterm.Green(utils.FormatAmount(result.TotalIncome)), result.Currency)
	v.renderSummaryLine("Total Expenses", pterm.Red(utils.FormatAmount(result.TotalExpense)), result.Currency)

	netStr := utils.FormatAmount(utils.AbsInt64(result.NetAmount))
	if result.NetAmount >= 0 {
		v.renderSummaryLine("Net", pterm.Green(netStr), result.Currency)
	} else {
		v.renderSummaryLine("Net", pterm.Red("-"+netStr), result.Currency)
	}

	// Net Worth: cumulative assets minus liabilities
	nwStr := utils.FormatAmount(utils.AbsInt64(result.NetWorth))
	if result.NetWorth >= 0 {
		v.renderSummaryLine("Net Worth", pterm.Green(nwStr), result.Currency)
	} else {
		v.renderSummaryLine("Net Worth", pterm.Red("-"+nwStr), result.Currency)
	}

	if result.NetWorthGrowthPct == nil {
		v.renderSummaryLineNoCurrency("Net Worth Growth", "N/A")
	} else {
		pctText := fmt.Sprintf("%+.2f%%", *result.NetWorthGrowthPct)
		switch {
		case *result.NetWorthGrowthPct > 0:
			v.renderSummaryLineNoCurrency("Net Worth Growth", pterm.Green(pctText))
		case *result.NetWorthGrowthPct < 0:
			v.renderSummaryLineNoCurrency("Net Worth Growth", pterm.Red(pctText))
		default:
			v.renderSummaryLineNoCurrency("Net Worth Growth", pctText)
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

	v.renderSummaryLine("Total Expenses", pterm.Red(utils.FormatAmount(result.TotalExpense)), result.Currency)
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
		pterm.Info.Println("  (none)")
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
		pterm.Info.Println("  (none)")
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
		pterm.Info.Println("  (none)")
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
	v.renderSummaryLine("Total Assets", pterm.Green(utils.FormatAmount(result.TotalAssets)), result.Currency)
	v.renderSummaryLine("Total Liabilities", pterm.Red(utils.FormatAmount(result.TotalLiabilities)), result.Currency)

	nwStr := utils.FormatAmount(utils.AbsInt64(result.NetWorth))
	if result.NetWorth >= 0 {
		v.renderSummaryLine("Net Worth", pterm.Green(nwStr), result.Currency)
	} else {
		v.renderSummaryLine("Net Worth", pterm.Red("-"+nwStr), result.Currency)
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
