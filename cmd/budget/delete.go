// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/prompts"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type BudgetDeleteProvider interface {
	DeleteBudget(ctx context.Context, id int64) error
}

type deleteFlags struct {
	Yes  bool
	JSON bool
}

type deleteRunner struct {
	svc  BudgetDeleteProvider
	yes  bool
	json bool
	out  io.Writer
}

func NewDeleteCmd(svc *service.Service) *cobra.Command {
	flags := &deleteFlags{}
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"del"},
		Short:   "Delete one budget version (see `kea budget list` for IDs)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &deleteRunner{svc: svc.Budget(), yes: flags.Yes || flags.JSON, json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context(), args[0])
		},
	}
	cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "confirm deletion without interactive prompt")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON (implies --yes)")
	return cmd
}

func (r *deleteRunner) Run(ctx context.Context, rawID string) error {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid budget id %q", rawID)
	}
	if !r.yes {
		ok, err := prompts.PromptConfirm(fmt.Sprintf("Delete budget version #%d?", id), false)
		if err != nil {
			return err
		}
		if !ok {
			pterm.Info.Println("Deletion cancelled")
			return nil
		}
	}
	if err := r.svc.DeleteBudget(ctx, id); err != nil {
		return fmt.Errorf("failed to delete budget: %w", err)
	}
	if r.json {
		return writeJSON(r.out, map[string]any{"id": id, "deleted": true})
	}
	pterm.Success.Printf("Budget version #%d deleted\n", id)
	return nil
}
