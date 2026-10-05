// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"encoding/json"
	"io"
	"time"

	"github.com/hance08/kea/internal/service"
	"github.com/spf13/cobra"
)

func NewBudgetCmd(svc *service.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "budget",
		Aliases: []string{"bg"},
		Short:   "Manage monthly budgets on Expense accounts",
		Long: `Set, stop, list and delete monthly budgets on Expense accounts, and compare them with actual spending.
A budget applies from its month until a later version replaces or stops it.`,
	}
	cmd.AddCommand(NewSetCmd(svc))
	cmd.AddCommand(NewStopCmd(svc))
	cmd.AddCommand(NewListCmd(svc))
	cmd.AddCommand(NewDeleteCmd(svc))
	cmd.AddCommand(NewReportCmd(svc))
	return cmd
}

func currentMonth() string { return time.Now().Format("2006-01") }

func monthOrCurrent(m string) string {
	if m == "" {
		return currentMonth()
	}
	return m
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
