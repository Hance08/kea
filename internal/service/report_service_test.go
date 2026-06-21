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
// Test helper: populate splitsWithAccts map
// ──────────────────────────────────────────────

// addTxSplits inserts a set of SplitDetails for a transaction into the map.
func addTxSplits(m map[int64][]model.SplitDetail, txID int64, splits ...model.SplitDetail) {
	m[txID] = splits
}

// ──────────────────────────────────────────────
// ResolveDateRange
// ──────────────────────────────────────────────

func TestResolveDateRange(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	t.Run("month format parses correctly", func(t *testing.T) {
		start, end, period, err := svc.ResolveDateRange(DateRangeParams{Month: "2025-03"})
		require.NoError(t, err)

		startT := time.Unix(start, 0)
		endT := time.Unix(end, 0)
		assert.Equal(t, 2025, startT.Year())
		assert.Equal(t, time.March, startT.Month())
		assert.Equal(t, 1, startT.Day())
		assert.Equal(t, 2025, endT.Year())
		assert.Equal(t, time.March, endT.Month())
		assert.Equal(t, 31, endT.Day())
		assert.Equal(t, "March 2025", period)
	})

	t.Run("invalid month format returns error", func(t *testing.T) {
		_, _, _, err := svc.ResolveDateRange(DateRangeParams{Month: "2025/03"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "YYYY-MM")
	})

	t.Run("from and to date range", func(t *testing.T) {
		start, end, period, err := svc.ResolveDateRange(DateRangeParams{
			From: "2025-01-15",
			To:   "2025-02-15",
		})
		require.NoError(t, err)
		assert.Greater(t, end, start)
		assert.Contains(t, period, "2025-01-15")
		assert.Contains(t, period, "2025-02-15")
	})

	t.Run("from only defaults to epoch start", func(t *testing.T) {
		_, _, _, err := svc.ResolveDateRange(DateRangeParams{From: "2025-01-15"})
		require.NoError(t, err)
	})

	t.Run("to only defaults from epoch", func(t *testing.T) {
		start, _, _, err := svc.ResolveDateRange(DateRangeParams{To: "2025-06-15"})
		require.NoError(t, err)
		assert.Equal(t, int64(0), start)
	})

	t.Run("to before from returns error", func(t *testing.T) {
		_, _, _, err := svc.ResolveDateRange(DateRangeParams{
			From: "2025-06-15",
			To:   "2025-01-01",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "on or after")
	})

	t.Run("invalid from format returns error", func(t *testing.T) {
		_, _, _, err := svc.ResolveDateRange(DateRangeParams{From: "not-a-date"})
		assert.Error(t, err)
	})

	t.Run("invalid to format returns error", func(t *testing.T) {
		_, _, _, err := svc.ResolveDateRange(DateRangeParams{To: "not-a-date"})
		assert.Error(t, err)
	})

	t.Run("empty params defaults to current month", func(t *testing.T) {
		start, end, period, err := svc.ResolveDateRange(DateRangeParams{})
		require.NoError(t, err)

		now := time.Now()
		startT := time.Unix(start, 0)
		assert.Equal(t, now.Year(), startT.Year())
		assert.Equal(t, now.Month(), startT.Month())
		assert.Equal(t, 1, startT.Day())
		assert.Greater(t, end, start)
		assert.NotEmpty(t, period)
	})

	t.Run("month takes priority over from/to", func(t *testing.T) {
		start, _, period, err := svc.ResolveDateRange(DateRangeParams{
			Month: "2025-06",
			From:  "2024-01-01",
			To:    "2024-12-31",
		})
		require.NoError(t, err)
		startT := time.Unix(start, 0)
		assert.Equal(t, 2025, startT.Year())
		assert.Equal(t, time.June, startT.Month())
		assert.Equal(t, "June 2025", period)
	})
}

// ──────────────────────────────────────────────
// previousPeriodRange (white-box)
// ──────────────────────────────────────────────

func TestPreviousPeriodRange(t *testing.T) {
	t.Run("30-day period", func(t *testing.T) {
		start := int64(1000)
		end := int64(1000 + 30*86400 - 1)
		prevStart, prevEnd := previousPeriodRange(start, end)
		assert.Equal(t, start-1, prevEnd)
		assert.Equal(t, end-start, prevEnd-prevStart)
	})

	t.Run("single-day period", func(t *testing.T) {
		start := int64(86400)
		end := int64(86400)
		prevStart, prevEnd := previousPeriodRange(start, end)
		assert.Equal(t, start-1, prevEnd)
		assert.Equal(t, prevStart, prevEnd)
	})
}

// ──────────────────────────────────────────────
// computeNetWorthGrowthPctMap (white-box)
// ──────────────────────────────────────────────

func TestComputeNetWorthGrowthPctMap(t *testing.T) {
	t.Run("basic growth", func(t *testing.T) {
		current := map[string]int64{"USD": 11000}
		previous := map[string]int64{"USD": 10000}
		result := computeNetWorthGrowthPctMap(current, previous)
		assert.InDelta(t, 10.0, result["USD"], 0.01)
	})

	t.Run("zero previous is omitted", func(t *testing.T) {
		current := map[string]int64{"USD": 5000}
		previous := map[string]int64{"USD": 0}
		result := computeNetWorthGrowthPctMap(current, previous)
		_, exists := result["USD"]
		assert.False(t, exists)
	})

	t.Run("negative previous uses absolute value", func(t *testing.T) {
		current := map[string]int64{"USD": -500}
		previous := map[string]int64{"USD": -1000}
		result := computeNetWorthGrowthPctMap(current, previous)
		assert.InDelta(t, 50.0, result["USD"], 0.01)
	})

	t.Run("multi-currency", func(t *testing.T) {
		current := map[string]int64{"USD": 12000, "TWD": 60000}
		previous := map[string]int64{"USD": 10000, "TWD": 50000}
		result := computeNetWorthGrowthPctMap(current, previous)
		assert.InDelta(t, 20.0, result["USD"], 0.01)
		assert.InDelta(t, 20.0, result["TWD"], 0.01)
	})

	t.Run("currency only in current is omitted", func(t *testing.T) {
		current := map[string]int64{"USD": 5000, "TWD": 10000}
		previous := map[string]int64{"USD": 4000}
		result := computeNetWorthGrowthPctMap(current, previous)
		assert.InDelta(t, 25.0, result["USD"], 0.01)
		_, exists := result["TWD"]
		assert.False(t, exists)
	})
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
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		nw, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		assert.Equal(t, int64(10000), nw["USD"])
	})

	t.Run("assets minus liabilities", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 15000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -15000, Currency: "USD"},
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: -5000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: 5000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		// assets = 15000, liabilities = -5000, net worth = 15000 + (-5000) = 10000
		nw, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		assert.Equal(t, int64(10000), nw["USD"])
	})

	t.Run("no transactions returns zero net worth", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		nw, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		assert.Empty(t, nw)
	})

	t.Run("only income/expense splits are excluded from net worth", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
			model.SplitDetail{AccountName: "Revenue:Salary", AccountType: model.AccountTypeRevenue, Amount: -500, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		nw, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		assert.Empty(t, nw)
	})

	t.Run("repo failure returns error", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.splitsRangeErr = assert.AnError
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		_, err := svc.GetNetWorthAt(context.Background(), 0)
		assert.Error(t, err)
	})

	t.Run("multi-currency keeps totals separate", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:USD_Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening_USD", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			model.SplitDetail{AccountName: "Assets:TWD_Bank", AccountType: model.AccountTypeAsset, Amount: 50000, Currency: "TWD"},
			model.SplitDetail{AccountName: "Equity:Opening_TWD", AccountType: model.AccountTypeEquity, Amount: -50000, Currency: "TWD"},
		)
		addTxSplits(txRepo.splitsWithAccts, 3,
			model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: -2000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening_USD", AccountType: model.AccountTypeEquity, Amount: 2000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		nw, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		// USD: assets 10000, liabilities -2000, net worth 10000 + (-2000) = 8000
		assert.Equal(t, int64(8000), nw["USD"])
		// TWD: assets 50000, liabilities 0 → net 50000
		assert.Equal(t, int64(50000), nw["TWD"])
		assert.Len(t, nw, 2)
	})

	t.Run("liability-only currency yields negative net worth", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: -3000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: 3000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		nw, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		assert.Equal(t, int64(-3000), nw["USD"])
	})

	t.Run("liability with partial payment", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: -5000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: 5000, Currency: "USD"},
		)
		addTxSplits(txRepo.splitsWithAccts, 3,
			model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: 3000, Currency: "USD"},
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: -3000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		// assets: 10000 - 3000 = 7000
		// liabilities: -5000 + 3000 = -2000
		// net worth: 7000 + (-2000) = 5000
		nw, err := svc.GetNetWorthAt(context.Background(), 0)
		require.NoError(t, err)
		assert.Equal(t, int64(5000), nw["USD"])
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
		assert.Equal(t, int64(2000), result.TotalIncome["USD"])
		assert.Empty(t, result.TotalExpense)
		assert.Equal(t, int64(2000), result.NetAmount["USD"])
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
		assert.Empty(t, result.TotalIncome)
		assert.Equal(t, int64(500), result.TotalExpense["USD"])
		assert.Equal(t, int64(-500), result.NetAmount["USD"])
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
		assert.Equal(t, int64(3000), result.TotalIncome["USD"])
		assert.Equal(t, int64(800), result.TotalExpense["USD"])
		assert.Equal(t, int64(2200), result.NetAmount["USD"]) // 3000 - 800
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
		assert.Empty(t, result.TotalIncome)
		assert.Empty(t, result.TotalExpense)
		assert.Empty(t, result.IncomeRows)
		assert.Empty(t, result.ExpenseRows)
	})

	t.Run("empty data returns all-zero result", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		result, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Empty(t, result.TotalIncome)
		assert.Empty(t, result.TotalExpense)
		assert.Empty(t, result.NetAmount)
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
		assert.Equal(t, int64(500), result.TotalExpense["USD"])
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

	t.Run("multi-currency totals are grouped by currency", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Revenue:Salary", AccountType: model.AccountTypeRevenue, Amount: -3000, Currency: "USD"},
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 3000, Currency: "USD"},
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			model.SplitDetail{AccountName: "Revenue:Freelance", AccountType: model.AccountTypeRevenue, Amount: -90000, Currency: "TWD"},
			model.SplitDetail{AccountName: "Assets:TWD_Bank", AccountType: model.AccountTypeAsset, Amount: 90000, Currency: "TWD"},
		)
		addTxSplits(txRepo.splitsWithAccts, 3,
			model.SplitDetail{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: -500, Currency: "USD"},
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Type: model.TxTypeIncome}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 3, Type: model.TxTypeExpense}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateIncomeStatement(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(3000), result.TotalIncome["USD"])
		assert.Equal(t, int64(90000), result.TotalIncome["TWD"])
		assert.Equal(t, int64(500), result.TotalExpense["USD"])
		assert.Equal(t, int64(2500), result.NetAmount["USD"])  // 3000 - 500
		assert.Equal(t, int64(90000), result.NetAmount["TWD"]) // 90000 - 0
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
		assert.Equal(t, int64(2000), result.TotalIncome["USD"])
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

	t.Run("unused expense rows slice is non-nil so it serializes as []", func(t *testing.T) {
		// Reason: JSON marshals a nil slice as `null`, which crashes the SPA's
		// `result.expense_rows.filter(...)`. The handler must emit `[]`.
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		result, err := svc.GenerateIncomeBreakdown(context.Background(), 0, 0)
		require.NoError(t, err)
		require.NotNil(t, result.ExpenseRows)
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
		assert.Equal(t, int64(500), result.TotalExpense["USD"])
		assert.NotEmpty(t, result.ExpenseRows)
		assert.Empty(t, result.IncomeRows)
	})

	t.Run("unused income rows slice is non-nil so it serializes as []", func(t *testing.T) {
		// Reason: a nil slice JSON-marshals to `null` and crashes the SPA.
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		result, err := svc.GenerateExpenseBreakdown(context.Background(), 0, 0)
		require.NoError(t, err)
		require.NotNil(t, result.IncomeRows)
	})
}

