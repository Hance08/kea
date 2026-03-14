package service

import "github.com/hance08/kea/internal/model"

type AccountRepository interface {
	CreateAccount(name string, accType model.AccountType, currency, description string, parentID *int64) (int64, error)
	GetAllAccounts() ([]*model.Account, error)
	GetAccountByName(name string) (*model.Account, error)
	GetAccountByID(id int64) (*model.Account, error)
	AccountExists(name string) (bool, error)
	GetAccountsByType(accType model.AccountType) ([]*model.Account, error)
	GetAccountBalance(accountID int64) (int64, error)
	HasChildAccounts(accountID int64) (bool, error)
	AccountHasTransactions(accountID int64) (bool, error)
	DeleteAccount(accountID int64) error
}

type TransactionRepository interface {
	CreateTransactionWithSplits(tx model.Transaction, splits []model.Split) (int64, error)
	GetTransactionByID(txID int64) (*model.Transaction, []*model.Split, error)
	GetTransactionsByAccount(accountID int64, limit int) ([]*model.Transaction, error)
	GetTransactionsByDateRange(startTime, endTime int64) ([]*model.Transaction, error)
	GetAllTransactions(limit int) ([]*model.Transaction, error)

	UpdateTransactionStatus(txID int64, status model.TransactionStatus) error
	DeleteTransaction(txID int64) error
	UpdateTransactionBasic(txID int64, description string, timestamp int64, status model.TransactionStatus) error

	CreateSplit(txID int64, split *model.Split) (int64, error)
	UpdateSplit(splitID int64, accountID int64, amount int64, currency string, memo string) error
	DeleteSplit(splitID int64) error
	GetSplitsByTransaction(txID int64) ([]*model.Split, error)
}

type Repository interface {
	AccountRepository
	TransactionRepository
}

type TransactionManager interface {
	ExecTx(fn func(Repository) error) error
}
