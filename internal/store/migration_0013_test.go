// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"io/fs"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration0013_SavingsTargetsConstraints(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	db := s.DB()

	savingsID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	insert := func(accountID int64, month string, amount int64, stopped int) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO savings_targets (account_id, effective_month, amount, stopped) VALUES (?, ?, ?, ?)`,
			accountID, month, amount, stopped)
		return err
	}

	require.NoError(t, insert(savingsID, "2026-01", 1500000, 0))
	assert.Error(t, insert(savingsID, "2026-01", 1, 0), "duplicate (account, month) must be rejected")
	assert.Error(t, insert(savingsID, "2026-1", 1, 0), "month shape must be YYYY-MM")
	assert.Error(t, insert(savingsID, "2026-02", -1, 0), "negative amount must be rejected")
	assert.Error(t, insert(savingsID, "2026-03", 5, 1), "stopped row must have amount 0")
	assert.Error(t, insert(savingsID, "2026-04", 0, 2), "stopped must be 0 or 1")
	assert.Error(t, insert(999, "2026-05", 0, 0), "unknown account must be rejected")
	require.NoError(t, insert(savingsID, "2026-06", 0, 1))
	require.NoError(t, insert(savingsID, "2026-07", 0, 0), "zero target is valid")
}

func TestMigration0013_CascadeOnAccountDelete(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	savingsID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO savings_targets (account_id, effective_month, amount) VALUES (?, '2026-01', 100)`, savingsID)
	require.NoError(t, err)

	require.NoError(t, s.DeleteAccount(ctx, savingsID))

	var n int
	require.NoError(t, s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM savings_targets`).Scan(&n))
	assert.Equal(t, 0, n)
}

func TestMigration0013_Down(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	down, err := fs.ReadFile(migrations.FS, "0013_create_savings_targets.down.sql")
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx, string(down))
	require.NoError(t, err)

	var n int
	require.NoError(t, s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('savings_targets', 'idx_savings_targets_account_month')`).Scan(&n))
	assert.Equal(t, 0, n)
}
