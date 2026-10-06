// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func savingsCRUDFixture() (*mockAccountRepo, *mockSavingsRepo, *SavingsService) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Savings", Type: model.AccountTypeAsset, Currency: "USD"})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"})
	sRepo := newMockSavingsRepo(accRepo)
	return accRepo, sRepo, newTestSavingsService(accRepo, newMockTransactionRepo(), sRepo)
}

func requireField(t *testing.T, err error, field string) {
	t.Helper()
	var verr *ValidationError
	require.True(t, errors.As(err, &verr), "expected ValidationError, got %v", err)
	assert.Equal(t, field, verr.Field)
}

func TestSetSavingsTarget(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a version", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		got, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: " Assets:Savings ", EffectiveMonth: "2026-10", Amount: 1500000})
		require.NoError(t, err)
		assert.Equal(t, "Assets:Savings", got.AccountName)
		assert.Equal(t, int64(1), got.AccountID)
		assert.Equal(t, "2026-10", got.EffectiveMonth)
		assert.Equal(t, int64(1500000), got.Amount)
		assert.False(t, got.Stopped)
	})

	t.Run("zero is a valid target", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		got, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-10", Amount: 0})
		require.NoError(t, err)
		assert.Equal(t, int64(0), got.Amount)
	})

	t.Run("validation", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		_, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: " ", EffectiveMonth: "2026-10", Amount: 1})
		requireField(t, err, "account_name")
		_, err = svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-13", Amount: 1})
		requireField(t, err, "effective_month")
		_, err = svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-10", Amount: -1})
		requireField(t, err, "amount")
	})

	t.Run("non-Asset account is rejected", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		_, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-10", Amount: 1})
		requireField(t, err, "account_name")
	})

	t.Run("unknown account is not found", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		_, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Assets:Nope", EffectiveMonth: "2026-10", Amount: 1})
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		_, sRepo, svc := savingsCRUDFixture()
		sRepo.upsertErr = errors.New("boom")
		_, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-10", Amount: 1})
		assert.ErrorContains(t, err, "boom")
	})
}

func TestStopSavingsTarget(t *testing.T) {
	ctx := context.Background()

	t.Run("stops an active target", func(t *testing.T) {
		_, sRepo, svc := savingsCRUDFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		got, err := svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-04"})
		require.NoError(t, err)
		assert.True(t, got.Stopped)
		assert.Equal(t, int64(0), got.Amount)
		assert.Empty(t, activeVersions(mustListTargets(t, svc), "2026-04", savingsVersionKey))
		assert.Len(t, activeVersions(mustListTargets(t, svc), "2026-03", savingsVersionKey), 1)
	})

	t.Run("no active target is a validation error", func(t *testing.T) {
		_, sRepo, svc := savingsCRUDFixture()
		_, err := svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-04"})
		requireField(t, err, "effective_month")

		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-02", 0, true)
		_, err = svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-04"})
		requireField(t, err, "effective_month")
	})

	t.Run("validation and type", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		_, err := svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "", EffectiveMonth: "2026-04"})
		requireField(t, err, "account_name")
		_, err = svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "x"})
		requireField(t, err, "effective_month")
		_, err = svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-04"})
		requireField(t, err, "account_name")
	})
}

func mustListTargets(t *testing.T, svc *SavingsService) []model.SavingsTarget {
	t.Helper()
	list, err := svc.ListSavingsTargets(context.Background())
	require.NoError(t, err)
	return list
}

func TestListAndDeleteSavingsTargets(t *testing.T) {
	ctx := context.Background()
	_, sRepo, svc := savingsCRUDFixture()

	list := mustListTargets(t, svc)
	assert.NotNil(t, list)
	assert.Empty(t, list)

	id, _ := sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
	list = mustListTargets(t, svc)
	require.Len(t, list, 1)
	assert.Equal(t, "Assets:Savings", list[0].AccountName)

	require.NoError(t, svc.DeleteSavingsTarget(ctx, id))
	assert.ErrorIs(t, svc.DeleteSavingsTarget(ctx, id), ErrNotFound)

	sRepo.listErr = errors.New("boom")
	_, err := svc.ListSavingsTargets(ctx)
	assert.ErrorContains(t, err, "boom")
}
