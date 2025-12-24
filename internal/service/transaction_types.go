package service

import (
	"errors"
)

type TransactionType string
type TransactionRule struct {
	Mode         string // e.g., "expense", "income", "transfer"
	SourceTypes  []string
	DestTypes    []string
	SourcePrompt string
	DestPrompt   string
}

const (
	TxTypeExpense    TransactionType = "Expense"
	TxTypeIncome     TransactionType = "Income"
	TxTypeTransfer   TransactionType = "Transfer"
	TxTypeOpening    TransactionType = "Opening"
	TxTypeDeposit    TransactionType = "Deposit"
	TxTypeWithdrawal TransactionType = "Withdrawal"
	TxTypeOther      TransactionType = "Other"
)

// TransactionSplitInput represents a split entry with account name instead of ID
type TransactionSplitInput struct {
	ID          int64
	AccountName string
	AccountID   int64
	Amount      int64
	Currency    string
	Memo        string
}

// TransactionInput represents user input for creating a transaction
type TransactionInput struct {
	Timestamp   int64
	Description string
	Splits      []TransactionSplitInput
	Status      int
}

// TransactionDetail represents a transaction with full split details
type TransactionDetail struct {
	ID          int64
	Timestamp   int64
	Description string
	Status      int
	Splits      []SplitDetail
}

type SplitDetail struct {
	ID          int64
	AccountID   int64
	AccountName string
	Amount      int64
	Currency    string
	Memo        string
}

func (d *TransactionDetail) ToSplitInputs() []TransactionSplitInput {
	var inputs []TransactionSplitInput
	for _, split := range d.Splits {
		inputs = append(inputs, TransactionSplitInput{
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
