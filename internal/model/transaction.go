// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

import "errors"

type Transaction struct {
	ID          int64             `json:"id"`
	Timestamp   int64             `json:"timestamp"`
	Description string            `json:"description"`
	Status      TransactionStatus `json:"status"`
	Type        TransactionType   `json:"type"`
	ExternalID  *string           `json:"external_id,omitempty"`
}

type TransactionDetail struct {
	ID          int64             `json:"id"`
	Timestamp   int64             `json:"timestamp"`
	Description string            `json:"description"`
	Status      TransactionStatus `json:"status"`
	Type        TransactionType   `json:"type"`
	Splits      []SplitDetail     `json:"splits"`
}

type TransactionRule struct {
	TxType       TransactionType
	SourceTypes  []string
	DestTypes    []string
	SourcePrompt string
	DestPrompt   string
}

func (d *TransactionDetail) ToSplitInputs() []SplitDetail {
	var inputs []SplitDetail
	for _, split := range d.Splits {
		inputs = append(inputs, SplitDetail{
			ID:          split.ID,
			AccountName: split.AccountName,
			AccountID:   split.AccountID,
			Amount:      split.Amount,
			Currency:    split.Currency,
			Memo:        split.Memo,
		})
	}
	return inputs
}

func (d *TransactionDetail) UpdateAmountPreservingBalance(newAbsAmount int64) error {
	if len(d.Splits) != 2 {
		return errors.New("auto-balance only supports 2 splits")
	}
	if d.Splits[0].Amount >= 0 {
		d.Splits[0].Amount = newAbsAmount
		d.Splits[1].Amount = -newAbsAmount
	} else {
		d.Splits[0].Amount = -newAbsAmount
		d.Splits[1].Amount = newAbsAmount
	}
	return nil
}

type Split struct {
	ID            int64  `json:"id"`
	TransactionID int64  `json:"transaction_id"`
	AccountID     int64  `json:"account_id"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Memo          string `json:"memo"`
}

type SplitDetail struct {
	ID          int64       `json:"id"`
	AccountID   int64       `json:"account_id"`
	AccountName string      `json:"account_name"`
	AccountType AccountType `json:"account_type"`
	Amount      int64       `json:"amount"`
	Currency    string      `json:"currency"`
	Memo        string      `json:"memo"`
}

// ReconcileEntry is a read-only projection used by the reconciliation workflow.
// It carries a transaction plus the net split amount for the queried account,
// so the TUI can display amounts and the service can compute the cleared balance
// without issuing per-transaction queries.
type ReconcileEntry struct {
	ID            int64
	Timestamp     int64
	Description   string
	Status        TransactionStatus
	Amount        int64  // net split amount for the reconciled account
	OffsetAccount string // other-side account name; "(split)" for multi-split transactions
}
