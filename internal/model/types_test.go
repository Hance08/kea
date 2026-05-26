// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

import (
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
		{"unknown", "", true},
		{"", "", true},
		{"opening", "", true},
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

func TestDescriptionMaxLength(t *testing.T) {
	assert.Equal(t, 500, DescriptionMaxLength)
}

func TestMemoMaxLength(t *testing.T) {
	assert.Equal(t, 200, MemoMaxLength)
}
