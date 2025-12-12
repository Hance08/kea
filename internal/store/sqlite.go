package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
)

type DBTX interface {
	Exec(query string, args ...any) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

type Store struct {
	db DBTX
}

func NewStore(dbPath string, migrationsFS fs.FS) (*Store, error) {
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("can not create database directory %s: %w", dbDir, err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("can not open database : %w", err)
	}
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("can not connect with database : %w", err)
	}
	if err := runMigrations(db, migrationsFS); err != nil {
		return nil, fmt.Errorf("failed to migrate database : %w", err)
	}
	// fmt.Println("Successfully migrate database !")

	return &Store{db: db}, nil
}

func (s *Store) ExecTx(fn func(Repository) error) error {
	db, ok := s.db.(*sql.DB)
	if !ok {
		return fmt.Errorf("store is already in a transaction")
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	txStore := &Store{db: tx}

	err = fn(txStore)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

func (s *Store) Close() error {
	if db, ok := s.db.(*sql.DB); ok {
		return db.Close()
	}
	return nil
}

func runMigrations(db *sql.DB, migrationsFS fs.FS) error {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to set up migrate driver : %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create iofs source driver : %w", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		"sqlite3",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to set up migrate instance : %w", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migration(up) : %w", err)
	}

	return nil
}

func (s *Store) CreateAccount(name, accType, currency, description string, parentID *int64) (int64, error) {
	stmt, err := s.db.Prepare(`
		INSERT INTO accounts (name, type, currency, description, parent_id)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id;
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare SQL : %w", err)
	}
	defer stmt.Close()

	var newID int64

	err = stmt.QueryRow(name, accType, currency, description, parentID).Scan(&newID)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: accounts.name") {
			return 0, fmt.Errorf("account name '%s' is already existed", name)
		}
		return 0, fmt.Errorf("failed to executing SQL insertion : %w", err)
	}

	return newID, nil
}

func (s *Store) GetAllAccounts() ([]*Account, error) {
	rows, err := s.db.Query(`
		SELECT id, name, type, parent_id, currency, description, is_hidden
		FROM accounts
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		acc := &Account{}
		var parentID sql.NullInt64

		err := rows.Scan(
			&acc.ID, &acc.Name, &acc.Type,
			&parentID, &acc.Currency, &acc.Description,
			&acc.IsHidden,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}

		if parentID.Valid {
			acc.ParentID = &parentID.Int64
		}

		accounts = append(accounts, acc)
	}

	return accounts, rows.Err()
}

func (s *Store) GetAccountByName(name string) (*Account, error) {
	row := s.db.QueryRow("SELECT id, name, type, parent_id, currency, description, is_hidden FROM accounts WHERE name = ?", name)

	acc := &Account{}

	var parentID sql.NullInt64

	err := row.Scan(
		&acc.ID, &acc.Name, &acc.Type,
		&parentID, &acc.Currency, &acc.Description,
		&acc.IsHidden,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account '%s' dosen't existed", name)
		}
		return nil, fmt.Errorf("failed to query account '%s' : %w", name, err)
	}

	if parentID.Valid {
		acc.ParentID = &parentID.Int64
	}

	return acc, nil
}

func (s *Store) GetAccountByID(id int64) (*Account, error) {
	row := s.db.QueryRow("SELECT id, name, type, parent_id, currency, description, is_hidden FROM accounts WHERE id = ?", id)

	acc := &Account{}

	var parentID sql.NullInt64

	err := row.Scan(
		&acc.ID, &acc.Name, &acc.Type,
		&parentID, &acc.Currency, &acc.Description,
		&acc.IsHidden,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to query account with ID %d: %w", id, err)
	}

	if parentID.Valid {
		acc.ParentID = &parentID.Int64
	}

	return acc, nil
}

func (s *Store) AccountExists(name string) (bool, error) {
	var exists bool
	row := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM accounts WHERE name = ?)", name)
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check account existence: %w", err)
	}
	return exists, nil
}

