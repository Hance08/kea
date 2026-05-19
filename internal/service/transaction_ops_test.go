// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// Test helper: standard account fixture
// ──────────────────────────────────────────────

func setupStandardAccounts(accRepo *mockAccountRepo) {
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: ""})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: ""})
	accRepo.addAccount(&model.Account{ID: 3, Name: "Revenue:Salary", Type: model.AccountTypeRevenue, Currency: ""})
	accRepo.addAccount(&model.Account{ID: 4, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "TWD"})
}

// ──────────────────────────────────────────────
// CreateTransaction
// ──────────────────────────────────────────────

func TestCreateTransaction(t *testing.T) {
	t.Run("valid double-entry transaction created", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		input := model.TransactionDetail{
			Description: "Lunch",
			Type:        model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		id, err := svc.CreateTransaction(context.Background(), input)
		require.NoError(t, err)
		assert.Greater(t, id, int64(0))
	})

	t.Run("fewer than 2 splits rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Assets:Bank", Amount: 1000},
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2")
	})

	t.Run("empty splits rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateTransaction(context.Background(), model.TransactionDetail{Type: model.TxTypeExpense})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2")
	})

	t.Run("error: missing type", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type is required")
	})

	t.Run("unbalanced splits rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -400}, // off by 100
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "balance")
	})

	t.Run("unknown account rejected with split number in error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank"})
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Assets:Bank", Amount: -500},
				{AccountName: "NonExistent:Account", Amount: 500},
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "#2")
	})

	t.Run("zero timestamp is set to current time", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		before := time.Now().Unix()
		input := model.TransactionDetail{
			Timestamp: 0,
			Type:      model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		id, err := svc.CreateTransaction(context.Background(), input)
		require.NoError(t, err)

		tx := txRepo.transactions[id]
		assert.GreaterOrEqual(t, tx.Timestamp, before)
	})

	t.Run("account currency overrides system default", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		// Both accounts use TWD so splits are same-currency
		accRepo.addAccount(&model.Account{ID: 10, Name: "Assets:TWDBank", Type: model.AccountTypeAsset, Currency: "TWD"})
		accRepo.addAccount(&model.Account{ID: 11, Name: "Expenses:TWDFood", Type: model.AccountTypeExpense, Currency: "TWD"})
		svc := newTestTransactionService(accRepo, txRepo)

		input := model.TransactionDetail{
			Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:TWDFood", Amount: 500},
				{AccountName: "Assets:TWDBank", Amount: -500},
			},
		}
		id, err := svc.CreateTransaction(context.Background(), input)
		require.NoError(t, err)

		splits := txRepo.splits[id]
		require.Len(t, splits, 2)
		for _, s := range splits {
			assert.Equal(t, "TWD", s.Currency)
		}
	})

	t.Run("mixed currency splits rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		// Assets:Cash is TWD, Expenses:Food has no currency (falls back to USD)
		input := model.TransactionDetail{
			Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Cash", Amount: -500},
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "currency")
	})

	t.Run("account without currency uses system default", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		input := model.TransactionDetail{
			Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		id, err := svc.CreateTransaction(context.Background(), input)
		require.NoError(t, err)

		for _, s := range txRepo.splits[id] {
			assert.Equal(t, "USD", s.Currency)
		}
	})

	t.Run("db failure returns error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.createErr = errors.New("db write failed")
		svc := newTestTransactionService(accRepo, txRepo)

		input := model.TransactionDetail{
			Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		assert.Error(t, err)
	})

	t.Run("split referencing hidden account rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		accRepo.addAccount(&model.Account{ID: 10, Name: "Assets:Old", Type: model.AccountTypeAsset, IsHidden: true})
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Description: "hidden test",
			Type:        model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Assets:Old", Amount: 500},
				{AccountName: "Expenses:Food", Amount: -500},
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "split #1")
		assert.Contains(t, err.Error(), "hidden")
	})

	t.Run("split referencing parent account rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		accRepo.addAccount(&model.Account{ID: 11, Name: "Assets", Type: model.AccountTypeAsset})
		accRepo.childMap[11] = true
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Description: "parent test",
			Type:        model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountName: "Assets", Amount: 500},
				{AccountName: "Expenses:Food", Amount: -500},
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "split #1")
		assert.Contains(t, err.Error(), "parent account")
	})

	t.Run("status reconciled(2) rejected on create", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Type:   model.TxTypeExpense,
			Status: model.StatusReconciled,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")

		var ve *ValidationError
		assert.True(t, errors.As(err, &ve))
		assert.Equal(t, "status", ve.Field)
	})

	t.Run("invalid status(99) rejected on create", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Type:   model.TxTypeExpense,
			Status: model.TransactionStatus(99),
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		_, err := svc.CreateTransaction(context.Background(), input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")

		var ve *ValidationError
		assert.True(t, errors.As(err, &ve))
		assert.Equal(t, "status", ve.Field)
	})
}

