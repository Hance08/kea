package transaction

import (
	"fmt"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type ShowView interface {
	Render(input *service.TransactionDetail, isCreate bool) error
}

type ShowProvider interface {
	GetTransactionByID(txID int64) (*service.TransactionDetail, error)
}

type showRunner struct {
	svc  ShowProvider
	view ShowView
}

func NewShowCmd(svc *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "show <transaction-id>",
		Short: "Show transaction details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &showRunner{
				svc:  svc.Transaction,
				view: views.NewTransactionDetailView(),
			}
			return runner.Run(args)
		},
	}
}

func (r *showRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	detail, err := r.svc.GetTransactionByID(txID)
	if err != nil {
		pterm.Error.Printf("Failed to get transaction: %v\n", err)
		return nil
	}

	if err := r.view.Render(detail, false); err != nil {
		return err
	}
	return nil
}
