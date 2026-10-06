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

// savingsFixture: Assets:Savings (USD) with children Travel (USD) and Japan
// (JPY); Assets:SavingsBox (USD, a name-prefix trap); Assets:Bank (USD);
// Assets:Emergency (default currency); Income:Interest.
func savingsFixture() (*mockAccountRepo, *mockTransactionRepo, *mockSavingsRepo) {
	accRepo := newMockAccountRepo()
	for _, a := range []*model.Account{
		{ID: 1, Name: "Assets:Savings", Type: model.AccountTypeAsset, Currency: "USD"},
		{ID: 2, Name: "Assets:Savings:Travel", Type: model.AccountTypeAsset, Currency: "USD"},
		{ID: 3, Name: "Assets:Savings:Japan", Type: model.AccountTypeAsset, Currency: "JPY"},
		{ID: 4, Name: "Assets:SavingsBox", Type: model.AccountTypeAsset, Currency: "USD"},
		{ID: 5, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"},
		{ID: 6, Name: "Assets:Emergency", Type: model.AccountTypeAsset, Currency: ""},
		{ID: 7, Name: "Income:Interest", Type: model.AccountTypeRevenue, Currency: "USD"},
	} {
		accRepo.addAccount(a)
	}
	return accRepo, newMockTransactionRepo(), newMockSavingsRepo(accRepo)
}

// at returns noon local time on the given day.
func at(y int, m time.Month, d int) int64 {
	return time.Date(y, m, d, 12, 0, 0, 0, time.Local).Unix()
}

func addSavingsTx(txRepo *mockTransactionRepo, id, ts int64, txType model.TransactionType, splits ...model.SplitDetail) {
	addTxSplits(txRepo.splitsWithAccts, id, splits...)
	txRepo.addTransaction(&model.Transaction{ID: id, Timestamp: ts, Type: txType}, nil)
}

// transferIn moves amount from Assets:Bank into account.
func transferIn(txRepo *mockTransactionRepo, id, ts int64, account string, amount int64) {
	addSavingsTx(txRepo, id, ts, model.TxTypeTransfer,
		split(account, model.AccountTypeAsset, amount),
		split("Assets:Bank", model.AccountTypeAsset, -amount))
}

func findSavingsRow(t *testing.T, r *model.SavingsReport, name string) model.SavingsReportRow {
	t.Helper()
	for _, row := range r.Rows {
		if row.AccountName == name {
			return row
		}
	}
	t.Fatalf("row %q not found", name)
	return model.SavingsReportRow{}
}

func TestGenerateSavingsReport(t *testing.T) {
	ctx := context.Background()

	t.Run("signed sum over the account and its descendants", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-03", 10000, false)
		transferIn(txRepo, 1, at(2026, 3, 5), "Assets:Savings", 8000)
		transferIn(txRepo, 2, at(2026, 3, 10), "Assets:Savings:Travel", 3000)
		addSavingsTx(txRepo, 3, at(2026, 3, 20), model.TxTypeIncome, // interest
			split("Assets:Savings", model.AccountTypeAsset, 100),
			split("Income:Interest", model.AccountTypeRevenue, -100))
		transferIn(txRepo, 4, at(2026, 3, 25), "Assets:Savings", -2000) // withdrawal
		addSavingsTx(txRepo, 5, at(2026, 3, 26), model.TxTypeTransfer, // inside the subtree
			split("Assets:Savings", model.AccountTypeAsset, -500),
			split("Assets:Savings:Travel", model.AccountTypeAsset, 500))
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, "2026-03", r.Month)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, "USD", row.Currency)
		assert.Equal(t, "2026-03", row.EffectiveMonth)
		assert.Equal(t, int64(10000), row.Target)
		assert.Equal(t, int64(9100), row.Saved)
		assert.Equal(t, int64(900), row.Remaining)
		assert.Equal(t, []string{}, row.ExcludedAccounts)
	})

	t.Run("name prefix is not a descendant", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		transferIn(txRepo, 1, at(2026, 1, 5), "Assets:SavingsBox", 900)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, int64(0), findSavingsRow(t, r, "Assets:Savings").Saved)
	})

	t.Run("opening transactions do not count", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		addSavingsTx(txRepo, 1, at(2026, 1, 1), model.TxTypeOpening,
			split("Assets:Savings", model.AccountTypeAsset, 5000000),
			split("Equity:OpeningBalances_USD", model.AccountTypeEquity, -5000000))
		transferIn(txRepo, 2, at(2026, 1, 9), "Assets:Savings", 300)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, int64(300), row.Saved)
		assert.Equal(t, int64(300), row.YTDSaved)
	})

	t.Run("every non-opening type counts", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 0, false)
		for i, typ := range []model.TransactionType{model.TxTypeDeposit, model.TxTypeWithdrawal, model.TxTypeInvestment, model.TxTypeExpense, model.TxTypeOther} {
			addSavingsTx(txRepo, int64(i+1), at(2026, 1, 10), typ,
				split("Assets:Savings", model.AccountTypeAsset, 10),
				split("Assets:Bank", model.AccountTypeAsset, -10))
		}
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, int64(50), findSavingsRow(t, r, "Assets:Savings").Saved)
	})

	t.Run("other-currency descendants are excluded and listed for the month only", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		jpyIn := func(id, ts int64) {
			japan := split("Assets:Savings:Japan", model.AccountTypeAsset, 50000)
			japan.Currency = "JPY"
			bank := split("Assets:Bank", model.AccountTypeAsset, -50000)
			bank.Currency = "JPY"
			addSavingsTx(txRepo, id, ts, model.TxTypeTransfer, japan, bank)
		}
		jpyIn(1, at(2026, 2, 3))
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-03")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, int64(0), row.YTDSaved)
		assert.Equal(t, []string{}, row.ExcludedAccounts, "February JPY activity is not listed for March")

		jpyIn(2, at(2026, 3, 3))
		r, err = svc.GenerateSavingsReport(ctx, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, []string{"Assets:Savings:Japan"}, findSavingsRow(t, r, "Assets:Savings").ExcludedAccounts)
	})

	t.Run("ytd counts only active months, across stop and restart", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 1000, false)
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-03", 0, true)
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-05", 2000, false)
		for m := time.January; m <= time.June; m++ {
			transferIn(txRepo, int64(m), at(2026, m, 15), "Assets:Savings", 500)
		}
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-06")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, "2026-05", row.EffectiveMonth)
		assert.Equal(t, int64(2000), row.Target)
		assert.Equal(t, int64(500), row.Saved)
		assert.Equal(t, []model.SavingsMonth{
			{Month: "2026-01", Target: 1000, Saved: 500},
			{Month: "2026-02", Target: 1000, Saved: 500},
			{Month: "2026-05", Target: 2000, Saved: 500},
			{Month: "2026-06", Target: 2000, Saved: 500},
		}, row.Months)
		assert.Equal(t, int64(6000), row.YTDTarget)
		assert.Equal(t, int64(2000), row.YTDSaved)
		assert.Equal(t, int64(4000), row.YTDRemaining)
	})

	t.Run("target starting after january ignores earlier months", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-04", 1000, false)
		transferIn(txRepo, 1, at(2026, 2, 10), "Assets:Savings", 700)
		transferIn(txRepo, 2, at(2026, 4, 10), "Assets:Savings", 300)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-04")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, []model.SavingsMonth{{Month: "2026-04", Target: 1000, Saved: 300}}, row.Months)
		assert.Equal(t, int64(300), row.YTDSaved)
		assert.Equal(t, int64(1000), row.YTDTarget)
	})

	t.Run("january ytd ignores the previous year", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2025-06", 1000, false)
		transferIn(txRepo, 1, at(2025, 12, 10), "Assets:Savings", 5000)
		transferIn(txRepo, 2, at(2026, 1, 10), "Assets:Savings", 400)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, int64(400), row.Saved)
		assert.Equal(t, []model.SavingsMonth{{Month: "2026-01", Target: 1000, Saved: 400}}, row.Months)
		assert.Equal(t, int64(400), row.YTDSaved)
	})

	t.Run("months are bucketed in local time", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 1000, false)
		transferIn(txRepo, 1, time.Date(2026, 1, 31, 23, 30, 0, 0, time.Local).Unix(), "Assets:Savings", 400)
		transferIn(txRepo, 2, time.Date(2026, 2, 1, 0, 15, 0, 0, time.Local).Unix(), "Assets:Savings", 600)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, int64(400), findSavingsRow(t, r, "Assets:Savings").Saved)

		r, err = svc.GenerateSavingsReport(ctx, "2026-02")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, int64(600), row.Saved)
		assert.Equal(t, int64(1000), row.YTDSaved)
	})

	t.Run("net withdrawal gives remaining above target", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 1000, false)
		transferIn(txRepo, 1, at(2026, 1, 10), "Assets:Savings", -300)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, int64(-300), row.Saved)
		assert.Equal(t, int64(1300), row.Remaining)
	})

	t.Run("totals skip same-currency targeted descendants", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 10000, false) // Savings
		_, _ = sRepo.UpsertSavingsTarget(ctx, 2, "2026-01", 4000, false)  // Savings:Travel
		_, _ = sRepo.UpsertSavingsTarget(ctx, 3, "2026-01", 70000, false) // Savings:Japan (JPY)
		_, _ = sRepo.UpsertSavingsTarget(ctx, 6, "2026-01", 2000, false)  // Emergency (default USD)
		transferIn(txRepo, 1, at(2026, 1, 3), "Assets:Savings:Travel", 1500)
		transferIn(txRepo, 2, at(2026, 1, 4), "Assets:Emergency", 500)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		require.Len(t, r.Rows, 4)
		assert.Equal(t, []string{"Assets:Emergency", "Assets:Savings", "Assets:Savings:Japan", "Assets:Savings:Travel"},
			[]string{r.Rows[0].AccountName, r.Rows[1].AccountName, r.Rows[2].AccountName, r.Rows[3].AccountName})
		assert.Equal(t, "USD", findSavingsRow(t, r, "Assets:Emergency").Currency)
		assert.Equal(t, map[string]int64{"USD": 12000, "JPY": 70000}, r.TotalTarget)
		assert.Equal(t, map[string]int64{"USD": 2000, "JPY": 0}, r.TotalSaved)
		assert.Equal(t, map[string]int64{"USD": 12000, "JPY": 70000}, r.TotalYTDTarget)
		assert.Equal(t, map[string]int64{"USD": 2000, "JPY": 0}, r.TotalYTDSaved)
	})

	t.Run("no active target gives an empty, non-nil report", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-02", 0, true)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-03")
		require.NoError(t, err)
		assert.NotNil(t, r.Rows)
		assert.Empty(t, r.Rows)
		for _, m := range []map[string]int64{r.TotalTarget, r.TotalSaved, r.TotalYTDTarget, r.TotalYTDSaved} {
			assert.NotNil(t, m)
		}
	})

	t.Run("default month is the current local month", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		svc := newTestSavingsService(accRepo, txRepo, sRepo)
		r, err := svc.GenerateSavingsReport(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, time.Now().Format("2006-01"), r.Month)
	})

	t.Run("invalid month", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		svc := newTestSavingsService(accRepo, txRepo, sRepo)
		_, err := svc.GenerateSavingsReport(ctx, "2026-13")
		requireField(t, err, "month")
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		txRepo.splitsRangeErr = errors.New("boom")
		svc := newTestSavingsService(accRepo, txRepo, sRepo)
		_, err := svc.GenerateSavingsReport(ctx, "2026-01")
		assert.ErrorContains(t, err, "boom")
	})
}
