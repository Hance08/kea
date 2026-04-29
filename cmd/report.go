package cmd

import (
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
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
  is       — Income statement: income vs expenses for a period (default)
  ib       — Income breakdown: ranked by income amount
  eb       — Expense breakdown: ranked by expense amount
  bs       — Balance sheet: current snapshot of all account balances

Examples:
  kea report
  kea report --type is --month 2026-01
  kea report --type ib --from 2026-01-01 --to 2026-01-31
  kea report --type eb --from 2026-01-01 --to 2026-01-31
  kea report --type bs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &reportRunner{
				flags:    flags,
				provider: svc.Transaction(),
				view:     newReportView(flags),
			}
			return runner.run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "report type: is | ib | eb | bs (default: is)")
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "calendar month to report on (YYYY-MM)")
	cmd.Flags().StringVar(&flags.From, "from", "", "range start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flags.To, "to", "", "range end date (YYYY-MM-DD)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output report as JSON (for scripting/automation)")

	return cmd
}
