// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// ValidateSplitsBalance
// ──────────────────────────────────────────────

func TestValidateSplitsBalance(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	makeSplits := func(amounts ...int64) []model.Split {
		result := make([]model.Split, len(amounts))
		for i, a := range amounts {
			result[i] = model.Split{Amount: a, Currency: "USD"}
		}
		return result
	}

	t.Run("two splits balance", func(t *testing.T) {
		err := svc.ValidateSplitsBalance(makeSplits(1000, -1000))
		require.NoError(t, err)
	})

	t.Run("three splits balance", func(t *testing.T) {
		err := svc.ValidateSplitsBalance(makeSplits(1000, -600, -400))
		require.NoError(t, err)
	})

	t.Run("four splits balance", func(t *testing.T) {
		err := svc.ValidateSplitsBalance(makeSplits(500, 500, -700, -300))
		require.NoError(t, err)
	})

	t.Run("empty slice sums to zero", func(t *testing.T) {
		err := svc.ValidateSplitsBalance(makeSplits())
		require.NoError(t, err)
	})

	t.Run("positive imbalance", func(t *testing.T) {
		err := svc.ValidateSplitsBalance(makeSplits(1000, -999))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "splits do not balance")
	})

	t.Run("negative imbalance", func(t *testing.T) {
		err := svc.ValidateSplitsBalance(makeSplits(999, -1000))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "splits do not balance")
	})

	t.Run("single non-zero split", func(t *testing.T) {
		err := svc.ValidateSplitsBalance(makeSplits(100))
		assert.Error(t, err)
	})

	t.Run("error message contains difference amount", func(t *testing.T) {
		err := svc.ValidateSplitsBalance(makeSplits(1000, -900))
		require.Error(t, err)
		// difference is 100 cents
		assert.Contains(t, err.Error(), "100")
	})

	t.Run("large balanced amounts", func(t *testing.T) {
		big := int64(9_223_372_036_854)
		err := svc.ValidateSplitsBalance(makeSplits(big, -big))
		require.NoError(t, err)
	})

	t.Run("mixed currencies rejected", func(t *testing.T) {
		splits := []model.Split{
			{Amount: 1000, Currency: "TWD"},
			{Amount: -1000, Currency: "USD"},
		}
		err := svc.ValidateSplitsBalance(splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "currency")
	})

	t.Run("all same currency passes", func(t *testing.T) {
		splits := []model.Split{
			{Amount: 1000, Currency: "TWD"},
			{Amount: -1000, Currency: "TWD"},
		}
		err := svc.ValidateSplitsBalance(splits)
		require.NoError(t, err)
	})
}

// ──────────────────────────────────────────────
// ValidateSplitDetailsBalance
// ──────────────────────────────────────────────

func TestValidateSplitDetailsBalance(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	makeSplitDetails := func(amounts ...int64) []model.SplitDetail {
		result := make([]model.SplitDetail, len(amounts))
		for i, a := range amounts {
			result[i] = model.SplitDetail{Amount: a, Currency: "USD"}
		}
		return result
	}

	t.Run("balanced splits pass", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails(1000, -1000))
		require.NoError(t, err)
	})

	t.Run("three splits balance", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails(1000, -600, -400))
		require.NoError(t, err)
	})

	t.Run("imbalanced splits rejected", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails(1000, -999))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "splits do not balance")
	})

	t.Run("single split rejected", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails(100))
		assert.Error(t, err)
	})

	t.Run("mixed currencies rejected", func(t *testing.T) {
		splits := []model.SplitDetail{
			{Amount: 1000, Currency: "TWD"},
			{Amount: -1000, Currency: "USD"},
		}
		err := svc.ValidateSplitDetailsBalance(splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "currency")
	})

	t.Run("same currency passes", func(t *testing.T) {
		splits := []model.SplitDetail{
			{Amount: 1000, Currency: "TWD"},
			{Amount: -1000, Currency: "TWD"},
		}
		err := svc.ValidateSplitDetailsBalance(splits)
		require.NoError(t, err)
	})

	t.Run("negative imbalance", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails(999, -1000))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "splits do not balance")
	})

	t.Run("error message contains difference amount", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails(1000, -900))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "100")
	})

	t.Run("empty slice sums to zero", func(t *testing.T) {
		err := svc.ValidateSplitDetailsBalance(makeSplitDetails())
		require.NoError(t, err)
	})

	t.Run("empty currency mixed with explicit currency rejected", func(t *testing.T) {
		splits := []model.SplitDetail{
			{Amount: -1000, Currency: ""},
			{Amount: 1000, Currency: "TWD"},
		}
		err := svc.ValidateSplitDetailsBalance(splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "currency")
	})
}

