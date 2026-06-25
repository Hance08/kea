// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTransactionWithSplits_And_GetByID(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	extID := "ext-123"
	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp:   1000,
		Description: "groceries",
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
		ExternalID:  &extID,
	}, []model.Split{
		{AccountID: assetID, Amount: -2000, Currency: "USD", Memo: "debit"},
		{AccountID: expenseID, Amount: 2000, Currency: "USD", Memo: "credit"},
	})
	require.NoError(t, err)
	assert.Positive(t, txID)

	tx, err := s.GetTransactionByID(ctx, txID)
	require.NoError(t, err)
	assert.Equal(t, txID, tx.ID)
	assert.Equal(t, int64(1000), tx.Timestamp)
	assert.Equal(t, "groceries", tx.Description)
	assert.Equal(t, model.StatusCleared, tx.Status)
	assert.Equal(t, model.TxTypeExpense, tx.Type)
	require.NotNil(t, tx.ExternalID)
	assert.Equal(t, "ext-123", *tx.ExternalID)
}

func TestGetTransactionByID_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.GetTransactionByID(ctx, 99999)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestGetTransactionsByAccount(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	otherID, err := s.CreateAccount(ctx, "Expenses:Other", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		_, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp: int64(1000 + i), Description: "food", Status: model.StatusPending, Type: model.TxTypeExpense,
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
	}
	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "other", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -200, Currency: "USD"},
		{AccountID: otherID, Amount: 200, Currency: "USD"},
	})
	require.NoError(t, err)

	txs, err := s.GetTransactionsByAccount(ctx, assetID, 100)
	require.NoError(t, err)
	assert.Len(t, txs, 3)

	txs, err = s.GetTransactionsByAccount(ctx, expenseID, 100)
	require.NoError(t, err)
	assert.Len(t, txs, 2)
}

func TestGetTransactionsByDateRange(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	for _, ts := range []int64{1000, 2000, 3000} {
		_, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp: ts, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
	}

	txs, err := s.GetTransactionsByDateRange(ctx, 1500, 2500)
	require.NoError(t, err)
	assert.Len(t, txs, 1)
	assert.Equal(t, int64(2000), txs[0].Timestamp)

	txs, err = s.GetTransactionsByDateRange(ctx, 1000, 3000)
	require.NoError(t, err)
	assert.Len(t, txs, 3)
}

func TestGetAllTransactions(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp: int64(1000 + i), Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
	}

	txs, err := s.GetAllTransactions(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, txs, 3)

	// limit=0 maps to default cap of 100; all 5 fit within that cap
	txs, err = s.GetAllTransactions(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, txs, 5)
	assert.LessOrEqual(t, len(txs), 100)
}

func TestUpdateTransactionStatus(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	err = s.UpdateTransactionStatus(ctx, txID, model.StatusCleared)
	require.NoError(t, err)

	tx, err := s.GetTransactionByID(ctx, txID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusCleared, tx.Status)
}

func TestUpdateTransactionStatus_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.UpdateTransactionStatus(ctx, 99999, model.StatusCleared)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestUpdateTransactionBasic(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "old", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	err = s.UpdateTransactionBasic(ctx, txID, "new desc", 2000, model.StatusCleared, model.TxTypeIncome, nil)
	require.NoError(t, err)

	tx, err := s.GetTransactionByID(ctx, txID)
	require.NoError(t, err)
	assert.Equal(t, "new desc", tx.Description)
	assert.Equal(t, int64(2000), tx.Timestamp)
	assert.Equal(t, model.StatusCleared, tx.Status)
	assert.Equal(t, model.TxTypeIncome, tx.Type)
}

func TestDeleteTransaction(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	err = s.DeleteTransaction(ctx, txID)
	require.NoError(t, err)

	_, err = s.GetTransactionByID(ctx, txID)
	require.Error(t, err)
}

func TestDeleteTransaction_CascadeDeletesSplits(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	splits, err := s.GetSplitsByTransaction(ctx, txID)
	require.NoError(t, err)
	assert.Len(t, splits, 2)

	err = s.DeleteTransaction(ctx, txID)
	require.NoError(t, err)

	// ON DELETE CASCADE should remove associated splits
	splits, err = s.GetSplitsByTransaction(ctx, txID)
	require.NoError(t, err)
	assert.Empty(t, splits)
}

