package cmd

import (
	"github.com/hance08/kea/internal/model"
)

type AddView interface {
	Render(input *model.TransactionDetail, isCreate bool) error
}

type AddProvider interface {
	GetAllAccounts() ([]*model.Account, error)
	GetAccountBalanceFormatted(accountID int64) (string, error)
	GetAccountByName(name string) (*model.Account, error)
	ValidateSelectableAccount(name string, allowedTypes []string) error
}

type TransactionProvider interface {
	GetTransactionRule(mode model.TransactionType) (model.TransactionRule, error)
	CreateSimpleTransaction(fromAccount string, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus) (model.TransactionDetail, error)
}

var modeUIConfigs = map[model.TransactionType]struct{ Src, Dst string }{
	model.ModeExpense:  {"Payment Source:", "Expense Type:"},
	model.ModeIncome:   {"Revenue Type:", "Deposit To:"},
	model.ModeTransfer: {"From Account:", "To Account:"},
}

type addFlags struct {
	Description string
	Amount      string
	From        string
	To          string
	Status      string
	Timestamp   string
	Type        string
}

type addTransactionInput struct {
	FromAccountID string
	ToAccountID   string
	AmountCents   int64
	Description   string
	Timestamp     int64
	Status        model.TransactionStatus
}
