package cmd

import (
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func newReportView(flags reportFlags) ReportView {
	if flags.JSON {
		return views.NewJSONReportView()
	}
	return views.NewReportView()
}

func NewReportCmd(svc *service.Service) *cobra.Command {
	var flags reportFlags

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate financial reports",
		Long: `Generate financial reports for your accounts.

Report types:
  income   — Income statement: income vs expenses for a period (default)
  expense  — Expense breakdown: ranked by spending amount
  balance  — Balance sheet: current snapshot of all account balances

Examples:
  kea report
  kea report --type income --month 2026-01
  kea report --type expense --from 2026-01-01 --to 2026-01-31
  kea report --type balance`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &reportRunner{
				flags:    flags,
				provider: svc.Transaction,
				view:     newReportView(flags),
			}
			if err := r.run(); err != nil {
				pterm.Error.Println(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "report type: income | expense | balance (default: income)")
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "calendar month to report on (YYYY-MM)")
	cmd.Flags().StringVar(&flags.From, "from", "", "range start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flags.To, "to", "", "range end date (YYYY-MM-DD)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output report as JSON (for scripting/automation)")

	return cmd
}
