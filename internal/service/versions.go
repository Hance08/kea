// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import "github.com/hance08/kea/internal/model"

// versionKey is what version selection needs from a per-account row that
// applies from Month until a later row for the same account (budgets,
// savings targets).
type versionKey struct {
	ID        int64
	AccountID int64
	Month     string
	Stopped   bool
}

func budgetVersionKey(b model.Budget) versionKey {
	return versionKey{ID: b.ID, AccountID: b.AccountID, Month: b.EffectiveMonth, Stopped: b.Stopped}
}

func savingsVersionKey(t model.SavingsTarget) versionKey {
	return versionKey{ID: t.ID, AccountID: t.AccountID, Month: t.EffectiveMonth, Stopped: t.Stopped}
}

// latestVersions returns, per account, the version with the greatest
// Month <= month (including stopped versions).
func latestVersions[T any](items []T, month string, key func(T) versionKey) map[int64]T {
	latest := map[int64]T{}
	latestMonth := map[int64]string{}
	for _, it := range items {
		k := key(it)
		if k.Month > month {
			continue
		}
		if cur, ok := latestMonth[k.AccountID]; !ok || k.Month > cur {
			latest[k.AccountID] = it
			latestMonth[k.AccountID] = k.Month
		}
	}
	return latest
}

// activeVersions returns the non-stopped version that applies to month for
// each account, in the input order. The result is never nil.
func activeVersions[T any](items []T, month string, key func(T) versionKey) []T {
	latest := latestVersions(items, month, key)
	out := []T{}
	for _, it := range items {
		k := key(it)
		if v, ok := latest[k.AccountID]; ok && key(v).ID == k.ID && !k.Stopped {
			out = append(out, it)
		}
	}
	return out
}

// activeVersionFor returns the non-stopped version that applies to month for
// accountID, if any.
func activeVersionFor[T any](items []T, accountID int64, month string, key func(T) versionKey) (T, bool) {
	v, ok := latestVersions(items, month, key)[accountID]
	if !ok || key(v).Stopped {
		var zero T
		return zero, false
	}
	return v, true
}

// hasAncestorInCurrency reports whether rows holds a strict ancestor of row's
// account in the same currency. Only then is row's amount already inside that
// ancestor's; an ancestor in a different currency excludes row's account, so
// row must still count in its own currency total.
func hasAncestorInCurrency[T any](row T, rows []T, nameCcy func(T) (name, currency string)) bool {
	name, ccy := nameCcy(row)
	for _, r := range rows {
		n, c := nameCcy(r)
		if c == ccy && n != name && isSelfOrDescendant(name, n) {
			return true
		}
	}
	return false
}