// ──────────────────────────────────────────────
// CreateSimpleTransaction
// ──────────────────────────────────────────────

func TestCreateSimpleTransaction(t *testing.T) {
	t.Run("creates two balanced splits with correct direction", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		detail, err := svc.CreateSimpleTransaction(
			context.Background(), model.CreateSimpleTransactionInput{FromAccount: "Assets:Bank", ToAccount: "Expenses:Food", Amount: 1000, Description: "Dinner", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense},
		)
		require.NoError(t, err)
		assert.Greater(t, detail.ID, int64(0))

		// to account = +amount, from account = -amount
		require.Len(t, detail.Splits, 2)
		amounts := map[string]int64{}
		for _, s := range detail.Splits {
			amounts[s.AccountName] = s.Amount
		}
		assert.Equal(t, int64(1000), amounts["Expenses:Food"])
		assert.Equal(t, int64(-1000), amounts["Assets:Bank"])
	})

	t.Run("splits sum to zero", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		detail, err := svc.CreateSimpleTransaction(
			context.Background(), model.CreateSimpleTransactionInput{FromAccount: "Assets:Bank", ToAccount: "Expenses:Food", Amount: 750, Description: "Coffee", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense},
		)
		require.NoError(t, err)

		var total int64
		for _, s := range detail.Splits {
			total += s.Amount
		}
		assert.Equal(t, int64(0), total)
	})

	t.Run("same account rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateSimpleTransaction(
			context.Background(), model.CreateSimpleTransactionInput{FromAccount: "Assets:Bank", ToAccount: "Assets:Bank", Amount: 1000, Description: "Self", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "same")
	})

	t.Run("zero amount rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateSimpleTransaction(
			context.Background(), model.CreateSimpleTransactionInput{FromAccount: "Assets:Bank", ToAccount: "Expenses:Food", Amount: 0, Description: "Zero", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "positive")
	})

	t.Run("negative amount rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateSimpleTransaction(
			context.Background(), model.CreateSimpleTransactionInput{FromAccount: "Assets:Bank", ToAccount: "Expenses:Food", Amount: -100, Description: "Negative", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "positive")
	})

	t.Run("empty type infers from account types", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		detail, err := svc.CreateSimpleTransaction(
			context.Background(), model.CreateSimpleTransactionInput{FromAccount: "Assets:Bank", ToAccount: "Expenses:Food", Amount: 500, Description: "Inferred", Timestamp: 0, Status: model.StatusPending, Type: ""},
		)
		require.NoError(t, err)
		assert.Equal(t, model.TxTypeExpense, detail.Type)
	})
}

// ──────────────────────────────────────────────
// DeleteTransaction
// ──────────────────────────────────────────────

func TestDeleteTransaction(t *testing.T) {
	t.Run("pending transaction deleted successfully", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(accRepo, txRepo)

		err := svc.DeleteTransaction(context.Background(), 5)
		require.NoError(t, err)
		_, exists := txRepo.transactions[5]
		assert.False(t, exists, "transaction should not exist after deletion")
	})

	t.Run("cleared transaction deleted successfully", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 6, Status: model.StatusCleared}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.DeleteTransaction(context.Background(), 6)
		require.NoError(t, err)
	})

	t.Run("opening balance transaction (ID=1) rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.DeleteTransaction(context.Background(), model.SystemTransactionID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable), "expected ErrNotEditable, got: %v", err)
	})

	t.Run("non-existent transaction returns ErrNotFound", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.DeleteTransaction(context.Background(), 999)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound, got: %v", err)
	})

	t.Run("reconciled transaction rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 7, Status: model.StatusReconciled}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.DeleteTransaction(context.Background(), 7)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrReconciled), "expected ErrReconciled, got: %v", err)
	})

	t.Run("ErrReconciled detectable via errors.Is through error chain", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 8, Status: model.StatusReconciled}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.DeleteTransaction(context.Background(), 8)
		// wrapped with %w — errors.Is must unwrap correctly
		assert.True(t, errors.Is(err, ErrReconciled))
		assert.False(t, errors.Is(err, ErrNotEditable))
	})
}

