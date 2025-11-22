package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string, migrationsPath string) (*Store, error) {
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
	if err := runMigrations(db, migrationsPath); err != nil {
		return nil, fmt.Errorf("failed to migrate database : %w", err)
	}
	// fmt.Println("Successfully migrate database !")

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func runMigrations(db *sql.DB, migrationsPath string) error {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to set up migrate driver : %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
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

func (s *Store) CreateTransactionWithSplits(tx Transaction, splits []Split) error {
	dbTx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start database : %w", err)
	}

	defer dbTx.Rollback()

	stmtTx, err := dbTx.Prepare(`
		INSERT INTO transactions (timestamp, description, status)
		VALUES (?, ?, ?)
		RETURNING id;
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare transaction SQL : %w", err)
	}
	defer stmtTx.Close()

	var newTxID int64
	err = stmtTx.QueryRow(tx.Timestamp, tx.Description, tx.Status).Scan(&newTxID)
	if err != nil {
		return fmt.Errorf("failed to insert transaction : %w", err)
	}

	stmtSplit, err := dbTx.Prepare(`
		INSERT INTO splits (transaction_id, account_id, amount, currency, memo)
		VALUES (?, ?, ?, ?, ?);
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare split SQL : %w", err)
	}
	defer stmtSplit.Close()

	for _, split := range splits {
		_, err := stmtSplit.Exec(newTxID, split.AccountID, split.Amount, split.Currency, split.Memo)
		if err != nil {
			return fmt.Errorf("failed to insert split : %w", err)
		}
	}

	return dbTx.Commit()
}
