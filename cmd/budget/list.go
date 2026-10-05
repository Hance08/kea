// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type BudgetListProvider interface {
	ListBudgets(ctx context.Context) ([]model.Budget, error)
}

type listFlags struct {
	Account string
	JSON    bool
}

type listRunner struct {
	svc     BudgetListProvider
	account string
	json    bool
	out     io.Writer
}

func NewListCmd(svc *service.Service) *cobra.Command {
	flags := &listFlags{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List every budget version",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &listRunner{svc: svc.Budget(), account: flags.Account, json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&flags.Account, "account", "", "only show versions of this account")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *listRunner) Run(ctx context.Context) error {
	items, err := r.svc.ListBudgets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list budgets: %w", err)
	}
	if r.account != "" {
		filtered := make([]model.Budget, 0, len(items))
		for _, b := range items {
			if b.AccountName == r.account {
				filtered = append(filtered, b)
			}
		}
		items = filtered
	}
	if r.json {
		return writeJSON(r.out, views.ToJSONBudgets(items))
	}
	views.NewBudgetListView(r.out).Render(items)
	return nil
}
