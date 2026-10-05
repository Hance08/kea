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

func TestMigration0012_BudgetsConstraints(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	db := s.DB()

	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	insert := func(accountID int64, month string, amount int64, stopped int) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO budgets (account_id, effective_month, amount, stopped) VALUES (?, ?, ?, ?)`,
			accountID, month, amount, stopped)
		return err
	}

	require.NoError(t, insert(foodID, "2026-01", 800000, 0))
	assert.Error(t, insert(foodID, "2026-01", 1, 0), "duplicate (account, month) must be rejected")
	assert.Error(t, insert(foodID, "2026-1", 1, 0), "month shape must be YYYY-MM")
	assert.Error(t, insert(foodID, "2026-02", -1, 0), "negative amount must be rejected")
	assert.Error(t, insert(foodID, "2026-03", 5, 1), "stopped row must have amount 0")
	assert.Error(t, insert(foodID, "2026-04", 0, 2), "stopped must be 0 or 1")
	assert.Error(t, insert(999, "2026-05", 0, 0), "unknown account must be rejected")
	require.NoError(t, insert(foodID, "2026-06", 0, 1))
	require.NoError(t, insert(foodID, "2026-07", 0, 0), "zero budget is valid")
}

func TestMigration0012_CascadeOnAccountDelete(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO budgets (account_id, effective_month, amount) VALUES (?, '2026-01', 100)`, foodID)
	require.NoError(t, err)

	require.NoError(t, s.DeleteAccount(ctx, foodID))

	var n int
	require.NoError(t, s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM budgets`).Scan(&n))
	assert.Equal(t, 0, n)
}

func TestMigration0012_Down(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	down, err := fs.ReadFile(migrations.FS, "0012_create_budgets.down.sql")
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx, string(down))
	require.NoError(t, err)

	var n int
	require.NoError(t, s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('budgets', 'idx_budgets_account_month')`).Scan(&n))
	assert.Equal(t, 0, n)
}
