package model

type Account struct {
	ID          int64
	Name        string
	Type        AccountType
	ParentID    *int64
	Currency    string
	Description string
	IsHidden    bool
}