// ──────────────────────────────────────────────
// ValidateTransactionEdit
// ──────────────────────────────────────────────

func TestValidateTransactionEdit(t *testing.T) {
	t.Run("two valid balanced splits", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense})

		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		splits := []model.SplitDetail{
			{AccountID: 1, Amount: -500, Currency: "USD"},
			{AccountID: 2, Amount: 500, Currency: "USD"},
		}
		err := svc.ValidateTransactionEdit(context.Background(), splits)
		require.NoError(t, err)
	})

	t.Run("fewer than 2 splits rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountID: 1, Amount: 1000},
		}
		err := svc.ValidateTransactionEdit(context.Background(), splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2")
	})

	t.Run("empty split list rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.ValidateTransactionEdit(context.Background(), []model.SplitDetail{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2")
	})

	t.Run("unbalanced splits rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food"})

		svc := newTestTransactionService(accRepo, newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountID: 1, Amount: -500},
			{AccountID: 2, Amount: 600}, // off by 100
		}
		err := svc.ValidateTransactionEdit(context.Background(), splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "balance")
	})

	t.Run("non-existent account ID rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank"})
		// ID 99 does not exist

		svc := newTestTransactionService(accRepo, newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountID: 1, Amount: -500},
			{AccountID: 99, Amount: 500},
		}
		err := svc.ValidateTransactionEdit(context.Background(), splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "99")
	})

	t.Run("account lookup failure includes split number in error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank"})
		accRepo.getByIDErr[2] = errors.New("db error")

		svc := newTestTransactionService(accRepo, newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountID: 1, Amount: -500},
			{AccountID: 2, Amount: 500},
		}
		err := svc.ValidateTransactionEdit(context.Background(), splits)
		require.Error(t, err)
		// error message should reference split #2
		assert.Contains(t, err.Error(), "#2")
	})

	t.Run("multiple balanced splits pass validation", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		for id := int64(1); id <= 4; id++ {
			accRepo.addAccount(&model.Account{ID: id, Name: fmt.Sprintf("Account:%d", id)})
		}
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountID: 1, Amount: -1000},
			{AccountID: 2, Amount: 400},
			{AccountID: 3, Amount: 300},
			{AccountID: 4, Amount: 300},
		}
		err := svc.ValidateTransactionEdit(context.Background(), splits)
		require.NoError(t, err)
	})

	t.Run("mixed currency splits rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense})

		svc := newTestTransactionService(accRepo, newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountID: 1, Amount: -1000, Currency: "USD"},
			{AccountID: 2, Amount: 1000, Currency: "TWD"},
		}
		err := svc.ValidateTransactionEdit(context.Background(), splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "currency")
	})

	t.Run("empty currency mixed with explicit currency rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense})

		svc := newTestTransactionService(accRepo, newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountID: 1, Amount: -1000, Currency: ""},
			{AccountID: 2, Amount: 1000, Currency: "TWD"},
		}
		err := svc.ValidateTransactionEdit(context.Background(), splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "currency")
	})

	t.Run("same currency splits pass currency check", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense})

		svc := newTestTransactionService(accRepo, newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountID: 1, Amount: -1000, Currency: "TWD"},
			{AccountID: 2, Amount: 1000, Currency: "TWD"},
		}
		err := svc.ValidateTransactionEdit(context.Background(), splits)
		require.NoError(t, err)
	})
}
