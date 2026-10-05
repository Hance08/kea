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

type BudgetReportProvider interface {
	GenerateBudgetReport(ctx context.Context, month string) (*model.BudgetReport, error)
}

type reportFlags struct {
	Month string
	JSON  bool
}

type reportRunner struct {
	svc   BudgetReportProvider
	month string
	json  bool
	out   io.Writer
}

func NewReportCmd(svc *service.Service) *cobra.Command {
	flags := &reportFlags{}
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show budget vs actual spending for a month",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &reportRunner{svc: svc.Budget(), month: flags.Month, json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "month to report on (YYYY-MM, default: current month)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output report as JSON")
	return cmd
}

func (r *reportRunner) Run(ctx context.Context) error {
	report, err := r.svc.GenerateBudgetReport(ctx, r.month)
	if err != nil {
		return fmt.Errorf("failed to generate budget report: %w", err)
	}
	if r.json {
		return writeJSON(r.out, views.ToJSONBudgetReport(report))
	}
	views.NewBudgetReportView(r.out).Render(report)
	return nil
}
