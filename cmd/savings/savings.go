// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package savings

import (
	"encoding/json"
	"io"
	"time"

	"github.com/hance08/kea/internal/service"
	"github.com/spf13/cobra"
)

func NewSavingsCmd(svc *service.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "savings",
		Aliases: []string{"sv"},
		Short:   "Manage monthly savings targets on Asset accounts",
		Long: `Set, stop, list and delete monthly savings targets on Asset accounts, and compare them with what was saved.
Saved is the account's balance change in the month (descendants included, opening balances excluded).
A target applies from its month until a later version replaces or stops it.`,
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
