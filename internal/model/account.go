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
