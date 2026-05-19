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

type listFlags struct {
	Type       string
	ShowHidden bool
	JSON       bool
}

type AccountListProvider interface {
	ListAccounts(ctx context.Context, opts service.ListAccountsOptions) ([]*model.Account, error)
	GetAccountBalance(ctx context.Context, id int64) (int64, error)
	GetAccountBalanceFormatted(ctx context.Context, id int64) (string, error)
}

type listRunner struct {
	svc   AccountListProvider
	flags *listFlags
}

func NewListCmd(svc *service.Service) *cobra.Command {
	flags := &listFlags{}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "List all accounts with their balances.",
		Long: `List all accounts in the system with their current balances.
You can filter by account type or show hidden accounts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &listRunner{
				svc:   svc.Account(),
				flags: flags,
			}
			return runner.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "Filter accounts by type (A, L, C, R, E)")
	cmd.Flags().BoolVar(&flags.ShowHidden, "show-hidden", false, "Show hidden accounts")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output as JSON")

	return cmd
}

func (r *listRunner) Run(ctx context.Context) error {
	opts := service.ListAccountsOptions{
		ShowHidden: r.flags.ShowHidden,
	}
	if r.flags.Type != "" {
		at := model.AccountType(r.flags.Type)
		opts.Type = &at
	}

	accounts, err := r.svc.ListAccounts(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	if r.flags.JSON {
		items := make([]views.JSONAccount, 0, len(accounts))
		for _, acc := range accounts {
			bal, err := r.svc.GetAccountBalance(ctx, acc.ID)
			if err != nil {
				return fmt.Errorf("failed to get balance for %s: %w", acc.Name, err)
			}
			items = append(items, views.ToJSONAccount(acc, bal))
		}
		return views.WriteJSON(items)
	}
	return views.NewAccountListView().Render(accounts, func(id int64) (string, error) {
		return r.svc.GetAccountBalanceFormatted(ctx, id)
	})
}