// ──────────────────────────────────────────────
// GenerateFullIncomeStatement
// ──────────────────────────────────────────────

func TestGenerateFullIncomeStatement(t *testing.T) {
	t.Run("populates period, net worth, and growth", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		addTxSplits(txRepo.splitsWithAccts, 1,
			split("Revenue:Salary", model.AccountTypeRevenue, -3000),
			split("Assets:Bank", model.AccountTypeAsset, 3000),
		)
		txRepo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		result, err := svc.GenerateFullIncomeStatement(context.Background(), DateRangeParams{Month: "2025-03"})
		require.NoError(t, err)
		assert.Equal(t, "March 2025", result.Period)
		assert.NotNil(t, result.NetWorth)
		assert.NotNil(t, result.PreviousNetWorth)
		assert.NotNil(t, result.NetWorthGrowthPct)
		assert.Equal(t, int64(3000), result.TotalIncome["USD"])
	})

	t.Run("invalid date params returns error", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.GenerateFullIncomeStatement(context.Background(), DateRangeParams{Month: "bad"})
		assert.Error(t, err)
	})

	t.Run("income statement error propagates", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.splitsRangeErr = assert.AnError
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		_, err := svc.GenerateFullIncomeStatement(context.Background(), DateRangeParams{Month: "2025-03"})
		assert.Error(t, err)
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
		accRepo.balances[2] = -3000
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		assert.Equal(t, int64(10000), result.TotalAssets["USD"])
		assert.Equal(t, int64(3000), result.TotalLiabilities["USD"])
		assert.Equal(t, int64(7000), result.NetWorth["USD"]) // 10000 - 3000
		require.Len(t, result.Assets, 1)
		require.Len(t, result.Liabilities, 1)
		assert.Equal(t, int64(3000), result.Liabilities[0].Amount)
	})

	t.Run("accounts with zero balance are included in the report", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Empty", Type: model.AccountTypeAsset})
		accRepo.balances[1] = 5000
		accRepo.balances[2] = 0
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		assert.Len(t, result.Assets, 2)
		assert.Equal(t, "Assets:Bank", result.Assets[0].AccountName)
		assert.Equal(t, int64(5000), result.Assets[0].Amount)
		assert.Equal(t, "Assets:Empty", result.Assets[1].AccountName)
		assert.Equal(t, int64(0), result.Assets[1].Amount)
	})

	t.Run("zero-balance liability is included", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Liabilities:Empty", Type: model.AccountTypeLiability})
		accRepo.balances[1] = 0
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		assert.Len(t, result.Liabilities, 1)
		assert.Equal(t, int64(0), result.Liabilities[0].Amount)
	})

	t.Run("zero-balance equity is included", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Equity:Empty", Type: model.AccountTypeEquity})
		accRepo.balances[1] = 0
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		assert.Len(t, result.Equity, 1)
		assert.Equal(t, int64(0), result.Equity[0].Amount)
	})

	t.Run("equity accounts are classified correctly", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Equity:Retained", Type: model.AccountTypeEquity})
		accRepo.balances[1] = 5000
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		assert.Equal(t, int64(5000), result.TotalEquity["USD"])
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

	t.Run("multi-currency totals are grouped by currency", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:USD_Bank", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:TWD_Bank", Type: model.AccountTypeAsset, Currency: "TWD"})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Liabilities:Card", Type: model.AccountTypeLiability, Currency: "USD"})
		accRepo.balances[1] = 10000
		accRepo.balances[2] = 50000
		accRepo.balances[3] = -3000
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		assert.Equal(t, int64(10000), result.TotalAssets["USD"])
		assert.Equal(t, int64(50000), result.TotalAssets["TWD"])
		assert.Equal(t, int64(3000), result.TotalLiabilities["USD"])
		assert.Equal(t, int64(7000), result.NetWorth["USD"])   // 10000 - 3000
		assert.Equal(t, int64(50000), result.NetWorth["TWD"])  // 50000 - 0
	})

	t.Run("overpaid liability has negative display balance", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Liabilities:Card", Type: model.AccountTypeLiability})
		accRepo.balances[1] = 7000
		accRepo.balances[2] = 1000
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		assert.Equal(t, int64(7000), result.TotalAssets["USD"])
		assert.Equal(t, int64(-1000), result.TotalLiabilities["USD"])
		assert.Equal(t, int64(8000), result.NetWorth["USD"]) // 7000 - (-1000)
		require.Len(t, result.Liabilities, 1)
		assert.Equal(t, int64(-1000), result.Liabilities[0].Amount)
	})

	t.Run("empty ledger returns non-nil slices that serialize as []", func(t *testing.T) {
		// Reason: a nil slice JSON-marshals to `null` and crashes the SPA's
		// `result.liabilities.filter(...)` on databases with no liabilities (or
		// no assets, or no equity accounts).
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		result, err := svc.GenerateBalanceSheet(context.Background(), 9999999999)
		require.NoError(t, err)
		require.NotNil(t, result.Assets)
		require.NotNil(t, result.Liabilities)
		require.NotNil(t, result.Equity)
	})
}

