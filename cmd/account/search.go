// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package account

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type searchFlags struct {
	Query    string
	Type     string
	Currency string
	Limit    int
	JSON     bool
}

type AccountSearchProvider interface {
	SearchAccounts(ctx context.Context, filter model.AccountFilter, opts model.ListOptions) (*model.ListResult[*model.Account], error)
	GetAccountBalance(ctx context.Context, id int64) (int64, error)
	GetAccountBalanceFormatted(ctx context.Context, id int64) (string, error)
}

type searchRunner struct {
	svc   AccountSearchProvider
	flags *searchFlags
}

func NewSearchCmd(svc *service.Service) *cobra.Command {
	flags := &searchFlags{}

	cmd := &cobra.Command{
		Use:     "search <query>",
		Aliases: []string{"s", "find"},
		Short:   "Search accounts by partial name match.",
		Long: `Search for accounts whose name contains the given query string.
The search is case-insensitive and matches anywhere in the account name.
Hidden accounts and system accounts are excluded from results.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				flags.Query = args[0]
			}
			runner := &searchRunner{
				svc:   svc.Account(),
				flags: flags,
			}
			return runner.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "Filter by account type (A, L, C, R, E)")
	cmd.Flags().StringVarP(&flags.Currency, "currency", "c", "", "Filter by currency code")
	cmd.Flags().IntVarP(&flags.Limit, "limit", "n", 20, "Maximum number of results")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "Output as JSON")

	return cmd
}

func (r *searchRunner) Run(ctx context.Context) error {
	filter := model.AccountFilter{}
	if r.flags.Query != "" {
		filter.Query = &r.flags.Query
	}
	if r.flags.Type != "" {
		at := model.AccountType(r.flags.Type)
		filter.Type = &at
	}
	if r.flags.Currency != "" {
		filter.Currency = &r.flags.Currency
	}

	opts := model.ListOptions{
		Limit:        r.flags.Limit,
		IncludeCount: true,
	}

	result, err := r.svc.SearchAccounts(ctx, filter, opts)
	if err != nil {
		return fmt.Errorf("failed to search accounts: %w", err)
	}

	if r.flags.JSON {
		items := make([]views.JSONAccount, 0, len(result.Items))
		for _, acc := range result.Items {
			bal, err := r.svc.GetAccountBalance(ctx, acc.ID)
			if err != nil {
				return fmt.Errorf("failed to get balance for %s: %w", acc.Name, err)
			}
			items = append(items, views.ToJSONAccount(acc, bal))
		}
		return views.WriteJSON(items)
	}

	if len(result.Items) == 0 {
		fmt.Println("No accounts found.")
		return nil
	}

	return views.NewAccountListView().Render(result.Items, func(id int64) (string, error) {
		return r.svc.GetAccountBalanceFormatted(ctx, id)
	})
}
