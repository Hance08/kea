// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// TestListRecentTransactions
// ──────────────────────────────────────────────

func TestListRecentTransactions(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		now := time.Now().Unix()
		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: now, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Timestamp: now, Description: "tx2", Status: model.StatusPending, Type: model.TxTypeExpense}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 3, Timestamp: now, Description: "tx3", Status: model.StatusPending, Type: model.TxTypeExpense}, nil)

		result, err := svc.ListRecentTransactions(context.Background(), model.ListOptions{Limit: 2, IncludeCount: true})
		require.NoError(t, err)
		assert.Len(t, result.Items, 2)
		assert.Equal(t, 3, result.TotalCount)
	})

	t.Run("zero limit uses repo default", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		result, err := svc.ListRecentTransactions(context.Background(), model.ListOptions{Limit: 0})
		require.NoError(t, err)
		assert.Equal(t, 20, result.Limit)
	})

	t.Run("empty result", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		result, err := svc.ListRecentTransactions(context.Background(), model.ListOptions{Limit: 10, IncludeCount: true})
		require.NoError(t, err)
		assert.NotNil(t, result.Items)
		assert.Empty(t, result.Items)
		assert.Equal(t, 0, result.TotalCount)
	})
}

// ──────────────────────────────────────────────
// TestListTransactionHistory
// ──────────────────────────────────────────────

func TestListTransactionHistory(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		now := time.Now().Unix()
		// tx1 touches Assets:Bank (ID=1) and Expenses:Food (ID=2)
		txRepo.addTransaction(
			&model.Transaction{ID: 1, Timestamp: now, Description: "lunch", Status: model.StatusPending, Type: model.TxTypeExpense},
			[]*model.Split{
				{ID: 1, TransactionID: 1, AccountID: 1, Amount: -500, Currency: "USD"},
				{ID: 2, TransactionID: 1, AccountID: 2, Amount: 500, Currency: "USD"},
			},
		)
		// tx2 only touches Revenue:Salary (ID=3) and Assets:Bank (ID=1)
		txRepo.addTransaction(
			&model.Transaction{ID: 2, Timestamp: now, Description: "salary", Status: model.StatusPending, Type: model.TxTypeIncome},
			[]*model.Split{
				{ID: 3, TransactionID: 2, AccountID: 3, Amount: -2000, Currency: "USD"},
				{ID: 4, TransactionID: 2, AccountID: 1, Amount: 2000, Currency: "USD"},
			},
		)
		// tx3 does NOT touch Assets:Bank
		txRepo.addTransaction(
			&model.Transaction{ID: 3, Timestamp: now, Description: "other", Status: model.StatusPending, Type: model.TxTypeExpense},
			[]*model.Split{
				{ID: 5, TransactionID: 3, AccountID: 2, Amount: -100, Currency: "USD"},
				{ID: 6, TransactionID: 3, AccountID: 3, Amount: 100, Currency: "USD"},
			},
		)

		result, err := svc.ListTransactionHistory(context.Background(), "Assets:Bank", model.ListOptions{Limit: 10, IncludeCount: true})
		require.NoError(t, err)
		// tx1 and tx2 both touch Assets:Bank
		assert.Len(t, result.Items, 2)
		assert.Equal(t, 2, result.TotalCount)
	})

	t.Run("account not found", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		_, err := svc.ListTransactionHistory(context.Background(), "Nonexistent:Account", model.ListOptions{Limit: 10})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account not found")
	})

	t.Run("with offset", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		now := time.Now().Unix()
		for i := int64(1); i <= 4; i++ {
			txRepo.addTransaction(
				&model.Transaction{ID: i, Timestamp: now, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense},
				[]*model.Split{
					{ID: i*10 + 1, TransactionID: i, AccountID: 1, Amount: -100, Currency: "USD"},
					{ID: i*10 + 2, TransactionID: i, AccountID: 2, Amount: 100, Currency: "USD"},
				},
			)
		}

		// First page: offset=0, limit=2 → 2 items
		page1, err := svc.ListTransactionHistory(context.Background(), "Assets:Bank", model.ListOptions{Limit: 2, Offset: 0, IncludeCount: true})
		require.NoError(t, err)
		assert.Len(t, page1.Items, 2)
		assert.Equal(t, 4, page1.TotalCount)

		// Second page: offset=2, limit=2 → 2 items
		page2, err := svc.ListTransactionHistory(context.Background(), "Assets:Bank", model.ListOptions{Limit: 2, Offset: 2, IncludeCount: true})
		require.NoError(t, err)
		assert.Len(t, page2.Items, 2)
		assert.Equal(t, 4, page2.TotalCount)
	})
}
