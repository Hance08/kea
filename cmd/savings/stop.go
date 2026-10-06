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
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type SavingsStopProvider interface {
	StopSavingsTarget(ctx context.Context, input model.StopSavingsTargetInput) (*model.SavingsTarget, error)
}

type stopFlags struct {
	Month string
	JSON  bool
}

type stopRunner struct {
	svc   SavingsStopProvider
	month string
	json  bool
	out   io.Writer
}

func NewStopCmd(svc *service.Service) *cobra.Command {
	flags := &stopFlags{}
	cmd := &cobra.Command{
		Use:   "stop <account>",
		Short: "Stop an account's savings target from a month on",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &stopRunner{svc: svc.Savings(), month: monthOrCurrent(flags.Month), json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context(), args[0])
		},
	}
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "first month without a target (YYYY-MM, default: current month)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *stopRunner) Run(ctx context.Context, account string) error {
	t, err := r.svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: account, EffectiveMonth: r.month})
	if err != nil {
		return fmt.Errorf("failed to stop savings target: %w", err)
	}
	if r.json {
		return writeJSON(r.out, views.ToJSONSavingsTarget(*t))
	}
	pterm.Success.Printf("Savings target for %s stopped from %s\n", t.AccountName, t.EffectiveMonth)
	return nil
}
