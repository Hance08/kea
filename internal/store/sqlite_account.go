// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
	sqlite "github.com/mattn/go-sqlite3"
)

func (s *Store) CreateAccount(ctx context.Context, name string, accType model.AccountType, currency string, description string, parentID *int64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !accType.IsValid() {
		return 0, fmt.Errorf("account type %q: %w", accType, ErrInvalidAccountType)
	}
	stmt, err := s.db.PrepareContext(ctx, `
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

	err = stmt.QueryRowContext(ctx, name, string(accType), currency, description, parentID).Scan(&newID)

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

func (s *Store) GetAllAccounts(ctx context.Context) ([]*model.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `
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

func (s *Store) GetAccountByName(ctx context.Context, name string) (*model.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRowContext(ctx, "SELECT id, name, type, parent_id, currency, description, is_hidden FROM accounts WHERE name = ?", name)

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

func (s *Store) GetAccountByID(ctx context.Context, id int64) (*model.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRowContext(ctx, "SELECT id, name, type, parent_id, currency, description, is_hidden FROM accounts WHERE id = ?", id)

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

func (s *Store) AccountExists(ctx context.Context, name string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var exists bool
	row := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounts WHERE name = ?)", name)
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check account existence: %w", err)
	}
	return exists, nil
}

func (s *Store) GetAllAccountBalances(ctx context.Context, asOf int64) (map[int64]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `
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

func (s *Store) HasChildAccounts(ctx context.Context, accountID int64) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var exists bool
	row := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounts WHERE parent_id = ?)", accountID)
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check child accounts: %w", err)
	}
	return exists, nil
}

func (s *Store) GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `
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

func (s *Store) GetAccountBalance(ctx context.Context, accountID int64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var balance sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
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
func (s *Store) AccountHasTransactions(ctx context.Context, accountID int64) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var exists bool
	row := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM splits WHERE account_id = ?)", accountID)
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check account transactions: %w", err)
	}
	return exists, nil
}

// RenameAccount updates the name of an account and cascades the rename to all
// descendants. The two UPDATEs are not wrapped in a transaction; callers should
// use ExecTx when atomicity is required.
func (s *Store) RenameAccount(ctx context.Context, oldName, newName string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res, err := s.db.ExecContext(ctx, `UPDATE accounts SET name = ? WHERE name = ?`, newName, oldName)
	if err != nil {
		return fmt.Errorf("failed to rename account %q: %w", oldName, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("account %q not found: %w", oldName, ErrRecordNotFound)
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE accounts SET name = ? || substr(name, length(?) + 1)
		 WHERE substr(name, 1, length(? || ':')) = ? || ':'`,
		newName, oldName, oldName, oldName,
	)
	if err != nil {
		return fmt.Errorf("failed to cascade rename from %q to %q: %w", oldName, newName, err)
	}

	return nil
}

// DeleteAccount removes an account record by ID.
func (s *Store) DeleteAccount(ctx context.Context, accountID int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, err := s.db.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", accountID)
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("account with ID %d not found: %w", accountID, ErrRecordNotFound)
	}

	return nil
}

// UpdateAccountMetadata updates the description and hidden status of an account.
func (s *Store) UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, err := s.db.ExecContext(ctx,
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
		return fmt.Errorf("account with ID %d not found: %w", accountID, ErrRecordNotFound)
	}
	return nil
}

func (s *Store) SearchAccounts(ctx context.Context, filter model.AccountFilter, opts model.ListOptions) (*model.ListResult[*model.Account], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opts = normalizeListOpts(opts)

	var whereClauses []string
	var args []any

	if filter.Query != nil && *filter.Query != "" {
		escaped := strings.NewReplacer("%", `\%`, "_", `\_`).Replace(*filter.Query)
		whereClauses = append(whereClauses, `name LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escaped+"%")
	}
	if filter.Type != nil {
		whereClauses = append(whereClauses, "type = ?")
		args = append(args, string(*filter.Type))
	}
	if filter.Currency != nil {
		whereClauses = append(whereClauses, "currency = ?")
		args = append(args, *filter.Currency)
	}
	whereClauses = append(whereClauses, "is_hidden = 0")
	whereClauses = append(whereClauses, "name NOT LIKE 'Equity:OpeningBalances_%'")

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	result := &model.ListResult[*model.Account]{
		Limit:  opts.Limit,
		Offset: opts.Offset,
	}

	if opts.IncludeCount {
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM accounts %s", whereSQL)
		if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&result.TotalCount); err != nil {
			return nil, fmt.Errorf("failed to count search results: %w", err)
		}
	}

	query := fmt.Sprintf(
		"SELECT id, name, type, parent_id, currency, description, is_hidden FROM accounts %s ORDER BY name LIMIT ? OFFSET ?",
		whereSQL,
	)
	queryArgs := append(args, opts.Limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to search accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items, err := s.scanAccounts(rows)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*model.Account{}
	}
	result.Items = items
	return result, nil
}
