package store

import "github.com/hance08/kea/internal/model"

type AccountRepository interface {
	CreateAccount(name, accType, currency, description string, parentID *int64) (int64, error)
	GetAllAccounts() ([]*model.Account, error)
	GetAccountByName(name string) (*model.Account, error)
	GetAccountByID(id int64) (*model.Account, error)
	AccountExists(name string) (bool, error)
	GetAccountsByType(accType string) ([]*model.Account, error)
	GetAccountBalance(accountID int64) (int64, error)
}

type TransactionRepository interface {
	CreateTransactionWithSplits(tx model.Transaction, splits []model.Split) (int64, error)
	GetTransactionByID(txID int64) (*model.Transaction, []*model.Split, error)
	GetTransactionsByAccount(accountID int64, limit int) ([]*model.Transaction, error)
	GetTransactionsByDateRange(startTime, endTime int64) ([]*model.Transaction, error)
	GetAllTransactions(limit int) ([]*model.Transaction, error)

	UpdateTransactionStatus(txID int64, status int) error
	DeleteTransaction(txID int64) error
	UpdateTransactionBasic(txID int64, description string, timestamp int64, status int) error

	CreateSplit(txID int64, split *model.Split) (int64, error)
	UpdateSplit(splitID int64, accountID int64, amount int64, currency string, memo string) error
	DeleteSplit(splitID int64) error
	GetSplitsByTransaction(txID int64) ([]*model.Split, error)
}
type Repository interface {
	AccountRepository
	TransactionRepository

	ExecTx(fn func(Repository) error) error
	Close() error
}