func (s *Store) GetAccountsByType(accType string) ([]*Account, error) {
	rows, err := s.db.Query(`
		SELECT id, name, type, parent_id, currency, description, is_hidden
		FROM accounts
		WHERE type = ?
		ORDER BY name
	`, accType)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		acc := &Account{}
		var parentID sql.NullInt64

		err := rows.Scan(
			&acc.ID, &acc.Name, &acc.Type,
			&parentID, &acc.Currency, &acc.Description,
			&acc.IsHidden,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}

		if parentID.Valid {
			acc.ParentID = &parentID.Int64
		}

		accounts = append(accounts, acc)
	}

	return accounts, rows.Err()
}

func (s *Store) GetAccountBalance(accountID int64) (int64, error) {
	var balance sql.NullInt64
	err := s.db.QueryRow(`
		SELECT SUM(amount)
		FROM splits
		WHERE account_id = ?
	`, accountID).Scan(&balance)

	if err != nil {
		return 0, fmt.Errorf("failed to calculate balance: %w", err)
	}

	if balance.Valid {
		return balance.Int64, nil
	}
	return 0, nil
}

func (s *Store) CreateTransactionWithSplits(tx Transaction, splits []Split) (int64, error) {
	db, ok := s.db.(*sql.DB)
	if !ok {
		return 0, fmt.Errorf("CreateTransactionWithSplits cannot be called within an existing transaction")
	}

	dbTx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to start database : %w", err)
	}

	defer dbTx.Rollback()

	stmtTx, err := dbTx.Prepare(`
		INSERT INTO transactions (timestamp, description, status, external_id)
		VALUES (?, ?, ?, ?)
		RETURNING id;
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare transaction SQL : %w", err)
	}
	defer stmtTx.Close()

	var newTxID int64
	err = stmtTx.QueryRow(tx.Timestamp, tx.Description, tx.Status, tx.ExternalID).Scan(&newTxID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, fmt.Errorf("transaction already exists (duplicate external_id)")
		}
		return 0, fmt.Errorf("failed to insert transaction : %w", err)
	}

	stmtSplit, err := dbTx.Prepare(`
		INSERT INTO splits (transaction_id, account_id, amount, currency, memo)
		VALUES (?, ?, ?, ?, ?);
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare split SQL : %w", err)
	}
	defer stmtSplit.Close()

	for _, split := range splits {
		_, err := stmtSplit.Exec(newTxID, split.AccountID, split.Amount, split.Currency, split.Memo)
		if err != nil {
			return 0, fmt.Errorf("failed to insert split : %w", err)
		}
	}

	if err := dbTx.Commit(); err != nil {
		return 0, err
	}

	return newTxID, nil
}

// GetTransactionByID retrieves a transaction and all its splits by transaction ID
func (s *Store) GetTransactionByID(txID int64) (*Transaction, []*Split, error) {
	// Query the transaction
	var tx Transaction
	err := s.db.QueryRow(`
		SELECT id, timestamp, description, status, external_id
		FROM transactions
		WHERE id = ?
	`, txID).Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status, &tx.ExternalID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("transaction with ID %d not found", txID)
		}
		return nil, nil, fmt.Errorf("failed to query transaction: %w", err)
	}

	// Query all splits for this transaction
	rows, err := s.db.Query(`
		SELECT id, transaction_id, account_id, amount, currency, memo
		FROM splits
		WHERE transaction_id = ?
		ORDER BY id
	`, txID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query splits: %w", err)
	}
	defer rows.Close()

	var splits []*Split
	for rows.Next() {
		split := &Split{}
		err := rows.Scan(
			&split.ID,
			&split.TransactionID,
			&split.AccountID,
			&split.Amount,
			&split.Currency,
			&split.Memo,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan split: %w", err)
		}
		splits = append(splits, split)
	}

	if err = rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating splits: %w", err)
	}

	return &tx, splits, nil
}

