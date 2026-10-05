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

func boolp(b bool) *bool { return &b }

// reportFixture: Expenses:Food (USD) with children Dining (USD) and Japan (JPY),
// Expenses:FoodTruck (USD, a name-prefix trap), Expenses:Rent (USD).
func reportFixture() (*mockAccountRepo, *mockTransactionRepo, *mockBudgetRepo) {
	accRepo := newMockAccountRepo()
	for _, a := range []*model.Account{
		{ID: 1, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"},
		{ID: 2, Name: "Expenses:Food:Dining", Type: model.AccountTypeExpense, Currency: "USD"},
		{ID: 3, Name: "Expenses:Food:Japan", Type: model.AccountTypeExpense, Currency: "JPY"},
		{ID: 4, Name: "Expenses:FoodTruck", Type: model.AccountTypeExpense, Currency: "USD"},
		{ID: 5, Name: "Expenses:Rent", Type: model.AccountTypeExpense, Currency: ""}, // default currency (USD)
		{ID: 6, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"},
	} {
		accRepo.addAccount(a)
	}
	return accRepo, newMockTransactionRepo(), newMockBudgetRepo(accRepo)
}

func addExpenseTx(txRepo *mockTransactionRepo, id int64, txType model.TransactionType, regular *bool, splits ...model.SplitDetail) {
	addTxSplits(txRepo.splitsWithAccts, id, splits...)
	txRepo.addTransaction(&model.Transaction{ID: id, Type: txType, Regular: regular}, nil)
}

func findRow(t *testing.T, r *model.BudgetReport, name string) model.BudgetReportRow {
	t.Helper()
	for _, row := range r.Rows {
		if row.AccountName == name {
			return row
		}
	}
	t.Fatalf("row %q not found", name)
	return model.BudgetReportRow{}
}

func TestGenerateBudgetReport(t *testing.T) {
	ctx := context.Background()

	t.Run("aggregates descendants, signed, split by regular", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100000, false)
		addExpenseTx(txRepo, 1, model.TxTypeExpense, boolp(true),
			split("Expenses:Food", model.AccountTypeExpense, 3000),
			split("Assets:Bank", model.AccountTypeAsset, -3000))
		addExpenseTx(txRepo, 2, model.TxTypeExpense, boolp(false),
			split("Expenses:Food:Dining", model.AccountTypeExpense, 5000),
			split("Assets:Bank", model.AccountTypeAsset, -5000))
		// Refund reduces actual.
		addExpenseTx(txRepo, 3, model.TxTypeExpense, boolp(false),
			split("Expenses:Food:Dining", model.AccountTypeExpense, -1000),
			split("Assets:Bank", model.AccountTypeAsset, 1000))
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, "2026-03", r.Month)
		row := findRow(t, r, "Expenses:Food")
		assert.Equal(t, "USD", row.Currency)
		assert.Equal(t, "2026-01", row.EffectiveMonth)
		assert.Equal(t, int64(100000), row.Budget)
		assert.Equal(t, int64(7000), row.Actual)
		assert.Equal(t, int64(3000), row.ActualRegular)
		assert.Equal(t, int64(4000), row.ActualIrregular)
		assert.Equal(t, int64(93000), row.Remaining)
	})

	t.Run("name prefix is not a descendant", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100, false)
		addExpenseTx(txRepo, 1, model.TxTypeExpense, boolp(true),
			split("Expenses:FoodTruck", model.AccountTypeExpense, 900),
			split("Assets:Bank", model.AccountTypeAsset, -900))
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, int64(0), findRow(t, r, "Expenses:Food").Actual)
	})

	t.Run("other-currency descendants are excluded and listed", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100, false)
		jpy := split("Expenses:Food:Japan", model.AccountTypeExpense, 50000)
		jpy.Currency = "JPY"
		bank := split("Assets:Bank", model.AccountTypeAsset, -50000)
		bank.Currency = "JPY"
		addExpenseTx(txRepo, 1, model.TxTypeExpense, boolp(true), jpy, bank)
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-01")
		require.NoError(t, err)
		row := findRow(t, r, "Expenses:Food")
		assert.Equal(t, int64(0), row.Actual)
		assert.Equal(t, []string{"Expenses:Food:Japan"}, row.ExcludedAccounts)
	})

	t.Run("only Expense-typed transactions count", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100, false)
		for i, typ := range []model.TransactionType{model.TxTypeTransfer, model.TxTypeInvestment, model.TxTypeOther, model.TxTypeOpening} {
			addExpenseTx(txRepo, int64(i+1), typ, nil,
				split("Expenses:Food", model.AccountTypeExpense, 700),
				split("Assets:Bank", model.AccountTypeAsset, -700))
		}
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, int64(0), findRow(t, r, "Expenses:Food").Actual)
	})

	t.Run("empty account currency uses default", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 5, "2026-01", 2000, false)
		addExpenseTx(txRepo, 1, model.TxTypeExpense, boolp(true),
			split("Expenses:Rent", model.AccountTypeExpense, 1500),
			split("Assets:Bank", model.AccountTypeAsset, -1500))
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-01")
		require.NoError(t, err)
		row := findRow(t, r, "Expenses:Rent")
		assert.Equal(t, "USD", row.Currency)
		assert.Equal(t, int64(1500), row.Actual)
	})

	t.Run("totals count only top-level budgeted rows", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 10000, false) // Food
		_, _ = bRepo.UpsertBudget(ctx, 2, "2026-01", 6000, false)  // Food:Dining
		_, _ = bRepo.UpsertBudget(ctx, 5, "2026-01", 20000, false) // Rent
		addExpenseTx(txRepo, 1, model.TxTypeExpense, boolp(true),
			split("Expenses:Food:Dining", model.AccountTypeExpense, 4000),
			split("Assets:Bank", model.AccountTypeAsset, -4000))
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-01")
		require.NoError(t, err)
		require.Len(t, r.Rows, 3)
		assert.Equal(t, []string{"Expenses:Food", "Expenses:Food:Dining", "Expenses:Rent"},
			[]string{r.Rows[0].AccountName, r.Rows[1].AccountName, r.Rows[2].AccountName})
		assert.Equal(t, int64(30000), r.TotalBudget["USD"])
		assert.Equal(t, int64(4000), r.TotalActual["USD"])
	})

	t.Run("stopped and future versions are not reported", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100, false)
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-02", 0, true)
		_, _ = bRepo.UpsertBudget(ctx, 5, "2026-09", 100, false)
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-03")
		require.NoError(t, err)
		assert.NotNil(t, r.Rows)
		assert.Empty(t, r.Rows)
		assert.NotNil(t, r.TotalBudget)
		assert.NotNil(t, r.TotalActual)
	})

	t.Run("default month is the current local month", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		r, err := svc.GenerateBudgetReport(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, time.Now().Format("2006-01"), r.Month)
	})

	t.Run("invalid month", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.GenerateBudgetReport(ctx, "2026-13")
		var verr *ValidationError
		require.True(t, errors.As(err, &verr))
		assert.Equal(t, "month", verr.Field)
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100, false)
		txRepo.splitsRangeErr = errors.New("boom")
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.GenerateBudgetReport(ctx, "2026-01")
		assert.ErrorContains(t, err, "boom")
	})
}
