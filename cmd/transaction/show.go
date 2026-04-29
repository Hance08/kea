// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package transaction

import (
	"context"
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
	GetTransactionByID(ctx context.Context, txID int64) (*model.TransactionDetail, error)
}

type showFlags struct {
	JSON bool
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
				json: flags.JSON,
			}
			return runner.Run(cmd.Context(), args)
		},
	}
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output as JSON")
	return cmd
}

func (r *showRunner) Run(ctx context.Context, args []string) error {
	var txID int64
	if _, err := fmt.Sscanf(args[0], "%d", &txID); err != nil {
		return fmt.Errorf("invalid transaction ID: %s", args[0])
	}

	detail, err := r.svc.GetTransactionByID(ctx, txID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if r.json {
		return views.WriteJSON(views.ToJSONTxDetail(detail))
	}
	return r.view.Render(detail, false)
}
