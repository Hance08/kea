// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/service"
)

func (r *reportRunner) run(ctx context.Context) error {
	reportType := r.flags.Type
	if reportType == "" {
		reportType = "is"
	}

	switch reportType {
	case "is":
		return r.runIncomeStatement(ctx)
	case "ib":
		return r.runIncomeBreakdown(ctx)
	case "eb":
		return r.runExpenseBreakdown(ctx)
	case "bs":
		return r.runBalanceSheet(ctx)
	default:
		return fmt.Errorf("unknown report type %q — use: is, ib, eb, bs", reportType)
	}
}

func (r *reportRunner) dateRangeParams() service.DateRangeParams {
	return service.DateRangeParams{
		Month: r.flags.Month,
		From:  r.flags.From,
		To:    r.flags.To,
	}
}

func (r *reportRunner) runIncomeStatement(ctx context.Context) error {
	result, err := r.provider.GenerateFullIncomeStatement(ctx, r.dateRangeParams())
	if err != nil {
		return err
	}
	return r.view.RenderIncomeStatement(result)
}

func (r *reportRunner) runExpenseBreakdown(ctx context.Context) error {
	result, err := r.provider.GenerateFullExpenseBreakdown(ctx, r.dateRangeParams())
	if err != nil {
		return err
	}
	return r.view.RenderExpenseBreakdown(result)
}

func (r *reportRunner) runIncomeBreakdown(ctx context.Context) error {
	result, err := r.provider.GenerateFullIncomeBreakdown(ctx, r.dateRangeParams())
	if err != nil {
		return err
	}
	return r.view.RenderIncomeBreakdown(result)
}

func (r *reportRunner) runBalanceSheet(ctx context.Context) error {
	_, end, _, err := r.provider.ResolveDateRange(r.dateRangeParams())
	if err != nil {
		return err
	}

	result, err := r.provider.GenerateBalanceSheet(ctx, end)
	if err != nil {
		return fmt.Errorf("failed to generate balance sheet: %w", err)
	}

	return r.view.RenderBalanceSheet(result)
}
