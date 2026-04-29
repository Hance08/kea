// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type Account struct {
	ID          int64
	Name        string
	Type        AccountType
	ParentID    *int64
	Currency    string
	Description string
	IsHidden    bool
}
