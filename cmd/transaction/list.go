// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package transaction

import (
	"context"
	"fmt"
	"time"

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
	FilterTransactions(ctx context.Context, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error)
	FilterTransactionsByAccountName(ctx context.Context, accountName string, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error)
	GetTransactionDetailsByIDs(ctx context.Context, txs []*model.Transaction) (map[int64]*model.TransactionDetail, error)
	BuildTransactionListItems(ctx context.Context, txs []*model.Transaction, details map[int64]*model.TransactionDetail) []model.TransactionListItem
}

type listFlags struct {
	Account     string
	Limit       int
	JSON        bool
	Status      string
	Type        string
	From        string
	To          string
	Description string
	RegularSet  bool
	Regular     bool
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
			if cmd.Flags().Changed("regular") {
				flags.RegularSet = true
				flags.Regular, _ = cmd.Flags().GetBool("regular")
			}
			return runner.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&flags.Account, "account", "a", "", "Filter transactions by account name")
	cmd.Flags().IntVarP(&flags.Limit, "limit", "l", 20, "Maximum number of transactions to display")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output as JSON")
	cmd.Flags().StringVar(&flags.Status, "status", "", "Filter by status (pending, cleared, reconciled)")
	cmd.Flags().StringVar(&flags.Type, "type", "", "Filter by type (expense, income, transfer)")
	cmd.Flags().StringVar(&flags.From, "from", "", "Filter from date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flags.To, "to", "", "Filter to date (YYYY-MM-DD)")
	cmd.Flags().StringVarP(&flags.Description, "description", "d", "", "Filter by description (substring match)")
	cmd.Flags().Bool("regular", true, "Filter by regular flag (use --regular=false for irregular)")

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
	filter, err := r.buildFilter()
	if err != nil {
		return nil, err
	}

	opts := model.ListOptions{Limit: r.flags.Limit, IncludeCount: false}

	var result *model.ListResult[*model.Transaction]
	if r.flags.Account != "" {
		result, err = r.svc.FilterTransactionsByAccountName(ctx, r.flags.Account, filter, opts)
	} else {
		result, err = r.svc.FilterTransactions(ctx, filter, opts)
	}
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (r *listRunner) buildFilter() (model.TransactionFilter, error) {
	var filter model.TransactionFilter

	if r.flags.Status != "" {
		s, err := model.ParseTransactionStatus(r.flags.Status)
		if err != nil {
			return filter, err
		}
		filter.Status = &s
	}
	if r.flags.Type != "" {
		txType, err := model.ParseTransactionType(r.flags.Type)
		if err != nil {
			return filter, err
		}
		filter.Type = &txType
	}
	if r.flags.From != "" {
		t, err := time.Parse(model.DateFormat, r.flags.From)
		if err != nil {
			return filter, fmt.Errorf("invalid --from date: %w", err)
		}
		ts := t.Unix()
		filter.StartTime = &ts
	}
	if r.flags.To != "" {
		t, err := time.Parse(model.DateFormat, r.flags.To)
		if err != nil {
			return filter, fmt.Errorf("invalid --to date: %w", err)
		}
		ts := t.Add(24*time.Hour - time.Second).Unix()
		filter.EndTime = &ts
	}
	if r.flags.Description != "" {
		filter.Description = &r.flags.Description
	}
	if r.flags.RegularSet {
		v := r.flags.Regular
		filter.Regular = &v
	}

	return filter, nil
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
			Regular:     item.Regular,
		}
	}
	return viewItems
}
