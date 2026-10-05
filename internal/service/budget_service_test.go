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

func budgetFixture() (*mockAccountRepo, *mockTransactionRepo, *mockBudgetRepo) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"})
	return accRepo, newMockTransactionRepo(), newMockBudgetRepo(accRepo)
}

func requireValidationField(t *testing.T, err error, field string) {
	t.Helper()
	var verr *ValidationError
	require.True(t, errors.As(err, &verr), "expected ValidationError, got %v", err)
	assert.Equal(t, field, verr.Field)
}

func TestValidateBudgetMonth(t *testing.T) {
	assert.NoError(t, ValidateBudgetMonth("2026-01"))
	assert.NoError(t, ValidateBudgetMonth("2026-12"))
	for _, bad := range []string{"", "2026-1", "2026-13", "2026-00", "26-01", "2026/01", "2026-01-01"} {
		requireValidationField(t, ValidateBudgetMonth(bad), "effective_month")
	}
}

func TestSetBudget(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a version", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		b, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 800000})
		require.NoError(t, err)
		assert.Equal(t, int64(1), b.AccountID)
		assert.Equal(t, "Expenses:Food", b.AccountName)
		assert.Equal(t, int64(800000), b.Amount)
		assert.False(t, b.Stopped)
	})

	t.Run("zero amount is allowed", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 0})
		require.NoError(t, err)
	})

	t.Run("negative amount", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: -1})
		requireValidationField(t, err, "amount")
	})

	t.Run("bad month", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-13", Amount: 1})
		requireValidationField(t, err, "effective_month")
	})

	t.Run("empty account name", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "  ", EffectiveMonth: "2026-01", Amount: 1})
		requireValidationField(t, err, "account_name")
	})

	t.Run("unknown account", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Nope", EffectiveMonth: "2026-01", Amount: 1})
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("non-expense account", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Assets:Bank", EffectiveMonth: "2026-01", Amount: 1})
		requireValidationField(t, err, "account_name")
	})

	t.Run("set after stop in the same month clears stopped", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 100})
		require.NoError(t, err)
		_, err = svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-03"})
		require.NoError(t, err)
		b, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-03", Amount: 200})
		require.NoError(t, err)
		assert.False(t, b.Stopped)
		active := activeBudgets(mustList(t, svc), "2026-03")
		require.Len(t, active, 1)
		assert.Equal(t, int64(200), active[0].Amount)
	})
}

func mustList(t *testing.T, svc *BudgetService) []model.Budget {
	t.Helper()
	list, err := svc.ListBudgets(context.Background())
	require.NoError(t, err)
	return list
}

func TestStopBudget(t *testing.T) {
	ctx := context.Background()

	t.Run("stops an active budget", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 100})
		require.NoError(t, err)
		b, err := svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-04"})
		require.NoError(t, err)
		assert.True(t, b.Stopped)
		assert.Equal(t, int64(0), b.Amount)
		assert.Empty(t, activeBudgets(mustList(t, svc), "2026-04"))
		assert.Len(t, activeBudgets(mustList(t, svc), "2026-03"), 1)
	})

	t.Run("no budget yet", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-04"})
		requireValidationField(t, err, "effective_month")
	})

	t.Run("already stopped", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 100})
		require.NoError(t, err)
		_, err = svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-02"})
		require.NoError(t, err)
		_, err = svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-05"})
		requireValidationField(t, err, "effective_month")
	})

	t.Run("unknown account", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Nope", EffectiveMonth: "2026-04"})
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestActiveBudgets(t *testing.T) {
	list := []model.Budget{
		{ID: 1, AccountID: 1, EffectiveMonth: "2026-01", Amount: 100},
		{ID: 2, AccountID: 1, EffectiveMonth: "2026-04", Amount: 200},
		{ID: 3, AccountID: 1, EffectiveMonth: "2026-07", Stopped: true},
		{ID: 4, AccountID: 1, EffectiveMonth: "2026-10", Amount: 300},
	}
	amountAt := func(month string) int64 {
		got := activeBudgets(list, month)
		if len(got) == 0 {
			return -1
		}
		return got[0].Amount
	}
	assert.Equal(t, int64(-1), amountAt("2025-12"), "before the first version")
	assert.Equal(t, int64(100), amountAt("2026-03"))
	assert.Equal(t, int64(200), amountAt("2026-04"))
	assert.Equal(t, int64(-1), amountAt("2026-08"), "after stop")
	assert.Equal(t, int64(300), amountAt("2026-11"), "restarted after stop")
}

func TestDeleteBudget_Service(t *testing.T) {
	ctx := context.Background()
	accRepo, txRepo, bRepo := budgetFixture()
	svc := newTestBudgetService(accRepo, txRepo, bRepo)
	b, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 1})
	require.NoError(t, err)

	require.NoError(t, svc.DeleteBudget(ctx, b.ID))
	assert.ErrorIs(t, svc.DeleteBudget(ctx, b.ID), ErrNotFound)
}

func TestListBudgets_Service(t *testing.T) {
	accRepo, txRepo, bRepo := budgetFixture()
	svc := newTestBudgetService(accRepo, txRepo, bRepo)
	list, err := svc.ListBudgets(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, list)
}
