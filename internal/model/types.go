// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

func (s TransactionStatus) MarshalJSON() ([]byte, error) {
	str := s.String()
	if str == "Unknown" {
		return nil, fmt.Errorf("cannot marshal invalid transaction status %d", int(s))
	}
	return json.Marshal(str)
}

func (s *TransactionStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		var num int
		if err2 := json.Unmarshal(data, &num); err2 != nil {
			return err
		}
		ts := TransactionStatus(num)
		if ts != StatusPending && ts != StatusCleared && ts != StatusReconciled {
			return fmt.Errorf("unknown transaction status %d", num)
		}
		*s = ts
		return nil
	}
	switch str {
	case "Pending":
		*s = StatusPending
	case "Cleared":
		*s = StatusCleared
	case "Reconciled":
		*s = StatusReconciled
	default:
		return fmt.Errorf("unknown transaction status %q", str)
	}
	return nil
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
	DateFormat                = "2006-01-02"
	SystemTransactionID int64 = 1
	MinSplitsCount            = 2

	LegacyOpeningBalancesName = "Equity:OpeningBalances"
)

func OpeningBalancesAccountName(currency string) string {
	return "Equity:OpeningBalances_" + strings.ToUpper(currency)
}

func IsOpeningBalancesAccount(name string) bool {
	return strings.HasPrefix(name, "Equity:OpeningBalances_")
}

func ParseTransactionType(s string) (TransactionType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "expense":
		return TxTypeExpense, nil
	case "income":
		return TxTypeIncome, nil
	case "transfer":
		return TxTypeTransfer, nil
	default:
		return "", fmt.Errorf("invalid transaction type %q: must be expense, income, or transfer", s)
	}
}

func ParseTransactionTypeLabel(s string) TransactionType {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "expense") {
		return TxTypeExpense
	}
	if strings.Contains(lower, "income") {
		return TxTypeIncome
	}
	return TxTypeTransfer
}

func ParseTransactionStatus(s string) TransactionStatus {
	if strings.ToLower(s) == "pending" {
		return StatusPending
	}
	return StatusCleared
}
