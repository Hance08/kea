// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type Account struct {
	ID          int64       `json:"id"`
	Name        string      `json:"name"`
	Type        AccountType `json:"type"`
	ParentID    *int64      `json:"parent_id,omitempty"`
	Currency    string      `json:"currency"`
	Description string      `json:"description"`
	IsHidden    bool        `json:"is_hidden"`
}

type AccountNode struct {
	Account  *Account       `json:"account"`
	Children []*AccountNode `json:"children"`
}

// AccountBalance is one row in a bulk-balance snapshot response.
// Joins per-account metadata (from Account) with a point-in-time balance.
type AccountBalance struct {
	AccountID int64       `json:"account_id"`
	Name      string      `json:"name"`
	Type      AccountType `json:"type"`
	ParentID  *int64      `json:"parent_id,omitempty"`
	Currency  string      `json:"currency"`
	Amount    int64       `json:"amount"`
	IsHidden  bool        `json:"is_hidden"`
}
