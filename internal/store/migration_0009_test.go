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

// TestMigration0009_BackfillInvestmentType inserts pre-existing rows of each
// shape and verifies the migration reclassifies only Investment-shaped ones.
func TestMigration0009_BackfillInvestmentType(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Insert accounts.
	invID, err := s.CreateAccount(ctx, "Assets:Investments:00878", model.AccountTypeAsset, "TWD", "", nil)
	require.NoError(t, err)
	bankID, err := s.CreateAccount(ctx, "Assets:Bank:DAWHO", model.AccountTypeAsset, "TWD", "", nil)
	require.NoError(t, err)
	revID, err := s.CreateAccount(ctx, "Revenue:Stuff", model.AccountTypeRevenue, "TWD", "", nil)
	require.NoError(t, err)
	feeID, err := s.CreateAccount(ctx, "Expenses:Fees:Stocks", model.AccountTypeExpense, "TWD", "", nil)
	require.NoError(t, err)
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "TWD", "", nil)
	require.NoError(t, err)

	// Insert pre-migration rows.
	sellID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1700000000, Description: "sell", Status: model.StatusCleared, Type: model.TxTypeIncome,
	}, []model.Split{
		{AccountID: invID, Amount: -5287, Currency: "TWD"},
		{AccountID: bankID, Amount: 8118, Currency: "TWD"},
		{AccountID: revID, Amount: -2831, Currency: "TWD"},
	})
	require.NoError(t, err)

	buyID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1700000001, Description: "buy", Status: model.StatusCleared, Type: model.TxTypeTransfer,
	}, []model.Split{
		{AccountID: invID, Amount: 20490, Currency: "TWD"},
		{AccountID: bankID, Amount: -20519, Currency: "TWD"},
		{AccountID: feeID, Amount: 29, Currency: "TWD"},
	})
	require.NoError(t, err)

	expID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1700000002, Description: "lunch", Status: model.StatusCleared, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: foodID, Amount: 500, Currency: "TWD"},
		{AccountID: bankID, Amount: -500, Currency: "TWD"},
	})
	require.NoError(t, err)

	incomeID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1700000003, Description: "tip", Status: model.StatusCleared, Type: model.TxTypeIncome,
	}, []model.Split{
		{AccountID: revID, Amount: -100, Currency: "TWD"},
		{AccountID: bankID, Amount: 100, Currency: "TWD"},
	})
	require.NoError(t, err)

	// Reset the type column on the Investment-shaped rows to simulate
	// their state BEFORE this migration ran. (The migration was already
	// applied once during setupTestDB on an empty database.)
	_, err = s.DB().ExecContext(ctx,
		`UPDATE transactions SET type = 'Income' WHERE id = ?`, sellID)
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx,
		`UPDATE transactions SET type = 'Transfer' WHERE id = ?`, buyID)
	require.NoError(t, err)

	// Re-run the 0009 up migration.
	sql, err := migrations.FS.ReadFile("0009_backfill_investment_type.up.sql")
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx, string(sql))
	require.NoError(t, err)

	// Helper to fetch type.
	fetchType := func(id int64) model.TransactionType {
		var ty string
		err := s.DB().QueryRowContext(ctx, `SELECT type FROM transactions WHERE id = ?`, id).Scan(&ty)
		require.NoError(t, err)
		return model.TransactionType(ty)
	}

	assert.Equal(t, model.TxTypeInvestment, fetchType(sellID), "sell should be Investment")
	assert.Equal(t, model.TxTypeInvestment, fetchType(buyID), "buy should be Investment")
	assert.Equal(t, model.TxTypeExpense, fetchType(expID), "expense untouched")
	assert.Equal(t, model.TxTypeIncome, fetchType(incomeID), "income untouched")
}