// ──────────────────────────────────────────────
// UpdateTransactionStatus
// ──────────────────────────────────────────────

func TestUpdateTransactionStatus(t *testing.T) {
	t.Run("pending to cleared is valid", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionStatus(context.Background(), 5, model.StatusCleared)
		require.NoError(t, err)
		assert.Equal(t, model.StatusCleared, txRepo.transactions[5].Status)
	})

	t.Run("cleared to pending is valid", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusCleared}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionStatus(context.Background(), 5, model.StatusPending)
		require.NoError(t, err)
	})

	t.Run("setting status to reconciled(2) rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionStatus(context.Background(), 5, model.StatusReconciled)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("invalid status(99) rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionStatus(context.Background(), 5, model.TransactionStatus(99))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("modifying reconciled transaction rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusReconciled}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionStatus(context.Background(), 5, model.StatusCleared)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrReconciled))
	})

	t.Run("system transaction (ID=1) rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.UpdateTransactionStatus(context.Background(), model.SystemTransactionID, model.StatusCleared)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable), "expected ErrNotEditable, got: %v", err)
	})

	t.Run("non-existent transaction returns error", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.UpdateTransactionStatus(context.Background(), 999, model.StatusCleared)
		require.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// UpdateTransactionComplete
// ──────────────────────────────────────────────

func TestUpdateTransactionComplete(t *testing.T) {
	makeExistingSplits := func(ids ...int64) []*model.Split {
		result := make([]*model.Split, len(ids))
		for i, id := range ids {
			result[i] = &model.Split{ID: id, AccountID: int64(i + 1), Amount: 0}
		}
		return result
	}

	t.Run("valid complete update succeeds", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			makeExistingSplits(10, 11),
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -800, Currency: "USD"},
			{ID: 11, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 800, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "Updated desc", Timestamp: 0, Status: model.StatusCleared, Type: model.TxTypeExpense, Splits: splits})
		require.NoError(t, err)
	})

	t.Run("invalid status(99) rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.TransactionStatus(99), Type: model.TxTypeExpense, Splits: nil})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("setting status to reconciled(2) rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			makeExistingSplits(10, 11),
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -800, Currency: "USD"},
			{ID: 11, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 800, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{
			ID: 5, Description: "Trying reconciled", Timestamp: 0,
			Status: model.StatusReconciled, Type: model.TxTypeExpense, Splits: splits,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")

		var ve *ValidationError
		assert.True(t, errors.As(err, &ve))
		assert.Equal(t, "status", ve.Field)
	})

	t.Run("non-existent transaction returns error", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 999, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{{AccountID: 1, Amount: 0}, {AccountID: 2, Amount: 0}}})
		require.Error(t, err)
	})

	t.Run("reconciled transaction rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusReconciled}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusCleared, Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{{AccountID: 1, Amount: 500}, {AccountID: 2, Amount: -500}}})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrReconciled))
	})

	t.Run("fewer than 2 splits rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{{AccountID: 1, Amount: 0}}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2")
	})

	t.Run("unbalanced splits rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(accRepo, txRepo)

		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountID: 1, Amount: -500},
				{AccountID: 2, Amount: 400}, // off by 100
			}})
		require.Error(t, err)
	})

	t.Run("mixed currency splits rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			makeExistingSplits(10, 11),
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -1000, Currency: "USD"},
			{ID: 11, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 1000, Currency: "TWD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "Mixed", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense, Splits: splits})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "currency")
	})

	t.Run("non-existent account rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank"})
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(accRepo, txRepo)

		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				{AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -500},
				{AccountID: 99, AccountType: model.AccountTypeExpense, Amount: 500}, // does not exist
			}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "99")
	})

	t.Run("removed split triggers DeleteSplit call", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		// original has 3 splits; new payload only has 2
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			[]*model.Split{
				{ID: 10, AccountID: 1, Amount: -1000},
				{ID: 11, AccountID: 2, Amount: 600},
				{ID: 12, AccountID: 3, Amount: 400}, // this split will be removed
			},
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -1000, Currency: "USD"},
			{ID: 11, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 1000, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense, Splits: splits})
		require.NoError(t, err)
		assert.Contains(t, txRepo.deleteSplitCalls, int64(12), "split ID 12 should be deleted")
	})

	t.Run("split with ID=0 triggers CreateSplit call", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			[]*model.Split{{ID: 10, AccountID: 1, Amount: -1000}},
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -1000, Currency: "USD"},
			{ID: 0, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 1000, Currency: "USD"}, // new split
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense, Splits: splits})
		require.NoError(t, err)
		assert.Len(t, txRepo.createSplitCalls, 1, "CreateSplit should be called once")
	})

	t.Run("existing split (ID≠0) triggers UpdateSplit call", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			[]*model.Split{
				{ID: 10, AccountID: 1, Amount: -1000},
				{ID: 11, AccountID: 2, Amount: 1000},
			},
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -800, Currency: "USD"},
			{ID: 11, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 800, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense, Splits: splits})
		require.NoError(t, err)
		assert.Contains(t, txRepo.updateSplitCalls, int64(10))
		assert.Contains(t, txRepo.updateSplitCalls, int64(11))
	})

	t.Run("split referencing hidden account rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		accRepo.addAccount(&model.Account{ID: 10, Name: "Assets:Old", Type: model.AccountTypeAsset, IsHidden: true})
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, makeExistingSplits(20, 21))
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{AccountID: 10, AccountType: model.AccountTypeAsset, Amount: -500, Currency: "USD"},
			{AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense, Splits: splits})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "hidden")
	})

	t.Run("split referencing parent account rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		accRepo.addAccount(&model.Account{ID: 11, Name: "Assets", Type: model.AccountTypeAsset})
		accRepo.childMap[11] = true
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, makeExistingSplits(20, 21))
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{AccountID: 11, AccountType: model.AccountTypeAsset, Amount: -500, Currency: "USD"},
			{AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense, Splits: splits})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent account")
	})

	t.Run("unchanged split on now-hidden account allowed", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		accRepo.addAccount(&model.Account{ID: 10, Name: "Assets:Old", Type: model.AccountTypeAsset, IsHidden: true})
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, []*model.Split{
			{ID: 20, AccountID: 10, Amount: -500},
			{ID: 21, AccountID: 2, Amount: 500},
		})
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 20, AccountID: 10, AccountType: model.AccountTypeAsset, Amount: -500, Currency: "USD"},
			{ID: 21, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "updated desc", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense, Splits: splits})
		require.NoError(t, err)
	})

	t.Run("existing split moved to hidden account rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		accRepo.addAccount(&model.Account{ID: 10, Name: "Assets:Old", Type: model.AccountTypeAsset, IsHidden: true})
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, []*model.Split{
			{ID: 20, AccountID: 1, Amount: -500}, // originally account 1 (Assets:Bank)
			{ID: 21, AccountID: 2, Amount: 500},
		})
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 20, AccountID: 10, AccountType: model.AccountTypeAsset, Amount: -500, Currency: "USD"}, // moved to hidden account
			{ID: 21, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense, Splits: splits})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "hidden")
	})

	t.Run("error: splits do not match declared type", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			makeExistingSplits(10, 11),
		)
		svc := newTestTransactionService(accRepo, txRepo)

		// txType = Transfer but splits contain an Expense account — should fail
		splits := []model.SplitDetail{
			{AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
			{AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -500, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeTransfer, Splits: splits})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "splits do not match transaction type")
	})

	t.Run("split ID from another transaction rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			makeExistingSplits(10, 11),
		)
		txRepo.addTransaction(
			&model.Transaction{ID: 6, Status: model.StatusPending},
			[]*model.Split{{ID: 20, AccountID: 1, Amount: 100}, {ID: 21, AccountID: 2, Amount: -100}},
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 20, AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -500, Currency: "USD"},
			{ID: 11, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "cross-tx", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense, Splits: splits})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not belong to transaction")
		assert.Empty(t, txRepo.deleteSplitCalls, "no splits should be deleted")
		assert.Empty(t, txRepo.updateSplitCalls, "no splits should be updated")
		assert.Empty(t, txRepo.createSplitCalls, "no splits should be created")
	})

	t.Run("duplicate split IDs rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			makeExistingSplits(10, 11),
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, AccountType: model.AccountTypeAsset, Amount: -500, Currency: "USD"},
			{ID: 10, AccountID: 2, AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(context.Background(), model.UpdateTransactionInput{ID: 5, Description: "dup", Timestamp: 0, Status: model.StatusPending, Type: model.TxTypeExpense, Splits: splits})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate split ID")
		assert.Empty(t, txRepo.deleteSplitCalls)
		assert.Empty(t, txRepo.updateSplitCalls)
		assert.Empty(t, txRepo.createSplitCalls)
	})
}

