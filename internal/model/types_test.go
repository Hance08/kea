// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpeningBalancesAccountName(t *testing.T) {
	assert.Equal(t, "Equity:OpeningBalances_USD", OpeningBalancesAccountName("USD"))
	assert.Equal(t, "Equity:OpeningBalances_TWD", OpeningBalancesAccountName("TWD"))
	assert.Equal(t, "Equity:OpeningBalances_TWD", OpeningBalancesAccountName("twd"))
}

func TestIsOpeningBalancesAccount(t *testing.T) {
	assert.True(t, IsOpeningBalancesAccount("Equity:OpeningBalances_USD"))
	assert.True(t, IsOpeningBalancesAccount("Equity:OpeningBalances_TWD"))
	assert.False(t, IsOpeningBalancesAccount("Equity:OpeningBalances"))
	assert.False(t, IsOpeningBalancesAccount("Equity:Retained"))
	assert.False(t, IsOpeningBalancesAccount(""))
}

func TestParseTransactionType(t *testing.T) {
	tests := []struct {
		input   string
		want    TransactionType
		wantErr bool
	}{
		{"expense", TxTypeExpense, false},
		{"Expense", TxTypeExpense, false},
		{"EXPENSE", TxTypeExpense, false},
		{"  expense  ", TxTypeExpense, false},
		{"income", TxTypeIncome, false},
		{"Income", TxTypeIncome, false},
		{"transfer", TxTypeTransfer, false},
		{"Transfer", TxTypeTransfer, false},
		{"opening", TxTypeOpening, false},
		{"Opening", TxTypeOpening, false},
		{"deposit", TxTypeDeposit, false},
		{"withdrawal", TxTypeWithdrawal, false},
		{"  Other  ", TxTypeOther, false},
		{"unknown", "", true},
		{"", "", true},
		{"garbage", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTransactionType(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseTransactionTypeLabel(t *testing.T) {
	tests := []struct {
		input string
		want  TransactionType
	}{
		{"Expense (pay a bill...)", TxTypeExpense},
		{"expense", TxTypeExpense},
		{"EXPENSE", TxTypeExpense},
		{"Income (receive payment...)", TxTypeIncome},
		{"income", TxTypeIncome},
		{"Transfer (move between accounts)", TxTypeTransfer},
		{"transfer", TxTypeTransfer},
		{"something else", TxTypeTransfer},
		{"", TxTypeTransfer},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseTransactionTypeLabel(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseTransactionStatus(t *testing.T) {
	tests := []struct {
		input   string
		want    TransactionStatus
		wantErr bool
	}{
		{"pending", StatusPending, false},
		{"Pending", StatusPending, false},
		{"PENDING", StatusPending, false},
		{"cleared", StatusCleared, false},
		{"Cleared", StatusCleared, false},
		{"reconciled", StatusReconciled, false},
		{"Reconciled", StatusReconciled, false},
		{"", 0, true},
		{"anything", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTransactionStatus(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestAccountTypeFromRootName(t *testing.T) {
	tests := []struct {
		root     string
		wantType AccountType
		wantOK   bool
	}{
		{"Assets", AccountTypeAsset, true},
		{"assets", AccountTypeAsset, true},
		{"ASSETS", AccountTypeAsset, true},
		{"Liabilities", AccountTypeLiability, true},
		{"liabilities", AccountTypeLiability, true},
		{"Equity", AccountTypeEquity, true},
		{"Revenue", AccountTypeRevenue, true},
		{"Expenses", AccountTypeExpense, true},
		{"expenses", AccountTypeExpense, true},
		{"Unknown", "", false},
		{"", "", false},
		{"Asset", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.root, func(t *testing.T) {
			gotType, gotOK := AccountTypeFromRootName(tt.root)
			assert.Equal(t, tt.wantType, gotType)
			assert.Equal(t, tt.wantOK, gotOK)
		})
	}
}

func TestAccountTypeFromRootName_RoundTrips(t *testing.T) {
	allTypes := []AccountType{
		AccountTypeAsset, AccountTypeLiability, AccountTypeEquity,
		AccountTypeRevenue, AccountTypeExpense,
	}
	for _, at := range allTypes {
		rootName, ok := at.RootName()
		require.True(t, ok, "RootName() should succeed for %s", at)

		gotType, gotOK := AccountTypeFromRootName(rootName)
		assert.True(t, gotOK)
		assert.Equal(t, at, gotType, "round-trip failed for %s", at)
	}
}

func TestTransactionType_IsValid(t *testing.T) {
	valid := []TransactionType{
		TxTypeExpense, TxTypeIncome, TxTypeTransfer,
		TxTypeOpening, TxTypeDeposit, TxTypeWithdrawal, TxTypeOther,
	}
	for _, v := range valid {
		assert.True(t, v.IsValid(), "expected %q to be valid", v)
	}

	invalid := []TransactionType{"", "garbage", "expense", "EXPENSE", "Unknown"}
	for _, v := range invalid {
		assert.False(t, v.IsValid(), "expected %q to be invalid", v)
	}
}

func TestTransactionType_MarshalJSON(t *testing.T) {
	t.Run("valid round trip", func(t *testing.T) {
		all := []TransactionType{
			TxTypeExpense, TxTypeIncome, TxTypeTransfer,
			TxTypeOpening, TxTypeDeposit, TxTypeWithdrawal, TxTypeOther,
		}
		for _, want := range all {
			data, err := json.Marshal(want)
			require.NoError(t, err)
			var got TransactionType
			require.NoError(t, json.Unmarshal(data, &got))
			assert.Equal(t, want, got)
		}
	})

	t.Run("invalid value fails to marshal", func(t *testing.T) {
		_, err := json.Marshal(TransactionType("garbage"))
		assert.Error(t, err)
	})
}

func TestTransactionType_UnmarshalJSON(t *testing.T) {
	t.Run("rejects unknown string", func(t *testing.T) {
		var got TransactionType
		err := json.Unmarshal([]byte(`"garbage"`), &got)
		assert.Error(t, err)
	})

	t.Run("rejects empty string", func(t *testing.T) {
		var got TransactionType
		err := json.Unmarshal([]byte(`""`), &got)
		assert.Error(t, err)
	})

	t.Run("rejects non-string", func(t *testing.T) {
		var got TransactionType
		err := json.Unmarshal([]byte(`123`), &got)
		assert.Error(t, err)
	})

	t.Run("rejects wrong case", func(t *testing.T) {
		var got TransactionType
		err := json.Unmarshal([]byte(`"expense"`), &got)
		assert.Error(t, err)
	})
}

func TestDescriptionMaxLength(t *testing.T) {
	assert.Equal(t, 500, DescriptionMaxLength)
}

func TestMemoMaxLength(t *testing.T) {
	assert.Equal(t, 200, MemoMaxLength)
}
