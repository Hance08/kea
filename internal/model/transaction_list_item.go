// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type TransactionListItem struct {
	ID            int64
	Date          string
	Type          string
	Account       string
	OffsetAccount string
	Description   string
	Amount        int64
	Currency      string
	Status        string
}