func TestDeleteTransaction_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.DeleteTransaction(ctx, 99999)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestSplitCRUD(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	splits, err := s.GetSplitsByTransaction(ctx, txID)
	require.NoError(t, err)
	assert.Len(t, splits, 2)
	assert.Equal(t, int64(-100), splits[0].Amount)
	assert.Equal(t, int64(100), splits[1].Amount)

	otherID, err := s.CreateAccount(ctx, "Expenses:Other", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	newSplitID, err := s.CreateSplit(ctx, txID, &model.Split{
		AccountID: otherID, Amount: 50, Currency: "USD", Memo: "added",
	})
	require.NoError(t, err)
	assert.Positive(t, newSplitID)

	splits, err = s.GetSplitsByTransaction(ctx, txID)
	require.NoError(t, err)
	assert.Len(t, splits, 3)

	err = s.UpdateSplit(ctx, newSplitID, otherID, 75, "USD", "updated memo")
	require.NoError(t, err)

	splits, err = s.GetSplitsByTransaction(ctx, txID)
	require.NoError(t, err)
	var updated *model.Split
	for _, sp := range splits {
		if sp.ID == newSplitID {
			updated = sp
		}
	}
	require.NotNil(t, updated)
	assert.Equal(t, int64(75), updated.Amount)
	assert.Equal(t, "updated memo", updated.Memo)

	err = s.DeleteSplit(ctx, newSplitID)
	require.NoError(t, err)

	splits, err = s.GetSplitsByTransaction(ctx, txID)
	require.NoError(t, err)
	assert.Len(t, splits, 2)
}

func TestUpdateSplit_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.UpdateSplit(ctx, 99999, 1, 100, "USD", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestDeleteSplit_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.DeleteSplit(ctx, 99999)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestGetSplitsWithAccountsByDateRange(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "in range", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 3000, Description: "out of range", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -200, Currency: "USD"},
		{AccountID: expenseID, Amount: 200, Currency: "USD"},
	})
	require.NoError(t, err)

	result, err := s.GetSplitsWithAccountsByDateRange(ctx, 500, 1500)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Len(t, result[tx1ID], 2)
	assert.Equal(t, "Assets:Bank", result[tx1ID][0].AccountName)
	assert.Equal(t, model.AccountTypeAsset, result[tx1ID][0].AccountType)
}

func TestGetSplitsWithAccountsByTransaction(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD", Memo: "debit"},
		{AccountID: expenseID, Amount: 100, Currency: "USD", Memo: "credit"},
	})
	require.NoError(t, err)

	details, err := s.GetSplitsWithAccountsByTransaction(ctx, txID)
	require.NoError(t, err)
	assert.Len(t, details, 2)
	assert.Equal(t, "Assets:Bank", details[0].AccountName)
	assert.Equal(t, int64(-100), details[0].Amount)
	assert.Equal(t, "debit", details[0].Memo)
	assert.Equal(t, "Expenses:Food", details[1].AccountName)
}

func TestGetSplitsWithAccountsByTransactionIDs(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	tx1ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx1", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -100, Currency: "USD"},
		{AccountID: expenseID, Amount: 100, Currency: "USD"},
	})
	require.NoError(t, err)

	tx2ID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "tx2", Status: model.StatusPending, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: assetID, Amount: -200, Currency: "USD"},
		{AccountID: expenseID, Amount: 200, Currency: "USD"},
	})
	require.NoError(t, err)

	result, err := s.GetSplitsWithAccountsByTransactionIDs(ctx, []int64{tx1ID, tx2ID})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Len(t, result[tx1ID], 2)
	assert.Len(t, result[tx2ID], 2)

	empty, err := s.GetSplitsWithAccountsByTransactionIDs(ctx, []int64{})
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestGetSplitsWithAccountsByTransactionIDs_LargeBatch(t *testing.T) {
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
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
		txIDs[i] = txID
	}

	result, err := s.GetSplitsWithAccountsByTransactionIDs(ctx, txIDs)
	require.NoError(t, err)
	assert.Len(t, result, n)
	for _, txID := range txIDs {
		assert.Len(t, result[txID], 2, "each transaction should have 2 splits")
	}
}
