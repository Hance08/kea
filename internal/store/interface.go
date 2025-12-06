package store

type Repository interface {
	// Account Operations
	CreateAccount(name, accType, currency, description string, parentID *int64) (int64, error)
	GetAllAccounts() ([]*Account, error)
	GetAccountByName(name string) (*Account, error)
	GetAccountByID(id int64) (*Account, error)
	AccountExists(name string) (bool, error)
	GetAccountsByType(accType string) ([]*Account, error)
	GetAccountBalance(accountID int64) (int64, error)

	// Transaction Operations
	CreateTransactionWithSplits(tx Transaction, splits []Split) (int64, error)
	GetTransactionByID(txID int64) (*Transaction, []*Split, error)
	GetTransactionsByAccount(accountID int64, limit int) ([]*Transaction, error)
	GetTransactionsByDateRange(startTime, endTime int64) ([]*Transaction, error)
	GetAllTransactions(limit int) ([]*Transaction, error)

	UpdateTransactionStatus(txID int64, status int) error
	DeleteTransaction(txID int64) error
	UpdateTransactionBasic(txID int64, description string, timestamp int64, status int) error

	// Split Operations
	CreateSplit(txID int64, split *Split) (int64, error)
	UpdateSplit(splitID int64, accountID int64, amount int64, currency string, memo string) error
	DeleteSplit(splitID int64) error
	GetSplitsByTransaction(txID int64) ([]*Split, error)

	Close() error
}
