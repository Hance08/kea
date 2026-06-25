// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUnreconciledTransactionsByAccount(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "groceries", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	tx2ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "dinner", Status: model.StatusCleared, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -300, Currency: "USD"},
		{AccountID: expenseID, Amount: 300, Currency: "USD"},
	})
	require.NoError(t, err)

	entries, err := s.GetUnreconciledTransactionsByAccount(ctx, assetID)
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	assert.Equal(t, tx1ID, entries[0].ID)
	assert.Equal(t, int64(-500), entries[0].Amount)
	assert.Equal(t, "Expenses:Food", entries[0].OffsetAccount)
	assert.Equal(t, tx2ID, entries[1].ID)
	assert.Equal(t, int64(-300), entries[1].Amount)
}

func TestGetUnreconciledTransactionsByAccount_ExcludesReconciled(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "tx2", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -300, Currency: "USD"},
		{AccountID: expenseID, Amount: 300, Currency: "USD"},
	})
	require.NoError(t, err)

	_, err = s.MarkSplitsReconciledByAccount(ctx, assetID, []int64{tx1ID})
	require.NoError(t, err)

	entries, err := s.GetUnreconciledTransactionsByAccount(ctx, assetID)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "tx2", entries[0].Description)
}

func TestGetUnreconciledTransactionsByAccount_MultiAccountIsolation(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "shared tx", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	_, err = s.MarkSplitsReconciledByAccount(ctx, assetID, []int64{txID})
	require.NoError(t, err)

	assetEntries, err := s.GetUnreconciledTransactionsByAccount(ctx, assetID)
	require.NoError(t, err)
	assert.Empty(t, assetEntries)

	expenseEntries, err := s.GetUnreconciledTransactionsByAccount(ctx, expenseID)
	require.NoError(t, err)
	assert.Len(t, expenseEntries, 1)
}

func TestGetUnreconciledTransactionsByAccount_SplitLabel(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expense1ID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	expense2ID, err := s.CreateAccount(ctx, "Expenses:Transport", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "multi", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -800, Currency: "USD"},
		{AccountID: expense1ID, Amount: 500, Currency: "USD"},
		{AccountID: expense2ID, Amount: 300, Currency: "USD"},
	})
	require.NoError(t, err)

	entries, err := s.GetUnreconciledTransactionsByAccount(ctx, assetID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "(split)", entries[0].OffsetAccount)
}

func TestMarkSplitsReconciledByAccount(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	tx2ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "tx2", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -300, Currency: "USD"},
		{AccountID: expenseID, Amount: 300, Currency: "USD"},
	})
	require.NoError(t, err)

	rowsAffected, err := s.MarkSplitsReconciledByAccount(ctx, assetID, []int64{tx1ID, tx2ID})
	require.NoError(t, err)
	assert.Equal(t, int64(2), rowsAffected)

	tx1, err := s.GetTransactionByID(ctx, tx1ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusReconciled, tx1.Status)

	tx2, err := s.GetTransactionByID(ctx, tx2ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusReconciled, tx2.Status)
}

func TestMarkSplitsReconciledByAccount_EmptyIDs(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	rows, err := s.MarkSplitsReconciledByAccount(ctx, assetID, []int64{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

func TestBulkUpdateTransactionStatus(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	tx2ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "tx2", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -200, Currency: "USD"},
		{AccountID: expenseID, Amount: 200, Currency: "USD"},
	})
	require.NoError(t, err)

	err = s.BulkUpdateTransactionStatus(ctx, []int64{tx1ID, tx2ID}, model.StatusCleared)
	require.NoError(t, err)

	tx1, err := s.GetTransactionByID(ctx, tx1ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusCleared, tx1.Status)

	tx2, err := s.GetTransactionByID(ctx, tx2ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusCleared, tx2.Status)
}

func TestBulkUpdateTransactionStatus_EmptyIDs(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.BulkUpdateTransactionStatus(ctx, []int64{}, model.StatusCleared)
	require.NoError(t, err)
}

func TestBulkUpdateTransactionStatus_MismatchedRowCount(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	err = s.BulkUpdateTransactionStatus(ctx, []int64{txID, 99999}, model.StatusCleared)
	require.Error(t, err)
}

func TestMarkSplitsReconciledByAccount_LargeBatch(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	const n = 600
	txIDs := make([]int64, n)
	for i := 0; i < n; i++ {
		txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp:   int64(1000 + i),
			Description: fmt.Sprintf("tx-%d", i),
			Status:      model.StatusPending,
			Type:        model.TxTypeExpense,
			Regular:     boolPtr(true),
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
		txIDs[i] = txID
	}

	rowsAffected, err := s.MarkSplitsReconciledByAccount(ctx, assetID, txIDs)
	require.NoError(t, err)
	assert.Equal(t, int64(n), rowsAffected)

	entries, err := s.GetUnreconciledTransactionsByAccount(ctx, assetID)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestBulkUpdateTransactionStatus_LargeBatch(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	const n = 600
	txIDs := make([]int64, n)
	for i := 0; i < n; i++ {
		txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp:   int64(1000 + i),
			Description: fmt.Sprintf("tx-%d", i),
			Status:      model.StatusPending,
			Type:        model.TxTypeExpense,
			Regular:     boolPtr(true),
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
		txIDs[i] = txID
	}

	err = s.BulkUpdateTransactionStatus(ctx, txIDs, model.StatusCleared)
	require.NoError(t, err)

	for _, txID := range txIDs {
		tx, err := s.GetTransactionByID(ctx, txID)
		require.NoError(t, err)
		assert.Equal(t, model.StatusCleared, tx.Status)
	}
}

func TestGetSetLastReconciledBalance(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	bal, err := s.GetLastReconciledBalance(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal)

	err = s.SetLastReconciledBalance(ctx, assetID, 5000)
	require.NoError(t, err)

	bal, err = s.GetLastReconciledBalance(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), bal)

	err = s.SetLastReconciledBalance(ctx, assetID, 7500)
	require.NoError(t, err)

	bal, err = s.GetLastReconciledBalance(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, int64(7500), bal)
}
