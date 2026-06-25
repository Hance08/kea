// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration0011_BackfillRegular(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	bankID, err := s.CreateAccount(ctx, "Assets:Bank:Main", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	salaryID, err := s.CreateAccount(ctx, "Revenue:Salary", model.AccountTypeRevenue, "USD", "", nil)
	require.NoError(t, err)

	boolPtr := func(b bool) *bool { return &b }

	incomeID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1_700_000_000, Description: "Salary",
		Status: model.StatusCleared, Type: model.TxTypeIncome,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: bankID, Amount: 100_000, Currency: "USD"},
		{AccountID: salaryID, Amount: -100_000, Currency: "USD"},
	})
	require.NoError(t, err)

	expenseID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1_700_000_001, Description: "Lunch",
		Status: model.StatusCleared, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: foodID, Amount: 500, Currency: "USD"},
		{AccountID: bankID, Amount: -500, Currency: "USD"},
	})
	require.NoError(t, err)

	transferID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1_700_000_002, Description: "ATM",
		Status: model.StatusCleared, Type: model.TxTypeTransfer,
	}, []model.Split{
		{AccountID: bankID, Amount: -1_000, Currency: "USD"},
		{AccountID: bankID, Amount: 1_000, Currency: "USD"},
	})
	require.NoError(t, err)

	fetchRegular := func(id int64) *int64 {
		var v *int64
		require.NoError(t, s.DB().QueryRowContext(ctx,
			`SELECT regular FROM transactions WHERE id = ?`, id).Scan(&v))
		return v
	}
	deref := func(p *int64) int64 {
		require.NotNil(t, p)
		return *p
	}

	assert.Equal(t, int64(1), deref(fetchRegular(incomeID)), "Income should store regular=1")
	assert.Equal(t, int64(1), deref(fetchRegular(expenseID)), "Expense should store regular=1")
	assert.Nil(t, fetchRegular(transferID), "Transfer should store regular=NULL")
}

func TestMigration0011_CheckConstraint(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	db := s.DB()

	_, err := db.ExecContext(ctx,
		`INSERT INTO transactions (timestamp, description, status, type, regular) VALUES (?, ?, ?, ?, ?)`,
		1, "x", 0, "Income", nil)
	assert.Error(t, err, "Income with regular=NULL must violate CHECK")

	_, err = db.ExecContext(ctx,
		`INSERT INTO transactions (timestamp, description, status, type, regular) VALUES (?, ?, ?, ?, ?)`,
		1, "x", 0, "Transfer", 1)
	assert.Error(t, err, "Transfer with regular=1 must violate CHECK")

	_, err = db.ExecContext(ctx,
		`INSERT INTO transactions (timestamp, description, status, type, regular) VALUES (?, ?, ?, ?, ?)`,
		1, "ok", 0, "Income", 0)
	assert.NoError(t, err, "Income with regular=0 must succeed")
}