// ──────────────────────────────────────────────
// IsEditable
// ──────────────────────────────────────────────

func TestIsEditable(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	t.Run("regular transaction is editable", func(t *testing.T) {
		detail := &model.TransactionDetail{ID: 5}
		editable, reason := svc.IsEditable(detail)
		assert.True(t, editable)
		assert.Equal(t, EditableOK, reason)
	})

	t.Run("opening balance transaction (ID=1) is not editable", func(t *testing.T) {
		detail := &model.TransactionDetail{ID: model.SystemTransactionID}
		editable, reason := svc.IsEditable(detail)
		assert.False(t, editable)
		assert.Equal(t, NotEditableSystemTx, reason)
	})

	t.Run("ID=2 is editable", func(t *testing.T) {
		detail := &model.TransactionDetail{ID: 2}
		editable, reason := svc.IsEditable(detail)
		assert.True(t, editable)
		assert.Equal(t, EditableOK, reason)
	})

	t.Run("reconciled transaction is not editable", func(t *testing.T) {
		detail := &model.TransactionDetail{ID: 5, Status: model.StatusReconciled}
		editable, reason := svc.IsEditable(detail)
		assert.False(t, editable)
		assert.Equal(t, NotEditableReconciled, reason)
	})
}

// ──────────────────────────────────────────────
// CreateTransactionFromSplits
// ──────────────────────────────────────────────

