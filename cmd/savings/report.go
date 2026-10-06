// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package savings

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

type SavingsReportProvider interface {
	GenerateSavingsReport(ctx context.Context, month string) (*model.SavingsReport, error)
}

type reportFlags struct {
	Month string
	JSON  bool
}

type reportRunner struct {
	svc   SavingsReportProvider
	month string
	json  bool
	out   io.Writer
}

func NewReportCmd(svc *service.Service) *cobra.Command {
	flags := &reportFlags{}
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show savings target vs saved for a month and year to date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &reportRunner{svc: svc.Savings(), month: flags.Month, json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "month to report on (YYYY-MM, default: current month)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output report as JSON")
	return cmd
}

func (r *reportRunner) Run(ctx context.Context) error {
	report, err := r.svc.GenerateSavingsReport(ctx, r.month)
	if err != nil {
		return fmt.Errorf("failed to generate savings report: %w", err)
	}
	if r.json {
		return writeJSON(r.out, views.ToJSONSavingsReport(report))
	}
	views.NewSavingsReportView(r.out).Render(report)
	return nil
}
