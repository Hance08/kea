// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type CreateAccountInput struct {
	Name        string
	Type        AccountType
	Currency    string
	Description string
	ParentID    *int64
	Balance     int64
}

type CreateSimpleTransactionInput struct {
	FromAccount string
	ToAccount   string
	Amount      int64
	Description string
	Timestamp   int64
	Status      TransactionStatus
	Type        TransactionType
}

type CreateTransactionFromSplitsInput struct {
	Splits      []SplitDetail
	Description string
	Timestamp   int64
	Status      TransactionStatus
	Type        TransactionType
}

type UpdateTransactionInput struct {
	ID          int64
	Description string
	Timestamp   int64
	Status      TransactionStatus
	Type        TransactionType
	Splits      []SplitDetail
}
