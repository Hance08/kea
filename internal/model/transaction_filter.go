// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type TransactionFilter struct {
	AccountID   *int64
	Type        *TransactionType
	Status      *TransactionStatus
	StartTime   *int64
	EndTime     *int64
	Description *string
}
