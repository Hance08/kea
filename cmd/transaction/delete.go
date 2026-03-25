package transaction

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/prompts"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type TransactionDeleteView interface {
	RenderPreview(data views.TransactionDeletePreview) error
	ShowSuccess(id int64)
}

type TxDeleteProvider interface {
	GetTransactionByID(txID int64) (*model.TransactionDetail, error)
	DeleteTransaction(txID int64) error
}

type deleteFlags struct {
	Yes     bool
	JSON    bool
}

type deleteRunner struct {
	svc  TxDeleteProvider
	view TransactionDeleteView
	yes  bool
	json bool
}

func NewDeleteCmd(svc *service.Service) *cobra.Command {
	flags := &deleteFlags{}

	cmd := &cobra.Command{
		Use:     "delete <transaction-id>",
		Short:   "Delete a transaction",
		Long:    `Delete a transaction and all its associated splits. This action cannot be undone.`,
		Aliases: []string{"del", "d"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &deleteRunner{
				svc:  svc.Transaction(),
				view: views.NewTransactionDeleteView(),
				yes:  flags.Yes || flags.JSON,
				json: flags.JSON,
			}
			return runner.Run(args)
		},
	}

	cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "confirm deletion without interactive prompt")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")

	return cmd
}

func (r *deleteRunner) Run(args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	detail, err := r.svc.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if !r.json {
		if err := r.view.RenderPreview(views.TransactionDeletePreview{
			ID:          detail.ID,
			Timestamp:   detail.Timestamp,
			Description: detail.Description,
			SplitCount:  len(detail.Splits),
		}); err != nil {
			return err
		}
	}

	if !r.yes {
		confirmation, err := prompts.PromptConfirm("Do you want to delete this transaction?", false)
		if err != nil {
			return err
		}
		if !confirmation {
			pterm.Info.Println("Deletion cancelled")
			return nil
		}
	}

	if err := r.svc.DeleteTransaction(txID); err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}

	if r.json {
		return views.WriteJSON(map[string]any{"id": txID, "deleted": true})
	}
	r.view.ShowSuccess(txID)
	return nil
}
