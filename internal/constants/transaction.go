package constants

const (
	// Transaction Modes
	ModeExpense  = "expense"
	ModeIncome   = "income"
	ModeTransfer = "transfer"

	// Status
	StatusPending = 0
	StatusCleared = 1

	// Date Layout
	DateFormat = "2006-01-02"

	OpeningBalanceTransactionID int64 = 1

	MinSplitsCount = 2
)
