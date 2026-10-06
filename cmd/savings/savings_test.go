// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package savings

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

type mockSavingsSvc struct {
	lastSet   model.SetSavingsTargetInput
	lastStop  model.StopSavingsTargetInput
	lastMonth string
	deletedID int64
	list      []model.SavingsTarget
	report    *model.SavingsReport
	err       error
}

func (m *mockSavingsSvc) SetSavingsTarget(_ context.Context, in model.SetSavingsTargetInput) (*model.SavingsTarget, error) {
	m.lastSet = in
	if m.err != nil {
		return nil, m.err
	}
	return &model.SavingsTarget{ID: 1, AccountName: in.AccountName, EffectiveMonth: in.EffectiveMonth, Amount: in.Amount}, nil
}

func (m *mockSavingsSvc) StopSavingsTarget(_ context.Context, in model.StopSavingsTargetInput) (*model.SavingsTarget, error) {
	m.lastStop = in
	if m.err != nil {
		return nil, m.err
	}
	return &model.SavingsTarget{ID: 2, AccountName: in.AccountName, EffectiveMonth: in.EffectiveMonth, Stopped: true}, nil
}

func (m *mockSavingsSvc) ListSavingsTargets(_ context.Context) ([]model.SavingsTarget, error) {
	return m.list, m.err
}

func (m *mockSavingsSvc) DeleteSavingsTarget(_ context.Context, id int64) error {
	m.deletedID = id
	return m.err
}

func (m *mockSavingsSvc) GenerateSavingsReport(_ context.Context, month string) (*model.SavingsReport, error) {
	m.lastMonth = month
	if m.err != nil {
		return nil, m.err
	}
	return m.report, nil
}

func TestSetRunner_FlagMode(t *testing.T) {
	m := &mockSavingsSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), []string{"Assets:Savings", "15000.50"}, &setFlags{Month: "2026-01"}))
	assert.Equal(t, model.SetSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-01", Amount: 1500050}, m.lastSet)
}

func TestSetRunner_DefaultsToCurrentMonth(t *testing.T) {
	m := &mockSavingsSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), []string{"Assets:Savings", "0"}, &setFlags{}))
	assert.Equal(t, time.Now().Format("2006-01"), m.lastSet.EffectiveMonth)
	assert.Equal(t, int64(0), m.lastSet.Amount)
}

func TestSetRunner_BadAmountAndMonth(t *testing.T) {
	m := &mockSavingsSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	assert.ErrorContains(t, r.Run(context.Background(), []string{"Assets:Savings", "abc"}, &setFlags{}), "invalid amount")
	assert.Error(t, r.Run(context.Background(), []string{"Assets:Savings", "10"}, &setFlags{Month: "2026-13"}))
	assert.Empty(t, m.lastSet.AccountName, "service must not be called with bad input")
}

func TestSetRunner_PropagatesError(t *testing.T) {
	r := &setRunner{svc: &mockSavingsSvc{err: errors.New("boom")}, out: &bytes.Buffer{}}
	assert.ErrorContains(t, r.Run(context.Background(), []string{"Assets:Savings", "10"}, &setFlags{}), "boom")
}

func TestSetRunner_JSON(t *testing.T) {
	var out bytes.Buffer
	r := &setRunner{svc: &mockSavingsSvc{}, out: &out}
	require.NoError(t, r.Run(context.Background(), []string{"Assets:Savings", "12.5"}, &setFlags{Month: "2026-02", JSON: true}))
	assert.Contains(t, out.String(), `"amount": 12.5`)
	assert.Contains(t, out.String(), `"effective_month": "2026-02"`)
}

func TestStopRunner(t *testing.T) {
	m := &mockSavingsSvc{}
	r := &stopRunner{svc: m, month: "2026-04", out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), "Assets:Savings"))
	assert.Equal(t, model.StopSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-04"}, m.lastStop)
}

func TestListRunner_FiltersByAccount(t *testing.T) {
	m := &mockSavingsSvc{list: []model.SavingsTarget{
		{ID: 1, AccountName: "Assets:Savings", EffectiveMonth: "2026-01", Amount: 100},
		{ID: 2, AccountName: "Assets:Trip", EffectiveMonth: "2026-01", Amount: 200},
	}}
	var out bytes.Buffer
	r := &listRunner{svc: m, account: "Assets:Trip", json: true, out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Contains(t, out.String(), "Assets:Trip")
	assert.NotContains(t, out.String(), "Assets:Savings")
}

func TestDeleteRunner(t *testing.T) {
	m := &mockSavingsSvc{}
	r := &deleteRunner{svc: m, yes: true, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), "42"))
	assert.Equal(t, int64(42), m.deletedID)

	err := (&deleteRunner{svc: m, yes: true, out: &bytes.Buffer{}}).Run(context.Background(), "x")
	assert.ErrorContains(t, err, "invalid savings target id")
}

func TestReportRunner(t *testing.T) {
	m := &mockSavingsSvc{report: &model.SavingsReport{Month: "2026-03", Rows: []model.SavingsReportRow{}}}
	var out bytes.Buffer
	r := &reportRunner{svc: m, month: "2026-03", out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Equal(t, "2026-03", m.lastMonth)
	assert.Contains(t, out.String(), "No savings targets in 2026-03")
}

func TestReportRunner_JSON(t *testing.T) {
	m := &mockSavingsSvc{report: &model.SavingsReport{
		Month: "2026-03",
		Rows: []model.SavingsReportRow{{AccountName: "Assets:Savings", Currency: "USD", Target: 1000, Saved: 250, Remaining: 750,
			Months: []model.SavingsMonth{{Month: "2026-03", Target: 1000, Saved: 250}}}},
		TotalTarget: map[string]int64{"USD": 1000},
		TotalSaved:  map[string]int64{"USD": 250},
	}}
	var out bytes.Buffer
	r := &reportRunner{svc: m, month: "2026-03", json: true, out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Contains(t, out.String(), `"saved": 2.5`)
	assert.Contains(t, out.String(), `"ytd_remaining"`)
}
