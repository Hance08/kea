// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// Test helper: populate splitsWithAccts map
// ──────────────────────────────────────────────

// addTxSplits inserts a set of SplitDetails for a transaction into the map.
func addTxSplits(m map[int64][]model.SplitDetail, txID int64, splits ...model.SplitDetail) {
	m[txID] = splits
}

// ──────────────────────────────────────────────
// offsetAccountName (package-level pure function, white-box test)
// ──────────────────────────────────────────────

func TestOffsetAccountName(t *testing.T) {
	tests := []struct {
		name        string
		splits      []model.SplitDetail
		primaryType model.AccountType
		want        string
	}{
		{
			name: "single offset",
			splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500},
				{AccountName: "Assets:Cash", AccountType: model.AccountTypeAsset, Amount: -500},
			},
			primaryType: model.AccountTypeExpense,
			want:        "Assets:Cash",
		},
		{
			name: "multiple offsets returns (multiple)",
			splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500},
				{AccountName: "Assets:Cash", AccountType: model.AccountTypeAsset, Amount: -300},
				{AccountName: "Assets:Card", AccountType: model.AccountTypeAsset, Amount: -200},
			},
			primaryType: model.AccountTypeExpense,
			want:        "(multiple)",
		},
		{
			name: "no offset (all splits are primary type) returns empty string",
			splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 300},
				{AccountName: "Expenses:Drink", AccountType: model.AccountTypeExpense, Amount: 200},
			},
			primaryType: model.AccountTypeExpense,
			want:        "",
		},
		{
			name:        "empty splits returns empty string",
			splits:      []model.SplitDetail{},
			primaryType: model.AccountTypeExpense,
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := offsetAccountName(tt.splits, tt.primaryType)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ──────────────────────────────────────────────
// GetNetWorthAt
// ──────────────────────────────────────────────

func TestGetNetWorthAt(t *testing.T) {
	t.Run("assets only", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Assets:Bank", model.AccountTypeAsset, 10000),
			split("Equity:Opening", model.AccountTypeEquity, -10000),
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		worth, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		assert.Equal(t, int64(10000), worth)
	})

	t.Run("assets minus liabilities", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Assets:Bank", model.AccountTypeAsset, 15000),
			split("Equity:Opening", model.AccountTypeEquity, -15000),
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			split("Liabilities:Card", model.AccountTypeLiability, 5000),
			split("Assets:Bank", model.AccountTypeAsset, -5000),
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		// totalAssets = 15000 + (-5000) = 10000
		// totalLiabilities = 5000
		// netWorth = 10000 - 5000 = 5000
		worth, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		assert.Equal(t, int64(5000), worth)
	})

	t.Run("no transactions returns zero net worth", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		worth, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), worth)
	})

	t.Run("only income/expense splits are excluded from net worth", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Expenses:Food", model.AccountTypeExpense, 500),
			split("Revenue:Salary", model.AccountTypeRevenue, -500),
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		worth, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), worth)
	})

	t.Run("repo failure returns error", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.splitsRangeErr = assert.AnError
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		_, err := svc.GetNetWorthAt(context.Background(), 0)
		assert.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// GenerateIncomeStatement
// ──────────────────────────────────────────────

func TestGenerateIncomeStatement(t *testing.T) {
	t.Run("single income transaction", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Revenue:Salary", model.AccountTypeRevenue, -2000),
			split("Assets:Bank", model.AccountTypeAsset, 2000),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2000), result.TotalIncome)
		assert.Equal(t, int64(0), result.TotalExpense)
		assert.Equal(t, int64(2000), result.NetAmount)
		require.Len(t, result.IncomeRows, 1)
		assert.Equal(t, "Revenue:Salary", result.IncomeRows[0].AccountName)
	})

	t.Run("single expense transaction", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Expenses:Food", model.AccountTypeExpense, 500),
			split("Assets:Bank", model.AccountTypeAsset, -500),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeExpense}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), result.TotalIncome)
		assert.Equal(t, int64(500), result.TotalExpense)
		assert.Equal(t, int64(-500), result.NetAmount)
		require.Len(t, result.ExpenseRows, 1)
		assert.Equal(t, "Expenses:Food", result.ExpenseRows[0].AccountName)
	})

	t.Run("income and expense both present", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Revenue:Salary", model.AccountTypeRevenue, -3000),
			split("Assets:Bank", model.AccountTypeAsset, 3000),
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			split("Expenses:Food", model.AccountTypeExpense, 800),
			split("Assets:Bank", model.AccountTypeAsset, -800),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Type: model.TxTypeExpense}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(3000), result.TotalIncome)
		assert.Equal(t, int64(800), result.TotalExpense)
		assert.Equal(t, int64(2200), result.NetAmount) // 3000 - 800
	})

	t.Run("transfer transaction excluded from income statement", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Assets:Savings", model.AccountTypeAsset, 1000),
			split("Assets:Checking", model.AccountTypeAsset, -1000),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeTransfer}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), result.TotalIncome)
		assert.Equal(t, int64(0), result.TotalExpense)
		assert.Empty(t, result.IncomeRows)
		assert.Empty(t, result.ExpenseRows)
	})

	t.Run("empty data returns all-zero result", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		result, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), result.TotalIncome)
		assert.Equal(t, int64(0), result.TotalExpense)
		assert.Equal(t, int64(0), result.NetAmount)
	})

	t.Run("IncomeRows sorted by amount descending", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Revenue:Bonus", model.AccountTypeRevenue, -500),
			split("Assets:Bank", model.AccountTypeAsset, 500),
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			split("Revenue:Salary", model.AccountTypeRevenue, -3000),
			split("Assets:Bank", model.AccountTypeAsset, 3000),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Type: model.TxTypeIncome}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		require.NoError(t, err)
		require.Len(t, result.IncomeRows, 2)
		assert.GreaterOrEqual(t, result.IncomeRows[0].Amount, result.IncomeRows[1].Amount)
	})

	t.Run("multiple transactions for the same account are accumulated", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Expenses:Food", model.AccountTypeExpense, 300),
			split("Assets:Bank", model.AccountTypeAsset, -300),
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			split("Expenses:Food", model.AccountTypeExpense, 200),
			split("Assets:Bank", model.AccountTypeAsset, -200),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeExpense}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Type: model.TxTypeExpense}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(500), result.TotalExpense)
		// two transactions for the same account should be merged into one row
		require.Len(t, result.ExpenseRows, 1)
		assert.Equal(t, int64(500), result.ExpenseRows[0].Amount)
		assert.Equal(t, 2, result.ExpenseRows[0].TxCount)
	})

	t.Run("repo failure returns error", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.splitsRangeErr = assert.AnError
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		_, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		assert.Error(t, err)
	})

	t.Run("txs by date range error returns error", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.txsByDateRangeErr = assert.AnError
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		_, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		assert.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// GenerateIncomeBreakdown
// ──────────────────────────────────────────────

func TestGenerateIncomeBreakdown(t *testing.T) {
	t.Run("returns only income rows, expense rows are empty", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Revenue:Salary", model.AccountTypeRevenue, -2000),
			split("Assets:Bank", model.AccountTypeAsset, 2000),
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			split("Expenses:Food", model.AccountTypeExpense, 500),
			split("Assets:Bank", model.AccountTypeAsset, -500),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Type: model.TxTypeExpense}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateIncomeBreakdown(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2000), result.TotalIncome)
		assert.NotEmpty(t, result.IncomeRows)
		assert.Empty(t, result.ExpenseRows)
	})

	t.Run("rows sorted by amount descending", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Revenue:Bonus", model.AccountTypeRevenue, -100),
			split("Assets:Bank", model.AccountTypeAsset, 100),
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			split("Revenue:Salary", model.AccountTypeRevenue, -5000),
			split("Assets:Bank", model.AccountTypeAsset, 5000),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Type: model.TxTypeIncome}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateIncomeBreakdown(context.Background(), 0, 0)
		require.NoError(t, err)
		require.Len(t, result.IncomeRows, 2)
		assert.GreaterOrEqual(t, result.IncomeRows[0].Amount, result.IncomeRows[1].Amount)
	})
}

