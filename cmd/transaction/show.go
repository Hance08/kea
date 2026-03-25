package transaction

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type ShowView interface {
	Render(input *model.TransactionDetail, isCreate bool) error
}

type ShowProvider interface {
	GetTransactionByID(txID int64) (*model.TransactionDetail, error)
}

type showFlags struct {
	JSONOut bool
}

type showRunner struct {
	svc  ShowProvider
	view ShowView
	json bool
}

func NewShowCmd(svc *service.Service) *cobra.Command {
	flags := &showFlags{}
	cmd := &cobra.Command{
		Use:   "show <transaction-id>",
		Short: "Show transaction details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &showRunner{
				svc:  svc.Transaction(),
				view: views.NewTransactionDetailView(),
				json: flags.JSONOut,
			}
			return runner.Run(args)
		},
	}
	cmd.Flags().BoolVarP(&flags.JSONOut, "json", "j", false, "output as JSON")
	return cmd
}

func (r *showRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	detail, err := r.svc.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if r.json {
		return views.WriteJSON(views.ToJSONTxDetail(detail))
	}
	return r.view.Render(detail, false)
}
