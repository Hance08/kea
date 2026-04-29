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
