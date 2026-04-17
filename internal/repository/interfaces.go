package repository

import (
	"context"

	"github.com/hance08/kea/internal/model"
)

type AccountRepository interface {
	CreateAccount(name string, accType model.AccountType, currency, description string, parentID *int64) (int64, error)
	GetAllAccounts() ([]*model.Account, error)
	GetAccountByName(name string) (*model.Account, error)
	GetAccountByID(id int64) (*model.Account, error)
	AccountExists(name string) (bool, error)
	GetAccountsByType(accType model.AccountType) ([]*model.Account, error)
	GetAccountBalance(accountID int64) (int64, error)
	GetAllAccountBalances(asOf int64) (map[int64]int64, error)
	HasChildAccounts(accountID int64) (bool, error)
	AccountHasTransactions(accountID int64) (bool, error)
	DeleteAccount(accountID int64) error
	RenameAccount(oldName, newName string) error
}

type TransactionRepository interface {
	CreateTransactionWithSplits(tx model.Transaction, splits []model.Split) (int64, error)
	GetTransactionByID(txID int64) (*model.Transaction, error)
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

	// GetSplitsWithAccountsByDateRange returns all splits (with account info) for
	// transactions in the given Unix time range, keyed by transaction ID.
	// Designed for bulk report queries to avoid N+1 patterns.
	GetSplitsWithAccountsByDateRange(startTime, endTime int64) (map[int64][]model.SplitDetail, error)

	// GetSplitsWithAccountsByTransaction returns all splits (with account info) for
	// a single transaction in one JOIN query.
	GetSplitsWithAccountsByTransaction(txID int64) ([]model.SplitDetail, error)

	// GetUnreconciledTransactionsByAccount returns all Pending and Cleared
	// transactions that have a split touching accountID, together with the
	// net split amount for that account. Ordered by timestamp ASC.
	GetUnreconciledTransactionsByAccount(accountID int64) ([]*model.ReconcileEntry, error)

	// BulkUpdateTransactionStatus sets status on all listed transaction IDs
	// in a single atomic UPDATE. Returns an error if the affected row count
	// does not match len(txIDs).
	BulkUpdateTransactionStatus(txIDs []int64, status model.TransactionStatus) error
}

type Repository interface {
	AccountRepository
	TransactionRepository
}

type TransactionManager interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
}