package cmd

import (
	"github.com/hance08/kea/internal/constant"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
)

type AddView interface {
	Render(input *service.TransactionDetail, isCreate bool) error
}

type AddProvider interface {
	GetAllAccounts() ([]*model.Account, error)
	GetAccountBalanceFormatted(accountID int64) (string, error)
}

type TransactionProvider interface {
	GetTransactionRule(mode string) (service.TransactionRule, error)
	CreateSimpleTransaction(fromAccount string, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus) (service.TransactionDetail, error)
}

var modeUIConfigs = map[string]struct{ Src, Dst string }{
	constant.ModeExpense:  {"Payment Source:", "Expense Type:"},
	constant.ModeIncome:   {"Revenue Type:", "Deposit To:"},
	constant.ModeTransfer: {"From Account:", "To Account:"},
}

type addFlags struct {
	Description string
	Amount      string
	From        string
	To          string
	Status      string
	Timestamp   string
}

type addTransactionInput struct {
	FromAccountID string
	ToAccountID   string
	AmountCents   int64
	Description   string
	Timestamp     int64
	Status        model.TransactionStatus
}
