// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model_test

import (
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// UpdateAmountPreservingBalance
// ──────────────────────────────────────────────

func TestUpdateAmountPreservingBalance(t *testing.T) {
	makeDetail := func(a, b int64) *model.TransactionDetail {
		return &model.TransactionDetail{
			Splits: []model.SplitDetail{
				{Amount: a, Currency: "USD"},
				{Amount: b, Currency: "USD"},
			},
		}
	}

	t.Run("positive first split: splits[0]=+new, splits[1]=-new", func(t *testing.T) {
		d := makeDetail(1000, -1000)
		err := d.UpdateAmountPreservingBalance(2000)
		require.NoError(t, err)
		assert.Equal(t, int64(2000), d.Splits[0].Amount)
		assert.Equal(t, int64(-2000), d.Splits[1].Amount)
	})

	t.Run("negative first split: splits[0]=-new, splits[1]=+new", func(t *testing.T) {
		d := makeDetail(-1000, 1000)
		err := d.UpdateAmountPreservingBalance(2000)
		require.NoError(t, err)
		assert.Equal(t, int64(-2000), d.Splits[0].Amount)
		assert.Equal(t, int64(2000), d.Splits[1].Amount)
	})

	t.Run("splits still sum to zero after update", func(t *testing.T) {
		d := makeDetail(500, -500)
		require.NoError(t, d.UpdateAmountPreservingBalance(999))
		assert.Equal(t, int64(0), d.Splits[0].Amount+d.Splits[1].Amount)
	})

	t.Run("zero first split (>= 0 condition) is treated as positive direction", func(t *testing.T) {
		d := makeDetail(0, 0)
		err := d.UpdateAmountPreservingBalance(300)
		require.NoError(t, err)
		assert.Equal(t, int64(300), d.Splits[0].Amount)
		assert.Equal(t, int64(-300), d.Splits[1].Amount)
	})

	t.Run("more than 2 splits returns error", func(t *testing.T) {
		d := &model.TransactionDetail{
			Splits: []model.SplitDetail{
				{Amount: 400},
				{Amount: 300},
				{Amount: -700},
			},
		}
		err := d.UpdateAmountPreservingBalance(500)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "2 splits")
	})

	t.Run("empty splits returns error", func(t *testing.T) {
		d := &model.TransactionDetail{}
		err := d.UpdateAmountPreservingBalance(500)
		require.Error(t, err)
	})

	t.Run("single split returns error", func(t *testing.T) {
		d := &model.TransactionDetail{
			Splits: []model.SplitDetail{{Amount: 100}},
		}
		err := d.UpdateAmountPreservingBalance(200)
		require.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// AccountType
// ──────────────────────────────────────────────

func TestAccountType_IsValid(t *testing.T) {
	valid := []model.AccountType{
		model.AccountTypeAsset,
		model.AccountTypeLiability,
		model.AccountTypeEquity,
		model.AccountTypeRevenue,
		model.AccountTypeExpense,
	}
	for _, at := range valid {
		assert.True(t, at.IsValid(), "AccountType %q should be valid", at)
	}

	invalid := []model.AccountType{"", "X", "a", "AA", " "}
	for _, at := range invalid {
		assert.False(t, at.IsValid(), "AccountType %q should be invalid", at)
	}
}

func TestAccountType_RootName(t *testing.T) {
	tests := []struct {
		accType  model.AccountType
		wantName string
		wantOK   bool
	}{
		{model.AccountTypeAsset, "Assets", true},
		{model.AccountTypeLiability, "Liabilities", true},
		{model.AccountTypeEquity, "Equity", true},
		{model.AccountTypeRevenue, "Revenue", true},
		{model.AccountTypeExpense, "Expenses", true},
		{"X", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.accType), func(t *testing.T) {
			name, ok := tt.accType.RootName()
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantName, name)
		})
	}
}

// ──────────────────────────────────────────────
// TransactionStatus
// ──────────────────────────────────────────────

func TestTransactionStatus_String(t *testing.T) {
	tests := []struct {
		status model.TransactionStatus
		want   string
	}{
		{model.StatusPending, "Pending"},
		{model.StatusCleared, "Cleared"},
		{model.StatusReconciled, "Reconciled"},
		{model.TransactionStatus(99), "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.String())
		})
	}
}
