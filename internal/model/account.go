package model

type Account struct {
	ID          int64
	Name        string
	Type        string
	ParentID    *int64
	Currency    string
	Description string
	IsHidden    bool
}
