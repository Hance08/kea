// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type AccountFilter struct {
	Query    *string
	Type     *AccountType
	Currency *string
}
