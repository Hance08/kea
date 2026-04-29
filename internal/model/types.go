// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

import "strings"

const (
	CentsPerUnit = 100
)

type AccountType string

const (
	AccountTypeAsset     AccountType = "A"
	AccountTypeLiability AccountType = "L"
	AccountTypeEquity    AccountType = "C"
	AccountTypeRevenue   AccountType = "R"
	AccountTypeExpense   AccountType = "E"
)

const (
	AccountNameMaxLength = 100
	MaxSafeBalanceFloat  = 9223372036854775.0
	OpeningAccountMemo   = "Opening Balance"
	TypeEquity           = "C"
)

var ReservedNames = map[string]bool{
	"assets":      true,
	"liabilities": true,
	"equity":      true,
	"revenue":     true,
	"expenses":    true,
}

func (at AccountType) String() string {
	return string(at)
}

func (at AccountType) RootName() (string, bool) {
	switch at {
	case AccountTypeAsset:
		return "Assets", true
	case AccountTypeLiability:
		return "Liabilities", true
	case AccountTypeExpense:
		return "Expenses", true
	case AccountTypeRevenue:
		return "Revenue", true
	case AccountTypeEquity:
		return "Equity", true
	}
	return "", false
}

func (at AccountType) IsValid() bool {
	switch at {
	case AccountTypeAsset, AccountTypeLiability, AccountTypeEquity, AccountTypeRevenue, AccountTypeExpense:
		return true
	}
	return false
}

type TransactionStatus int
type TransactionType string

const (
	StatusPending    TransactionStatus = 0
	StatusCleared    TransactionStatus = 1
	StatusReconciled TransactionStatus = 2
)

func (s TransactionStatus) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusCleared:
		return "Cleared"
	case StatusReconciled:
		return "Reconciled"
	default:
		return "Unknown"
	}
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

const (
	ModeExpense  = "Expense"
	ModeIncome   = "Income"
	ModeTransfer = "Transfer"

	DateFormat                        = "2006-01-02"
	SystemTransactionID int64 = 1
	MinSplitsCount                    = 2
)

func OpeningBalancesAccountName(currency string) string {
	return "Equity:OpeningBalances_" + strings.ToUpper(currency)
}

func IsOpeningBalancesAccount(name string) bool {
	return strings.HasPrefix(name, "Equity:OpeningBalances_")
}