// ──────────────────────────────────────────────
// GetDailyNetWorthSeries
// ──────────────────────────────────────────────

func TestGetDailyNetWorthSeries(t *testing.T) {
	// 2026-01-01 00:00:00 UTC and 2026-01-03 00:00:00 UTC.
	day1 := int64(1767225600)
	day3 := day1 + 2*86400

	t.Run("empty ledger returns empty slice", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		out, err := svc.GetDailyNetWorthSeries(context.Background())
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("single-day single-currency emits one point", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: day1}, nil)
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		out, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day1)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, "USD", out[0].Currency)
		require.Len(t, out[0].Points, 1)
		assert.Equal(t, "2026-01-01", out[0].Points[0].Date)
		assert.Equal(t, int64(10000), out[0].Points[0].Balance)
	})

	t.Run("front-fills days without activity", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: day1}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Timestamp: day3}, nil)
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 5000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -5000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		out, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day3)
		require.NoError(t, err)
		require.Len(t, out, 1)
		pts := out[0].Points
		require.Len(t, pts, 3)
		assert.Equal(t, "2026-01-01", pts[0].Date)
		assert.Equal(t, int64(10000), pts[0].Balance)
		assert.Equal(t, "2026-01-02", pts[1].Date)
		assert.Equal(t, int64(10000), pts[1].Balance) // front-filled
		assert.Equal(t, "2026-01-03", pts[2].Date)
		assert.Equal(t, int64(15000), pts[2].Balance)
	})

	t.Run("multi-currency keeps series separate", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: day1}, nil)
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:USD_Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Assets:TWD_Bank", AccountType: model.AccountTypeAsset, Amount: 50000, Currency: "TWD"},
			model.SplitDetail{AccountName: "Equity:Opening_USD", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening_TWD", AccountType: model.AccountTypeEquity, Amount: -50000, Currency: "TWD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		out, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day1)
		require.NoError(t, err)
		require.Len(t, out, 2)
		bySeries := map[string]int64{}
		for _, s := range out {
			require.Len(t, s.Points, 1)
			bySeries[s.Currency] = s.Points[0].Balance
		}
		assert.Equal(t, int64(10000), bySeries["USD"])
		assert.Equal(t, int64(50000), bySeries["TWD"])
	})

	t.Run("excludes income and expense splits", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: day1}, nil)
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
			model.SplitDetail{AccountName: "Revenue:Salary", AccountType: model.AccountTypeRevenue, Amount: -500, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		out, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day1)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("liability reduces net worth", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: day1}, nil)
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: -3000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -7000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		out, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day1)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, int64(7000), out[0].Points[0].Balance)
	})

	t.Run("repo failure returns error", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: day1}, nil)
		txRepo.splitsRangeErr = assert.AnError
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		_, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day1)
		assert.Error(t, err)
	})
}
