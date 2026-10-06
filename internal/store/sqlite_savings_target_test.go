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

func TestUpsertSavingsTarget_InsertThenReplace(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	accID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	id1, err := s.UpsertSavingsTarget(ctx, accID, "2026-01", 1500000, false)
	require.NoError(t, err)
	id2, err := s.UpsertSavingsTarget(ctx, accID, "2026-01", 2000000, false)
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "same (account, month) must update the same row")

	list, err := s.ListSavingsTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int64(2000000), list[0].Amount)
}

func TestUpsertSavingsTarget_SetAfterStopClearsStopped(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	accID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.UpsertSavingsTarget(ctx, accID, "2026-03", 0, true)
	require.NoError(t, err)
	_, err = s.UpsertSavingsTarget(ctx, accID, "2026-03", 5000, false)
	require.NoError(t, err)

	list, err := s.ListSavingsTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.False(t, list[0].Stopped)
	assert.Equal(t, int64(5000), list[0].Amount)
}

func TestListSavingsTargets_JoinsNameAndOrders(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	tripID, err := s.CreateAccount(ctx, "Assets:Trip", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	savID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.UpsertSavingsTarget(ctx, tripID, "2026-01", 1, false)
	require.NoError(t, err)
	_, err = s.UpsertSavingsTarget(ctx, savID, "2026-05", 2, false)
	require.NoError(t, err)
	_, err = s.UpsertSavingsTarget(ctx, savID, "2026-02", 3, false)
	require.NoError(t, err)

	list, err := s.ListSavingsTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "Assets:Savings", list[0].AccountName)
	assert.Equal(t, "2026-02", list[0].EffectiveMonth)
	assert.Equal(t, "2026-05", list[1].EffectiveMonth)
	assert.Equal(t, "Assets:Trip", list[2].AccountName)
}

func TestListSavingsTargets_EmptyIsNonNil(t *testing.T) {
	s := setupTestDB(t)
	list, err := s.ListSavingsTargets(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestListSavingsTargets_ReflectsAccountRename(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	accID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.UpsertSavingsTarget(ctx, accID, "2026-01", 1, false)
	require.NoError(t, err)

	require.NoError(t, s.RenameAccount(ctx, "Assets:Savings", "Assets:Banks:Cube_Saving"))

	list, err := s.ListSavingsTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Assets:Banks:Cube_Saving", list[0].AccountName)
}

func TestDeleteSavingsTarget(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	accID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	id, err := s.UpsertSavingsTarget(ctx, accID, "2026-01", 1, false)
	require.NoError(t, err)

	require.NoError(t, s.DeleteSavingsTarget(ctx, id))
	err = s.DeleteSavingsTarget(ctx, id)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
