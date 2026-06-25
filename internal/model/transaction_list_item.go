// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// TransactionListItem is a display-ready projection of a transaction for list views.
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
	Regular       *bool
}
