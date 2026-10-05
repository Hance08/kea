// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBudgetSvc struct {
	lastSet   model.SetBudgetInput
	lastStop  model.StopBudgetInput
	lastMonth string
	deletedID int64
	list      []model.Budget
	report    *model.BudgetReport
	err       error
}

func (m *mockBudgetSvc) SetBudget(_ context.Context, in model.SetBudgetInput) (*model.Budget, error) {
	m.lastSet = in
	if m.err != nil {
		return nil, m.err
	}
	return &model.Budget{ID: 1, AccountName: in.AccountName, EffectiveMonth: in.EffectiveMonth, Amount: in.Amount}, nil
}

func (m *mockBudgetSvc) StopBudget(_ context.Context, in model.StopBudgetInput) (*model.Budget, error) {
	m.lastStop = in
	if m.err != nil {
		return nil, m.err
	}
	return &model.Budget{ID: 2, AccountName: in.AccountName, EffectiveMonth: in.EffectiveMonth, Stopped: true}, nil
}

func (m *mockBudgetSvc) ListBudgets(_ context.Context) ([]model.Budget, error) { return m.list, m.err }

func (m *mockBudgetSvc) DeleteBudget(_ context.Context, id int64) error {
	m.deletedID = id
	return m.err
}

func (m *mockBudgetSvc) GenerateBudgetReport(_ context.Context, month string) (*model.BudgetReport, error) {
	m.lastMonth = month
	if m.err != nil {
		return nil, m.err
	}
	return m.report, nil
}

func TestSetRunner_FlagMode(t *testing.T) {
	m := &mockBudgetSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), []string{"Expenses:Food", "8000.50"}, &setFlags{Month: "2026-01"}))
	assert.Equal(t, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 800050}, m.lastSet)
}

func TestSetRunner_DefaultsToCurrentMonth(t *testing.T) {
	m := &mockBudgetSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), []string{"Expenses:Food", "10"}, &setFlags{}))
	assert.Equal(t, time.Now().Format("2006-01"), m.lastSet.EffectiveMonth)
}

func TestSetRunner_BadAmount(t *testing.T) {
	r := &setRunner{svc: &mockBudgetSvc{}, out: &bytes.Buffer{}}
	err := r.Run(context.Background(), []string{"Expenses:Food", "abc"}, &setFlags{})
	assert.ErrorContains(t, err, "invalid amount")
}

func TestSetRunner_BadMonth(t *testing.T) {
	m := &mockBudgetSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	err := r.Run(context.Background(), []string{"Expenses:Food", "10"}, &setFlags{Month: "2026-13"})
	assert.Error(t, err)
	assert.Empty(t, m.lastSet.AccountName, "service must not be called with a bad month")
}

func TestSetRunner_PropagatesError(t *testing.T) {
	r := &setRunner{svc: &mockBudgetSvc{err: errors.New("boom")}, out: &bytes.Buffer{}}
	assert.ErrorContains(t, r.Run(context.Background(), []string{"Expenses:Food", "10"}, &setFlags{}), "boom")
}

func TestSetRunner_JSON(t *testing.T) {
	var out bytes.Buffer
	r := &setRunner{svc: &mockBudgetSvc{}, out: &out}
	require.NoError(t, r.Run(context.Background(), []string{"Expenses:Food", "12.5"}, &setFlags{Month: "2026-02", JSON: true}))
	assert.Contains(t, out.String(), `"amount": 12.5`)
	assert.Contains(t, out.String(), `"effective_month": "2026-02"`)
}

func TestStopRunner(t *testing.T) {
	m := &mockBudgetSvc{}
	r := &stopRunner{svc: m, month: "2026-04", out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), "Expenses:Food"))
	assert.Equal(t, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-04"}, m.lastStop)
}

func TestListRunner_FiltersByAccount(t *testing.T) {
	m := &mockBudgetSvc{list: []model.Budget{
		{ID: 1, AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 100},
		{ID: 2, AccountName: "Expenses:Rent", EffectiveMonth: "2026-01", Amount: 200},
	}}
	var out bytes.Buffer
	r := &listRunner{svc: m, account: "Expenses:Rent", json: true, out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Contains(t, out.String(), "Expenses:Rent")
	assert.NotContains(t, out.String(), "Expenses:Food")
}

func TestDeleteRunner(t *testing.T) {
	m := &mockBudgetSvc{}
	r := &deleteRunner{svc: m, yes: true, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), "42"))
	assert.Equal(t, int64(42), m.deletedID)

	err := (&deleteRunner{svc: m, yes: true, out: &bytes.Buffer{}}).Run(context.Background(), "x")
	assert.ErrorContains(t, err, "invalid budget id")
}

func TestReportRunner(t *testing.T) {
	m := &mockBudgetSvc{report: &model.BudgetReport{Month: "2026-03", Rows: []model.BudgetReportRow{}}}
	var out bytes.Buffer
	r := &reportRunner{svc: m, month: "2026-03", out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Equal(t, "2026-03", m.lastMonth)
	assert.Contains(t, out.String(), "No budgets in 2026-03")
}

func TestReportRunner_JSON(t *testing.T) {
	m := &mockBudgetSvc{report: &model.BudgetReport{
		Month:       "2026-03",
		Rows:        []model.BudgetReportRow{{AccountName: "Expenses:Food", Currency: "USD", Budget: 1000, Actual: 250, Remaining: 750}},
		TotalBudget: map[string]int64{"USD": 1000},
		TotalActual: map[string]int64{"USD": 250},
	}}
	var out bytes.Buffer
	r := &reportRunner{svc: m, month: "2026-03", json: true, out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Contains(t, out.String(), `"actual": 2.5`)
}
