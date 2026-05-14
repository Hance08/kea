// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestAccounts sets up two leaf accounts used by pagination tests.
// Returns (assetID, expenseID).
func createTestAccounts(t *testing.T, s interface {
	CreateAccount(ctx context.Context, name string, accountType model.AccountType, currency string, description string, parentID *int64) (int64, error)
}) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	return assetID, expenseID
}

// addTx is a helper to insert a balanced transaction (asset debit, expense credit).
func addTx(t *testing.T, s interface {
	CreateTransactionWithSplits(ctx context.Context, tx model.Transaction, splits []model.Split) (int64, error)
}, assetID, expenseID int64, desc string) int64 {
	t.Helper()
	id, err := s.CreateTransactionWithSplits(context.Background(), model.Transaction{
		Timestamp:   time.Now().Unix(),
		Description: desc,
		Status:      model.StatusPending,
		Type:        model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -1000, Currency: "USD"},
		{AccountID: expenseID, Amount: 1000, Currency: "USD"},
	})
	require.NoError(t, err)
	return id
}

// ──────────────────────────────────────────────
// TestListTransactions
// ──────────────────────────────────────────────

func TestListTransactions(t *testing.T) {
	t.Run("pagination", func(t *testing.T) {
		s := setupTestDB(t)
		ctx := context.Background()
		assetID, expenseID := createTestAccounts(t, s)

		for i := 0; i < 5; i++ {
			addTx(t, s, assetID, expenseID, "tx")
		}

		// Page 1: offset=0, limit=2 → 2 items
		r1, err := s.ListTransactions(ctx, model.ListOptions{Limit: 2, Offset: 0})
		require.NoError(t, err)
		assert.Len(t, r1.Items, 2)

		// Page 2: offset=2, limit=2 → 2 items
		r2, err := s.ListTransactions(ctx, model.ListOptions{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Len(t, r2.Items, 2)

		// Page 3: offset=4, limit=2 → 1 item
		r3, err := s.ListTransactions(ctx, model.ListOptions{Limit: 2, Offset: 4})
		require.NoError(t, err)
		assert.Len(t, r3.Items, 1)
	})

	t.Run("include count", func(t *testing.T) {
		s := setupTestDB(t)
		assetID, expenseID := createTestAccounts(t, s)
		ctx := context.Background()

		for i := 0; i < 5; i++ {
			addTx(t, s, assetID, expenseID, "tx")
		}

		withCount, err := s.ListTransactions(ctx, model.ListOptions{Limit: 10, IncludeCount: true})
		require.NoError(t, err)
		assert.Equal(t, 5, withCount.TotalCount)

		withoutCount, err := s.ListTransactions(ctx, model.ListOptions{Limit: 10, IncludeCount: false})
		require.NoError(t, err)
		assert.Equal(t, 0, withoutCount.TotalCount)
	})

	t.Run("default limit", func(t *testing.T) {
		s := setupTestDB(t)
		ctx := context.Background()

		result, err := s.ListTransactions(ctx, model.ListOptions{Limit: 0})
		require.NoError(t, err)
		assert.Equal(t, 20, result.Limit)
	})

	t.Run("offset beyond total", func(t *testing.T) {
		s := setupTestDB(t)
		assetID, expenseID := createTestAccounts(t, s)
		ctx := context.Background()

		for i := 0; i < 5; i++ {
			addTx(t, s, assetID, expenseID, "tx")
		}

		result, err := s.ListTransactions(ctx, model.ListOptions{Limit: 10, Offset: 100, IncludeCount: true})
		require.NoError(t, err)
		assert.Empty(t, result.Items)
		assert.Equal(t, 5, result.TotalCount)
	})
}

// ──────────────────────────────────────────────
// TestListTransactionsByAccount
// ──────────────────────────────────────────────

func TestListTransactionsByAccount(t *testing.T) {
	t.Run("filters by account", func(t *testing.T) {
		s := setupTestDB(t)
		ctx := context.Background()

		assetID, expenseID := createTestAccounts(t, s)
		// Third account not touched by any tx
		otherID, err := s.CreateAccount(ctx, "Expenses:Other", model.AccountTypeExpense, "USD", "", nil)
		require.NoError(t, err)
		_ = otherID

		// 3 txs touch assetID + expenseID
		for i := 0; i < 3; i++ {
			addTx(t, s, assetID, expenseID, "food")
		}
		// 2 txs touch assetID + otherID
		for i := 0; i < 2; i++ {
			_, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
				Timestamp:   time.Now().Unix(),
				Description: "other",
				Status:      model.StatusPending,
				Type:        model.TxTypeExpense,
			}, []model.Split{
				{AccountID: assetID, Amount: -500, Currency: "USD"},
				{AccountID: otherID, Amount: 500, Currency: "USD"},
			})
			require.NoError(t, err)
		}

		// Filter to expenseID only — should see 3 txs
		result, err := s.ListTransactionsByAccount(ctx, expenseID, model.ListOptions{Limit: 10, IncludeCount: true})
		require.NoError(t, err)
		assert.Len(t, result.Items, 3)
		assert.Equal(t, 3, result.TotalCount)
	})

	t.Run("pagination with account filter", func(t *testing.T) {
		s := setupTestDB(t)
		ctx := context.Background()
		assetID, expenseID := createTestAccounts(t, s)

		for i := 0; i < 5; i++ {
			addTx(t, s, assetID, expenseID, "tx")
		}

		page1, err := s.ListTransactionsByAccount(ctx, expenseID, model.ListOptions{Limit: 2, Offset: 0})
		require.NoError(t, err)
		assert.Len(t, page1.Items, 2)

		page2, err := s.ListTransactionsByAccount(ctx, expenseID, model.ListOptions{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Len(t, page2.Items, 2)

		page3, err := s.ListTransactionsByAccount(ctx, expenseID, model.ListOptions{Limit: 2, Offset: 4})
		require.NoError(t, err)
		assert.Len(t, page3.Items, 1)
	})

	t.Run("include count with filter", func(t *testing.T) {
		s := setupTestDB(t)
		ctx := context.Background()
		assetID, expenseID := createTestAccounts(t, s)
		otherID, err := s.CreateAccount(ctx, "Expenses:Other", model.AccountTypeExpense, "USD", "", nil)
		require.NoError(t, err)

		// 2 txs touch expenseID
		for i := 0; i < 2; i++ {
			addTx(t, s, assetID, expenseID, "food")
		}
		// 3 txs touch otherID
		for i := 0; i < 3; i++ {
			_, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
				Timestamp:   time.Now().Unix(),
				Description: "other",
				Status:      model.StatusPending,
				Type:        model.TxTypeExpense,
			}, []model.Split{
				{AccountID: assetID, Amount: -500, Currency: "USD"},
				{AccountID: otherID, Amount: 500, Currency: "USD"},
			})
			require.NoError(t, err)
		}

		// TotalCount should reflect expenseID's 2 txs, not all 5
		result, err := s.ListTransactionsByAccount(ctx, expenseID, model.ListOptions{Limit: 10, IncludeCount: true})
		require.NoError(t, err)
		assert.Len(t, result.Items, 2)
		assert.Equal(t, 2, result.TotalCount)
	})
}
