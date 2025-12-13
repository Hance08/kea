package transaction

import (
	"fmt"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type ShowCommandRunner struct {
	svc *service.Service
}

func NewShowCmd(svc *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "show <transaction-id>",
		Short: "Show transaction details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &ShowCommandRunner{
				svc: svc,
			}
			return runner.Run(args)
		},
	}
}

func (r *ShowCommandRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	detail, err := r.svc.Transaction.GetTransactionByID(txID)
	if err != nil {
		pterm.Error.Printf("Failed to get transaction: %v\n", err)
		return nil
	}

	views.RenderTransactionDetail(detail)
	return nil
}
