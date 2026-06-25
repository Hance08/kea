// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigration0010_BackfillInvestmentTypeFromExpense covers the gap left by
// 0009: rows that the legacy 0006 SQL backfill stamped as 'Expense' (e.g., a
// buy with a non-zero fee split) need reclassification too.
func TestMigration0010_BackfillInvestmentTypeFromExpense(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Insert accounts.
	invID, err := s.CreateAccount(ctx, "Assets:Investments:0050", model.AccountTypeAsset, "TWD", "", nil)
	require.NoError(t, err)
	bankID, err := s.CreateAccount(ctx, "Assets:Bank:DAWHO", model.AccountTypeAsset, "TWD", "", nil)
	require.NoError(t, err)
	feeID, err := s.CreateAccount(ctx, "Expenses:Fees:Stocks", model.AccountTypeExpense, "TWD", "", nil)
	require.NoError(t, err)
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "TWD", "", nil)
	require.NoError(t, err)

	boolPtr := func(b bool) *bool { return &b }

	// Buy-with-fee row that the legacy 0006 backfill would have stamped as
	// 'Expense' (Expense + ≥1 A/L → Expense in the SQL backfill).
	buyID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1700000000, Description: "buy", Status: model.StatusCleared, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: invID, Amount: 20490, Currency: "TWD"},
		{AccountID: bankID, Amount: -20519, Currency: "TWD"},
		{AccountID: feeID, Amount: 29, Currency: "TWD"},
	})
	require.NoError(t, err)

	// A real Expense with no Investments split — must NOT be reclassified.
	lunchID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1700000001, Description: "lunch", Status: model.StatusCleared, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: foodID, Amount: 500, Currency: "TWD"},
		{AccountID: bankID, Amount: -500, Currency: "TWD"},
	})
	require.NoError(t, err)

	// Reset buy row to 'Expense' to simulate the state after 0009 ran but
	// missed it. (setupTestDB already applied both migrations on an empty
	// DB; we override the type to recreate the user-reported bug.)
	//
	// buyID was inserted with regular=true only so CreateTransactionWithSplits'
	// CHECK constraint is satisfied at insert time. The 0010 migration will
	// reclassify this row to type='Investment', which requires regular IS
	// NULL — the historically accurate shape, since the "regular" column did
	// not exist when 0010 originally ran. PRAGMA settings are
	// connection-scoped, so a single *sql.Conn guarantees the pragma toggle
	// applies to the UPDATE below.
	conn, err := s.DB().Conn(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `PRAGMA ignore_check_constraints = 1`)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx,
		`UPDATE transactions SET type = 'Expense', regular = NULL WHERE id = ?`, buyID)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `PRAGMA ignore_check_constraints = 0`)
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	// Re-run the 0010 up migration.
	sql, err := migrations.FS.ReadFile("0010_backfill_investment_type_expense.up.sql")
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx, string(sql))
	require.NoError(t, err)

	fetchType := func(id int64) model.TransactionType {
		var ty string
		err := s.DB().QueryRowContext(ctx, `SELECT type FROM transactions WHERE id = ?`, id).Scan(&ty)
		require.NoError(t, err)
		return model.TransactionType(ty)
	}

	assert.Equal(t, model.TxTypeInvestment, fetchType(buyID), "buy stored as Expense should be reclassified to Investment")
	assert.Equal(t, model.TxTypeExpense, fetchType(lunchID), "real Expense without Investments split must be untouched")
}
