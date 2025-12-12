package store

type Account struct {
	ID          int64
	Name        string
	Type        string
	ParentID    *int64
	Currency    string
	Description string
	IsHidden    bool
}

// Error implements error.
func (a *Account) Error() string {
	panic("unimplemented")
}

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
