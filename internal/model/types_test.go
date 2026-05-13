// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestParseTransactionStatus(t *testing.T) {
	tests := []struct {
		input string
		want  TransactionStatus
	}{
		{"pending", StatusPending},
		{"Pending", StatusPending},
		{"PENDING", StatusPending},
		{"cleared", StatusCleared},
		{"Cleared", StatusCleared},
		{"", StatusCleared},
		{"anything", StatusCleared},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseTransactionStatus(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
