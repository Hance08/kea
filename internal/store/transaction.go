package store

const (
	StatusPending    = 0
	StatusCleared    = 1
	StatusReconciled = 2
)

type Transaction struct {
	ID          int64
	Timestamp   int64
	Description string
	Status      int
	ExternalID  *string
}

type Split struct {
	ID            int64
	TransactionID int64
	AccountID     int64
	Amount        int64
	Currency      string
	Memo          string
}
