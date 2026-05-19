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

type ListView interface {
	ShowWarning(format string, a ...any)
	Render(items []views.TransactionListItem, limit int) error
}

type ListProvider interface {
	GetTransactionHistory(ctx context.Context, accountName string, limit int) ([]*model.Transaction, error)
	GetRecentTransactions(ctx context.Context, limit int) ([]*model.Transaction, error)
	GetTransactionDetailsByIDs(ctx context.Context, txs []*model.Transaction) (map[int64]*model.TransactionDetail, error)
	BuildTransactionListItems(ctx context.Context, txs []*model.Transaction, details map[int64]*model.TransactionDetail) []model.TransactionListItem
}

type listFlags struct {
	Account string
	Limit   int
	JSON    bool
}

type listRunner struct {
	svc   ListProvider
	view  ListView
	flags *listFlags
}

func NewListCmd(svc *service.Service) *cobra.Command {
	flags := &listFlags{}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "List recent transactions (alias: tls)",
		Long: `List recent transactions from your accounting records.

This command displays a table of transactions with their details including
date, type, account, description, amount, and status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &listRunner{
				svc:   svc.Transaction(),
				view:  views.NewTransactionListView(),
				flags: flags,
			}
			return runner.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&flags.Account, "account", "a", "", "Filter transactions by account name")
	cmd.Flags().IntVarP(&flags.Limit, "limit", "l", 20, "Maximum number of transactions to display")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output as JSON")

	return cmd
}

func (r *listRunner) Run(ctx context.Context) error {
	// 1. Fetch Data
	transactions, err := r.fetchTransactions(ctx)
	if err != nil {
		return err
	}

	// 2. Transform Data (Model -> View Model)
	viewItems := r.buildViewItems(ctx, transactions)

	// 3. Render
	if r.flags.JSON {
		jsonItems := make([]views.JSONTxListItem, len(viewItems))
		for i, item := range viewItems {
			jsonItems[i] = views.ToJSONTxListItem(item)
		}
		return views.WriteJSON(jsonItems)
	}
	return r.view.Render(viewItems, r.flags.Limit)
}

func (r *listRunner) fetchTransactions(ctx context.Context) ([]*model.Transaction, error) {
	if r.flags.Account != "" {
		return r.svc.GetTransactionHistory(ctx, r.flags.Account, r.flags.Limit)
	}
	return r.svc.GetRecentTransactions(ctx, r.flags.Limit)
}

func (r *listRunner) buildViewItems(ctx context.Context, transactions []*model.Transaction) []views.TransactionListItem {
	detailsMap, err := r.svc.GetTransactionDetailsByIDs(ctx, transactions)
	if err != nil {
		if !r.flags.JSON {
			r.view.ShowWarning("Failed to load transaction details: %v\n", err)
		}
		return nil
	}

	items := r.svc.BuildTransactionListItems(ctx, transactions, detailsMap)

	viewItems := make([]views.TransactionListItem, len(items))
	for i, item := range items {
		amountFloat := float64(item.Amount) / 100.0
		viewItems[i] = views.TransactionListItem{
			ID:          item.ID,
			Date:        item.Date,
			Type:        item.Type,
			Account:     item.Account,
			Offset:      item.OffsetAccount,
			Description: item.Description,
			Amount:      fmt.Sprintf("%.2f", amountFloat),
			Currency:    item.Currency,
			Status:      item.Status,
		}
	}
	return viewItems
}
