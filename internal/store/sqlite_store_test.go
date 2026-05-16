// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStore_DB verifies that the DB() accessor returns a non-nil *sql.DB
// that can be used to interact with the underlying database.
func TestStore_DB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.NewStore(dbPath, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	db := s.DB()
	assert.NotNil(t, db, "DB() should return a non-nil *sql.DB")

	// Verify the returned *sql.DB responds to Ping
	err = db.Ping()
	assert.NoError(t, err, "should be able to Ping the returned *sql.DB")
}

func TestExecTx_CommitsOnSuccess(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
		return err
	})
	require.NoError(t, err)

	acc, err := s.GetAccountByName(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.Equal(t, "Assets:Bank", acc.Name)
}

func TestExecTx_RollsBackOnError(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.ExecTx(ctx, func(repo repository.Repository) error {
		_, err := repo.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
		if err != nil {
			return err
		}
		return fmt.Errorf("simulated failure")
	})
	require.Error(t, err)

	exists, err := s.AccountExists(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.False(t, exists)
}