// GetTransactionsByAccount retrieves transactions that involve a specific account
// Returns transactions ordered by timestamp (newest first)
func (s *Store) GetTransactionsByAccount(accountID int64, limit int) ([]*Transaction, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	rows, err := s.db.Query(`
		SELECT DISTINCT t.id, t.timestamp, t.description, t.status, t.external_id
		FROM transactions t
		INNER JOIN splits s ON t.id = s.transaction_id
		WHERE s.account_id = ?
		ORDER BY t.timestamp DESC, t.id DESC
		LIMIT ?
	`, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*Transaction
	for rows.Next() {
		tx := &Transaction{}
		err := rows.Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, rows.Err()
}

// GetTransactionsByDateRange retrieves transactions within a time range
// startTime and endTime are Unix timestamps
func (s *Store) GetTransactionsByDateRange(startTime, endTime int64) ([]*Transaction, error) {
	rows, err := s.db.Query(`
		SELECT id, timestamp, description, status, external_id
		FROM transactions
		WHERE timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC, id DESC
	`, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions by date range: %w", err)
	}
	defer rows.Close()

	var transactions []*Transaction
	for rows.Next() {
		tx := &Transaction{}
		err := rows.Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, rows.Err()
}

// GetAllTransactions retrieves recent transactions with a limit
// Ordered by timestamp (newest first)
func (s *Store) GetAllTransactions(limit int) ([]*Transaction, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	rows, err := s.db.Query(`
		SELECT id, timestamp, description, status, external_id
		FROM transactions
		ORDER BY timestamp DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*Transaction
	for rows.Next() {
		tx := &Transaction{}
		err := rows.Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, rows.Err()
}

// UpdateTransactionStatus updates the status of a transaction
// Status: 0=Pending, 1=Cleared
func (s *Store) UpdateTransactionStatus(txID int64, status int) error {
	result, err := s.db.Exec(`
		UPDATE transactions
		SET status = ?
		WHERE id = ?
	`, status, txID)
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("transaction with ID %d not found", txID)
	}

	return nil
}

// DeleteTransaction deletes a transaction and all its associated splits
// Due to ON DELETE CASCADE, splits will be automatically deleted
func (s *Store) DeleteTransaction(txID int64) error {
	result, err := s.db.Exec(`
		DELETE FROM transactions
		WHERE id = ?
	`, txID)
	if err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("transaction with ID %d not found", txID)
	}

	return nil
}

// UpdateTransactionBasic updates the basic fields of a transaction (description, timestamp, status)
// Does not modify splits
func (s *Store) UpdateTransactionBasic(txID int64, description string, timestamp int64, status int) error {
	result, err := s.db.Exec(`
		UPDATE transactions
		SET description = ?, timestamp = ?, status = ?
		WHERE id = ?
	`, description, timestamp, status, txID)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("transaction with ID %d not found", txID)
	}

	return nil
}

// UpdateSplit updates a single split's information
func (s *Store) UpdateSplit(splitID int64, accountID int64, amount int64, currency string, memo string) error {
	result, err := s.db.Exec(`
		UPDATE splits
		SET account_id = ?, amount = ?, currency = ?, memo = ?
		WHERE id = ?
	`, accountID, amount, currency, memo, splitID)
	if err != nil {
		return fmt.Errorf("failed to update split: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("split with ID %d not found", splitID)
	}

	return nil
}

// DeleteSplit deletes a single split from a transaction
func (s *Store) DeleteSplit(splitID int64) error {
	result, err := s.db.Exec(`
		DELETE FROM splits
		WHERE id = ?
	`, splitID)
	if err != nil {
		return fmt.Errorf("failed to delete split: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("split with ID %d not found", splitID)
	}

	return nil
}

// CreateSplit adds a new split to an existing transaction
func (s *Store) CreateSplit(txID int64, split *Split) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO splits (transaction_id, account_id, amount, currency, memo)
		VALUES (?, ?, ?, ?, ?)
	`, txID, split.AccountID, split.Amount, split.Currency, split.Memo)
	if err != nil {
		return 0, fmt.Errorf("failed to create split: %w", err)
	}

	splitID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return splitID, nil
}

// GetSplitsByTransaction retrieves all splits for a transaction
func (s *Store) GetSplitsByTransaction(txID int64) ([]*Split, error) {
	rows, err := s.db.Query(`
		SELECT id, transaction_id, account_id, amount, currency, memo
		FROM splits
		WHERE transaction_id = ?
		ORDER BY id
	`, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to query splits: %w", err)
	}
	defer rows.Close()

	var splits []*Split
	for rows.Next() {
		split := &Split{}
		err := rows.Scan(
			&split.ID,
			&split.TransactionID,
			&split.AccountID,
			&split.Amount,
			&split.Currency,
			&split.Memo,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan split: %w", err)
		}
		splits = append(splits, split)
	}

	return splits, rows.Err()
}
