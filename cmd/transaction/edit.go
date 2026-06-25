// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package transaction

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type editRunner struct {
	txSvc  EditProvider
	accSvc AccountProvider
	view   EditView

	txID int64
}

func NewEditCmd(svc *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <transaction-id>",
		Short: "Edit a transaction",
		Long:  `Edit a transaction's description, date, status, and splits interactively.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &editRunner{
				txSvc:  svc.Transaction(),
				accSvc: svc.Account(),
				view:   views.NewTransactionEditView(),
			}
			return runner.Run(cmd.Context(), args)
		},
	}
}

func (r *editRunner) Run(ctx context.Context, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid transaction ID '%s': %w", args[0], err)
	}
	r.txID = id

	// Fetch Data
	detail, err := r.txSvc.GetTransactionByID(ctx, r.txID)
	if err != nil {
		r.view.ShowError("Failed to get transaction", err)
		return nil
	}

	isEditable, reason := r.txSvc.IsEditable(detail)
	if !isEditable {
		switch reason {
		case service.NotEditableReconciled:
			r.view.ShowWarning("This transaction cannot be edited (Reconciled Transaction)")
		}
		return nil
	}

	if err := r.view.RenderDetail(detail); err != nil {
		return err
	}

	return r.runEditMenu(ctx, detail)
}

func (r *editRunner) runEditMenu(ctx context.Context, detail *model.TransactionDetail) error {
	for {
		items := r.getAvailableMenuItems(ctx, detail)

		var options []string
		itemMap := make(map[string]menuItem)
		for _, item := range items {
			options = append(options, item.Label)
			itemMap[item.Label] = item
		}

		choice, err := r.view.AskSelection("What would you like to edit?", options)
		if err != nil {
			return err
		}

		selectedItem := itemMap[choice]

		if err := selectedItem.Action(detail); err != nil {
			if errors.Is(err, errExitLoop) {
				return nil
			}
			r.view.ShowError("Operation Failed", err)
		}
	}
}

func (r *editRunner) getAvailableMenuItems(ctx context.Context, detail *model.TransactionDetail) []menuItem {
	allActions := []menuItem{
		{
			Label:  OptBasicInfo,
			Action: r.actionEditBasicInfo,
		},
		{
			Label:     OptChangeType,
			Condition: func(d *model.TransactionDetail) bool { return d.Type != model.TxTypeOpening },
			Action: func(d *model.TransactionDetail) error {
				return r.actionEditType(ctx, d)
			},
		},
		{
			Label: OptToggleRegular,
			Condition: func(d *model.TransactionDetail) bool {
				return d.Type == model.TxTypeIncome || d.Type == model.TxTypeExpense
			},
			Action: func(d *model.TransactionDetail) error {
				return r.actionToggleRegular(d)
			},
		},
		{
			Label:     OptQuickAccount,
			Condition: func(d *model.TransactionDetail) bool { return len(d.Splits) == 2 },
			Action: func(d *model.TransactionDetail) error {
				return r.actionQuickChangeAccount(ctx, d)
			},
		},
		{
			Label:     OptQuickAmount,
			Condition: func(d *model.TransactionDetail) bool { return len(d.Splits) == 2 },
			Action:    r.actionQuickChangeAmount,
		},
		{
			Label: OptEditSplits,
			Action: func(d *model.TransactionDetail) error {
				return r.runSplitsMenu(ctx, d)
			},
		},
		{
			Label: OptSave,
			Action: func(d *model.TransactionDetail) error {
				if err := r.actionSave(ctx, d); err != nil {
					r.view.ShowError("Cannot save", err)
					r.view.ShowWarning("Please fix the errors before saving")
					return nil
				}
				return errExitLoop
			},
		},
		{
			Label: OptCancel,
			Action: func(d *model.TransactionDetail) error {
				r.view.ShowInfo("Changes discarded")
				return errExitLoop
			},
		},
	}

	var available []menuItem
	for _, item := range allActions {
		if item.Condition == nil || item.Condition(detail) {
			available = append(available, item)
		}
	}
	return available
}