func TestCreateTransactionFromSplits(t *testing.T) {
	t.Run("delegates to CreateTransaction and returns detail with ID", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{AccountName: "Assets:Bank", Amount: -200000},
			{AccountName: "Expenses:Food", Amount: 200000},
		}
		result, err := svc.CreateTransactionFromSplits(context.Background(), model.CreateTransactionFromSplitsInput{Splits: splits, Description: "team lunch", Timestamp: 0, Status: model.StatusCleared, Type: model.TxTypeExpense})
		require.NoError(t, err)
		assert.Greater(t, result.ID, int64(0))
		assert.Equal(t, "team lunch", result.Description)
		assert.Equal(t, model.TxTypeExpense, result.Type)
		assert.Equal(t, splits, result.Splits)
	})

	t.Run("propagates CreateTransaction error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		txRepo.createErr = errors.New("db error")
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{AccountName: "Assets:Bank", Amount: -200000},
			{AccountName: "Expenses:Food", Amount: 200000},
		}
		_, err := svc.CreateTransactionFromSplits(context.Background(), model.CreateTransactionFromSplitsInput{Splits: splits, Description: "team lunch", Timestamp: 0, Status: model.StatusCleared, Type: model.TxTypeExpense})
		require.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// ParseTransactionDate
// ──────────────────────────────────────────────

func TestParseTransactionDate(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	t.Run("empty string returns current time", func(t *testing.T) {
		before := time.Now().Unix()
		got, err := svc.ParseTransactionDate("")
		after := time.Now().Unix()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, got, before)
		assert.LessOrEqual(t, got, after)
	})

	t.Run("valid date string parses correctly", func(t *testing.T) {
		got, err := svc.ParseTransactionDate("2025-06-15")
		require.NoError(t, err)
		parsed := time.Unix(got, 0)
		assert.Equal(t, 2025, parsed.Year())
		assert.Equal(t, time.June, parsed.Month())
		assert.Equal(t, 15, parsed.Day())
	})

	t.Run("invalid format returns error", func(t *testing.T) {
		_, err := svc.ParseTransactionDate("15/06/2025")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "2006-01-02")
	})

	t.Run("partial date returns error", func(t *testing.T) {
		_, err := svc.ParseTransactionDate("2025-06")
		assert.Error(t, err)
	})
}
