package model

type Transaction struct {
	ID          int64
	Timestamp   int64
	Description string
	Status      TransactionStatus
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
