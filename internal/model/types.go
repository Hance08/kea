package model

type AccountType string

const (
	AccountTypeAsset     AccountType = "A"
	AccountTypeLiability AccountType = "L"
	AccountTypeEquity    AccountType = "C"
	AccountTypeRevenue   AccountType = "R"
	AccountTypeExpense   AccountType = "E"
)

func (at AccountType) String() string {
	return string(at)
}

func (at AccountType) IsValid() bool {
	switch at {
	case AccountTypeAsset, AccountTypeLiability, AccountTypeEquity, AccountTypeRevenue, AccountTypeExpense:
		return true
	}
	return false
}

type TransactionStatus int

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
