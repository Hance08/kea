// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertBudget_InsertThenReplace(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	id1, err := s.UpsertBudget(ctx, foodID, "2026-01", 800000, false)
	require.NoError(t, err)
	id2, err := s.UpsertBudget(ctx, foodID, "2026-01", 900000, false)
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "same (account, month) must update the same row")

	list, err := s.ListBudgets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int64(900000), list[0].Amount)
}

func TestUpsertBudget_SetAfterStopClearsStopped(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.UpsertBudget(ctx, foodID, "2026-03", 0, true)
	require.NoError(t, err)
	_, err = s.UpsertBudget(ctx, foodID, "2026-03", 5000, false)
	require.NoError(t, err)

	list, err := s.ListBudgets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.False(t, list[0].Stopped)
	assert.Equal(t, int64(5000), list[0].Amount)
}

func TestListBudgets_JoinsNameAndOrders(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	rentID, err := s.CreateAccount(ctx, "Expenses:Rent", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.UpsertBudget(ctx, rentID, "2026-01", 1, false)
	require.NoError(t, err)
	_, err = s.UpsertBudget(ctx, foodID, "2026-05", 2, false)
	require.NoError(t, err)
	_, err = s.UpsertBudget(ctx, foodID, "2026-02", 3, false)
	require.NoError(t, err)

	list, err := s.ListBudgets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "Expenses:Food", list[0].AccountName)
	assert.Equal(t, "2026-02", list[0].EffectiveMonth)
	assert.Equal(t, "2026-05", list[1].EffectiveMonth)
	assert.Equal(t, "Expenses:Rent", list[2].AccountName)
}

func TestListBudgets_EmptyIsNonNil(t *testing.T) {
	s := setupTestDB(t)
	list, err := s.ListBudgets(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestListBudgets_ReflectsAccountRename(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.UpsertBudget(ctx, foodID, "2026-01", 1, false)
	require.NoError(t, err)

	require.NoError(t, s.RenameAccount(ctx, "Expenses:Food", "Expenses:Groceries"))

	list, err := s.ListBudgets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Expenses:Groceries", list[0].AccountName)
}

func TestDeleteBudget(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	id, err := s.UpsertBudget(ctx, foodID, "2026-01", 1, false)
	require.NoError(t, err)

	require.NoError(t, s.DeleteBudget(ctx, id))
	err = s.DeleteBudget(ctx, id)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
