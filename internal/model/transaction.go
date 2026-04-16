package model

import "errors"

type Transaction struct {
	ID          int64
	Timestamp   int64
	Description string
	Status      TransactionStatus
	ExternalID  *string
}

type TransactionDetail struct {
	ID          int64
	Timestamp   int64
	Description string
	Status      TransactionStatus
	Splits      []SplitDetail
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
	ID            int64
	TransactionID int64
	AccountID     int64
	Amount        int64
	Currency      string
	Memo          string
}

type SplitDetail struct {
	ID          int64
	AccountID   int64
	AccountName string
	AccountType AccountType
	Amount      int64
	Currency    string
	Memo        string
}

// ReconcileEntry is a read-only projection used by the reconciliation workflow.
// It carries a transaction plus the net split amount for the queried account,
// so the TUI can display amounts and the service can compute the cleared balance
// without issuing per-transaction queries.
type ReconcileEntry struct {
	ID          int64
	Timestamp   int64
	Description string
	Status      TransactionStatus
	Amount      int64 // net split amount for the reconciled account
}
