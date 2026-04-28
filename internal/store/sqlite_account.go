package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/hance08/kea/internal/model"
	sqlite "github.com/mattn/go-sqlite3"
)

func (s *Store) CreateAccount(name string, accType model.AccountType, currency string, description string, parentID *int64) (int64, error) {
	stmt, err := s.db.PrepareContext(context.Background(), `
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

	err = stmt.QueryRowContext(context.Background(), name, string(accType), currency, description, parentID).Scan(&newID)

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

func (s *Store) GetAllAccounts() ([]*model.Account, error) {
	rows, err := s.db.QueryContext(context.Background(), `
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

func (s *Store) GetAccountByName(name string) (*model.Account, error) {
	row := s.db.QueryRowContext(context.Background(), "SELECT id, name, type, parent_id, currency, description, is_hidden FROM accounts WHERE name = ?", name)

	acc := &model.Account{}
	var parentID sql.NullInt64

	err := row.Scan(
		&acc.ID, &acc.Name, &acc.Type,
		&parentID, &acc.Currency, &acc.Description,
		&acc.IsHidden,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account '%s' not found: %w", name, ErrRecordNotFound)
		}
		return nil, fmt.Errorf("failed to query account '%s': %w", name, err)
	}

	if parentID.Valid {
		acc.ParentID = &parentID.Int64
	}

	return acc, nil
}

func (s *Store) GetAccountByID(id int64) (*model.Account, error) {
	row := s.db.QueryRowContext(context.Background(), "SELECT id, name, type, parent_id, currency, description, is_hidden FROM accounts WHERE id = ?", id)

	acc := &model.Account{}
	var parentID sql.NullInt64

	err := row.Scan(
		&acc.ID, &acc.Name, &acc.Type,
		&parentID, &acc.Currency, &acc.Description,
		&acc.IsHidden,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account with ID %d not found: %w", id, ErrRecordNotFound)
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
	row := s.db.QueryRowContext(context.Background(), "SELECT EXISTS(SELECT 1 FROM accounts WHERE name = ?)", name)
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check account existence: %w", err)
	}
	return exists, nil
}

func (s *Store) GetAllAccountBalances(asOf int64) (map[int64]int64, error) {
	rows, err := s.db.QueryContext(context.Background(), `
        SELECT s.account_id, SUM(s.amount)
        FROM splits s
        JOIN transactions t ON s.transaction_id = t.id
        WHERE t.timestamp <= ?
        GROUP BY s.account_id
    `, asOf)
	if err != nil {
		return nil, fmt.Errorf("failed to query account balances: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64]int64)
	for rows.Next() {
		var accountID int64
		var balance sql.NullInt64
		if err := rows.Scan(&accountID, &balance); err != nil {
			return nil, fmt.Errorf("failed to scan account balance: %w", err)
		}
		if balance.Valid {
			result[accountID] = balance.Int64
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return result, nil
}

func (s *Store) HasChildAccounts(accountID int64) (bool, error) {
	var exists bool
	row := s.db.QueryRowContext(context.Background(), "SELECT EXISTS(SELECT 1 FROM accounts WHERE parent_id = ?)", accountID)
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check child accounts: %w", err)
	}
	return exists, nil
}

func (s *Store) GetAccountsByType(accType model.AccountType) ([]*model.Account, error) {
	rows, err := s.db.QueryContext(context.Background(), `
        SELECT id, name, type, parent_id, currency, description, is_hidden
        FROM accounts
        WHERE type = ?
        ORDER BY name
    `, string(accType))

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
	err := s.db.QueryRowContext(context.Background(), `
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

func (s *Store) scanAccounts(rows *sql.Rows) ([]*model.Account, error) {
	var accounts []*model.Account
	for rows.Next() {
		acc := &model.Account{}
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return accounts, nil
}

// AccountHasTransactions returns true when the account is referenced by any split.
func (s *Store) AccountHasTransactions(accountID int64) (bool, error) {
	var exists bool
	row := s.db.QueryRowContext(context.Background(), "SELECT EXISTS(SELECT 1 FROM splits WHERE account_id = ?)", accountID)
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check account transactions: %w", err)
	}
	return exists, nil
}

// RenameAccount updates the name of an account and cascades the rename to all descendants.
// Both updates run in a single transaction.
func (s *Store) RenameAccount(oldName, newName string) error {
	if s.rawDB == nil {
		return fmt.Errorf("store is already in a transaction")
	}

	tx, err := s.rawDB.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(context.Background(), `UPDATE accounts SET name = ? WHERE name = ?`, newName, oldName)
	if err != nil {
		return fmt.Errorf("failed to rename account %q: %w", oldName, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("account %q not found", oldName)
	}

	_, err = tx.ExecContext(context.Background(),
		`UPDATE accounts SET name = replace(name, ?, ?) WHERE name LIKE ? || ':%'`,
		oldName, newName, oldName,
	)
	if err != nil {
		return fmt.Errorf("failed to cascade rename from %q to %q: %w", oldName, newName, err)
	}

	return tx.Commit()
}

// DeleteAccount removes an account record by ID.
func (s *Store) DeleteAccount(accountID int64) error {
	result, err := s.db.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = ?", accountID)
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("account with ID %d not found", accountID)
	}

	return nil
}

// UpdateAccountMetadata updates the description and hidden status of an account.
func (s *Store) UpdateAccountMetadata(accountID int64, description string, isHidden bool) error {
	res, err := s.db.ExecContext(context.Background(),
		`UPDATE accounts SET description = ?, is_hidden = ? WHERE id = ?`,
		description, isHidden, accountID,
	)
	if err != nil {
		return fmt.Errorf("failed to update account metadata for ID %d: %w", accountID, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("account with ID %d not found", accountID)
	}
	return nil
}
