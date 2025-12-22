package store

import (
	"database/sql"
	"errors"
	"fmt"

	sqlite "github.com/mattn/go-sqlite3"
)

func (s *Store) CreateAccount(name, accType, currency, description string, parentID *int64) (int64, error) {
	stmt, err := s.db.Prepare(`
        INSERT INTO accounts (name, type, currency, description, parent_id)
        VALUES (?, ?, ?, ?, ?)
        RETURNING id;
    `)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare SQL : %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	var newID int64

	err = stmt.QueryRow(name, accType, currency, description, parentID).Scan(&newID)

	if err != nil {
		var sqliteErr sqlite.Error
		if errors.As(err, &sqliteErr) {
			if errors.Is(sqliteErr.Code, sqlite.ErrConstraint) || errors.Is(sqliteErr.ExtendedCode, sqlite.ErrConstraintUnique) {
				return 0, fmt.Errorf("failed to create account '%s': %w", name, ErrAccountExists)
			}
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
	defer func() {
		_ = rows.Close()
	}()

	return s.scanAccounts(rows)
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
			return nil, fmt.Errorf("account '%s' doesn't exist", name)
		}
		return nil, fmt.Errorf("failed to query account '%s' : %w", name, ErrRecordNotFound)
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
		return nil, fmt.Errorf("failed to query account with ID %d: %w", id, ErrRecordNotFound)
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
	defer func() {
		_ = rows.Close()
	}()

	return s.scanAccounts(rows)
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

func (s *Store) scanAccounts(rows *sql.Rows) ([]*Account, error) {
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
