// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
)

func targets() []model.SavingsTarget {
	return []model.SavingsTarget{
		{ID: 1, AccountID: 10, EffectiveMonth: "2026-01", Amount: 100},
		{ID: 2, AccountID: 10, EffectiveMonth: "2026-03", Stopped: true},
		{ID: 3, AccountID: 10, EffectiveMonth: "2026-05", Amount: 300},
		{ID: 4, AccountID: 20, EffectiveMonth: "2026-04", Amount: 50},
	}
}

func ids(ts []model.SavingsTarget) []int64 {
	out := make([]int64, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

func TestActiveVersions(t *testing.T) {
	all := targets()
	assert.Equal(t, []int64{}, ids(activeVersions([]model.SavingsTarget{}, "2026-01", savingsVersionKey)))
	assert.NotNil(t, activeVersions([]model.SavingsTarget{}, "2026-01", savingsVersionKey))
	assert.Equal(t, []int64{}, ids(activeVersions(all, "2025-12", savingsVersionKey)), "before first version")
	assert.Equal(t, []int64{1}, ids(activeVersions(all, "2026-02", savingsVersionKey)))
	assert.Equal(t, []int64{}, ids(activeVersions(all, "2026-03", savingsVersionKey)), "stopped")
	assert.Equal(t, []int64{4}, ids(activeVersions(all, "2026-04", savingsVersionKey)), "account 10 still stopped")
	assert.Equal(t, []int64{3, 4}, ids(activeVersions(all, "2026-09", savingsVersionKey)), "restarted")
}

func TestActiveVersionFor(t *testing.T) {
	all := targets()
	v, ok := activeVersionFor(all, 10, "2026-02", savingsVersionKey)
	assert.True(t, ok)
	assert.Equal(t, int64(100), v.Amount)
	_, ok = activeVersionFor(all, 10, "2026-04", savingsVersionKey)
	assert.False(t, ok, "stopped month")
	_, ok = activeVersionFor(all, 20, "2026-03", savingsVersionKey)
	assert.False(t, ok, "before first version")
	_, ok = activeVersionFor(all, 99, "2026-09", savingsVersionKey)
	assert.False(t, ok, "unknown account")
}

func TestHasAncestorInCurrency(t *testing.T) {
	rows := []model.SavingsReportRow{
		{AccountName: "Assets:Savings", Currency: "USD"},
		{AccountName: "Assets:Savings:Travel", Currency: "USD"},
		{AccountName: "Assets:Savings:Japan", Currency: "JPY"},
		{AccountName: "Assets:SavingsBox", Currency: "USD"},
	}
	nc := func(r model.SavingsReportRow) (string, string) { return r.AccountName, r.Currency }
	assert.False(t, hasAncestorInCurrency(rows[0], rows, nc))
	assert.True(t, hasAncestorInCurrency(rows[1], rows, nc))
	assert.False(t, hasAncestorInCurrency(rows[2], rows, nc), "ancestor in another currency")
	assert.False(t, hasAncestorInCurrency(rows[3], rows, nc), "name prefix is not an ancestor")
}