// ──────────────────────────────────────────────
// GenerateExpenseBreakdown
// ──────────────────────────────────────────────

func TestGenerateExpenseBreakdown(t *testing.T) {
	t.Run("returns only expense rows, income rows are empty", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Revenue:Salary", model.AccountTypeRevenue, -2000),
			split("Assets:Bank", model.AccountTypeAsset, 2000),
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			split("Expenses:Food", model.AccountTypeExpense, 500),
			split("Assets:Bank", model.AccountTypeAsset, -500),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Type: model.TxTypeExpense}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateExpenseBreakdown(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(500), result.TotalExpense)
		assert.NotEmpty(t, result.ExpenseRows)
		assert.Empty(t, result.IncomeRows)
	})
}

// ──────────────────────────────────────────────
// GenerateBalanceSheet
// ──────────────────────────────────────────────

func TestGenerateBalanceSheet(t *testing.T) {
	t.Run("basic asset and liability calculation", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Liabilities:Card", Type: model.AccountTypeLiability})
		accRepo.balances[1] = 10000
		accRepo.balances[2] = 3000
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		assert.Equal(t, int64(10000), result.TotalAssets)
		assert.Equal(t, int64(3000), result.TotalLiabilities)
		assert.Equal(t, int64(7000), result.NetWorth) // 10000 - 3000
		require.Len(t, result.Assets, 1)
		require.Len(t, result.Liabilities, 1)
	})

	t.Run("accounts with zero balance are excluded from the report", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Empty", Type: model.AccountTypeAsset})
		accRepo.balances[1] = 5000
		accRepo.balances[2] = 0 // zero balance
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		assert.Len(t, result.Assets, 1)
		assert.Equal(t, "Assets:Bank", result.Assets[0].AccountName)
	})

	t.Run("equity accounts are classified correctly", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Equity:Retained", Type: model.AccountTypeEquity})
		accRepo.balances[1] = 5000
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		assert.Equal(t, int64(5000), result.TotalEquity)
		require.Len(t, result.Equity, 1)
	})

	t.Run("account with custom currency uses account currency", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:TWD", Type: model.AccountTypeAsset, Currency: "TWD"})
		accRepo.balances[1] = 10000
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		require.Len(t, result.Assets, 1)
		assert.Equal(t, "TWD", result.Assets[0].Currency)
	})

	t.Run("account without custom currency falls back to system default USD", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: ""})
		accRepo.balances[1] = 10000
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		require.Len(t, result.Assets, 1)
		assert.Equal(t, "USD", result.Assets[0].Currency)
	})

	t.Run("Assets sorted by amount descending", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Small", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Big", Type: model.AccountTypeAsset})
		accRepo.balances[1] = 1000
		accRepo.balances[2] = 9000
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		require.Len(t, result.Assets, 2)
		assert.GreaterOrEqual(t, result.Assets[0].Amount, result.Assets[1].Amount)
	})

	t.Run("GetAllAccountBalances failure returns error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.getAllBalancesErr = assert.AnError
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		_, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		assert.Error(t, err)
	})
}
