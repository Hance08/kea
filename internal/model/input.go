// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

type CreateAccountInput struct {
	Name        string      `json:"name"`
	Type        AccountType `json:"type"`
	Currency    string      `json:"currency"`
	Description string      `json:"description"`
	ParentID    *int64      `json:"parent_id,omitempty"`
	Balance     int64       `json:"balance"`
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
	Splits      []SplitDetail     `json:"splits"`
	Description string            `json:"description"`
	Timestamp   int64             `json:"timestamp"`
	Status      TransactionStatus `json:"status"`
	Type        TransactionType   `json:"type"`
}

type UpdateTransactionInput struct {
	ID          int64             `json:"-"`
	Description string            `json:"description"`
	Timestamp   int64             `json:"timestamp"`
	Status      TransactionStatus `json:"status"`
	Type        TransactionType   `json:"type"`
	Splits      []SplitDetail     `json:"splits"`
}
