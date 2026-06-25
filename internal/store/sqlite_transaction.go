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
	"github.com/hance08/kea/internal/repository"
	sqlite "github.com/mattn/go-sqlite3"
)

// CreateTransactionWithSplits inserts a transaction and its splits.
// It relies on the caller (Service layer) to wrap it in ExecTx for atomicity.
func (s *Store) CreateTransactionWithSplits(ctx context.Context, tx model.Transaction, splits []model.Split) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stmtTx, err := s.db.PrepareContext(ctx, `
        INSERT INTO transactions (timestamp, description, status, external_id, type, regular)
        VALUES (?, ?, ?, ?, ?, ?)
        RETURNING id;
    `)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare transaction SQL: %w", err)
	}
	defer func() {
		_ = stmtTx.Close()
	}()

	var newTxID int64
	err = stmtTx.QueryRowContext(ctx, tx.Timestamp, tx.Description, tx.Status, tx.ExternalID, tx.Type, tx.Regular).Scan(&newTxID)

	if err != nil {
		var sqliteErr sqlite.Error
		if errors.As(err, &sqliteErr) {
			if errors.Is(sqliteErr.Code, sqlite.ErrConstraint) || errors.Is(sqliteErr.ExtendedCode, sqlite.ErrConstraintUnique) {
				return 0, fmt.Errorf("duplicate external_id: %w", ErrTransactionExists)
			}
		}
		return 0, fmt.Errorf("failed to insert transaction: %w", err)
	}

	stmtSplit, err := s.db.PrepareContext(ctx, `
        INSERT INTO splits (transaction_id, account_id, amount, currency, memo)
        VALUES (?, ?, ?, ?, ?);
    `)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare split SQL: %w", err)
	}
	defer func() {
		_ = stmtSplit.Close()
	}()

	for _, split := range splits {
		_, err := stmtSplit.ExecContext(ctx, newTxID, split.AccountID, split.Amount, split.Currency, split.Memo)
		if err != nil {
			return 0, fmt.Errorf("failed to insert split (account_id: %d): %w", split.AccountID, err)
		}
	}

	return newTxID, nil
}

func (s *Store) GetTransactionByID(ctx context.Context, txID int64) (*model.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tx model.Transaction
	err := s.db.QueryRowContext(ctx, `
        SELECT id, timestamp, description, status, external_id, type, regular
        FROM transactions
        WHERE id = ?
    `, txID).Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status, &tx.ExternalID, &tx.Type, &tx.Regular)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("transaction with ID %d not found: %w", txID, ErrRecordNotFound)
		}
		return nil, fmt.Errorf("failed to query transaction: %w", err)
	}
	return &tx, nil
}

func (s *Store) GetTransactionsByAccount(ctx context.Context, accountID int64, limit int) ([]*model.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
        SELECT DISTINCT t.id, t.timestamp, t.description, t.status, t.external_id, t.type, t.regular
        FROM transactions t
        INNER JOIN splits s ON t.id = s.transaction_id
        WHERE s.account_id = ?
        ORDER BY t.timestamp DESC, t.id DESC
        LIMIT ?
    `, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	return s.scanTransactions(rows)
}

func (s *Store) GetTransactionsByDateRange(ctx context.Context, startTime, endTime int64) ([]*model.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
        SELECT id, timestamp, description, status, external_id, type, regular
        FROM transactions
        WHERE timestamp >= ? AND timestamp <= ?
        ORDER BY timestamp DESC, id DESC
    `, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions by date range: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	return s.scanTransactions(rows)
}

func (s *Store) GetAllTransactions(ctx context.Context, limit int) ([]*model.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
        SELECT id, timestamp, description, status, external_id, type, regular
        FROM transactions
        ORDER BY timestamp DESC, id DESC
        LIMIT ?
    `, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	return s.scanTransactions(rows)
}

func (s *Store) UpdateTransactionStatus(ctx context.Context, txID int64, status model.TransactionStatus) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, err := s.db.ExecContext(ctx, `
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
		return fmt.Errorf("transaction with ID %d not found: %w", txID, ErrRecordNotFound)
	}

	return nil
}

func (s *Store) DeleteTransaction(ctx context.Context, txID int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, err := s.db.ExecContext(ctx, `
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
		return fmt.Errorf("transaction with ID %d not found: %w", txID, ErrRecordNotFound)
	}

	return nil
}

func (s *Store) UpdateTransactionBasic(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, regular *bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, err := s.db.ExecContext(ctx, `
        UPDATE transactions
        SET description = ?, timestamp = ?, status = ?, type = ?, regular = ?
        WHERE id = ?
    `, description, timestamp, status, txType, regular, txID)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("transaction with ID %d not found: %w", txID, ErrRecordNotFound)
	}

	return nil
}

func (s *Store) UpdateSplit(ctx context.Context, splitID int64, accountID int64, amount int64, currency string, memo string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, err := s.db.ExecContext(ctx, `
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
		return fmt.Errorf("split with ID %d not found: %w", splitID, ErrRecordNotFound)
	}

	return nil
}

func (s *Store) DeleteSplit(ctx context.Context, splitID int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, err := s.db.ExecContext(ctx, `
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
		return fmt.Errorf("split with ID %d not found: %w", splitID, ErrRecordNotFound)
	}

	return nil
}

func (s *Store) CreateSplit(ctx context.Context, txID int64, split *model.Split) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, err := s.db.ExecContext(ctx, `
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

func (s *Store) GetSplitsByTransaction(ctx context.Context, txID int64) ([]*model.Split, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
        SELECT id, transaction_id, account_id, amount, currency, memo
        FROM splits
        WHERE transaction_id = ?
        ORDER BY id
    `, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to query splits: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var splits []*model.Split
	for rows.Next() {
		split := &model.Split{}
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

func (s *Store) GetSplitsWithAccountsByDateRange(ctx context.Context, startTime, endTime int64) (map[int64][]model.SplitDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
        SELECT
            s.id, s.transaction_id, s.account_id, s.amount, s.currency, s.memo,
            a.name, a.type
        FROM transactions t
        JOIN splits s ON t.id = s.transaction_id
        JOIN accounts a ON s.account_id = a.id
        WHERE t.timestamp >= ? AND t.timestamp <= ?
        ORDER BY t.id, s.id
    `, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query splits with accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64][]model.SplitDetail)
	for rows.Next() {
		var d model.SplitDetail
		var txID int64
		if err := rows.Scan(
			&d.ID, &txID, &d.AccountID, &d.Amount, &d.Currency, &d.Memo,
			&d.AccountName, &d.AccountType,
		); err != nil {
			return nil, fmt.Errorf("failed to scan split with account: %w", err)
		}
		result[txID] = append(result[txID], d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return result, nil
}

func (s *Store) GetSplitsWithAccountsByTransaction(ctx context.Context, txID int64) ([]model.SplitDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
        SELECT
            s.id, s.account_id, s.amount, s.currency, s.memo,
            a.name, a.type
        FROM splits s
        JOIN accounts a ON s.account_id = a.id
        WHERE s.transaction_id = ?
        ORDER BY s.id
    `, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to query splits with accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []model.SplitDetail
	for rows.Next() {
		var d model.SplitDetail
		if err := rows.Scan(
			&d.ID, &d.AccountID, &d.Amount, &d.Currency, &d.Memo,
			&d.AccountName, &d.AccountType,
		); err != nil {
			return nil, fmt.Errorf("failed to scan split with account: %w", err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return result, nil
}

// GetSplitsWithAccountsByTransactionIDs returns splits grouped by transaction ID.
// Large ID slices are chunked to stay within SQLite's variable limit.
func (s *Store) GetSplitsWithAccountsByTransactionIDs(ctx context.Context, txIDs []int64) (map[int64][]model.SplitDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(txIDs) == 0 {
		return make(map[int64][]model.SplitDetail), nil
	}

	result := make(map[int64][]model.SplitDetail)
	for _, chunk := range chunkInt64(txIDs, sqliteChunkSize) {
		placeholders := make([]byte, 0, len(chunk)*2-1)
		args := make([]any, len(chunk))
		for i, id := range chunk {
			if i > 0 {
				placeholders = append(placeholders, ',')
			}
			placeholders = append(placeholders, '?')
			args[i] = id
		}

		query := `
            SELECT
                s.id, s.transaction_id, s.account_id, s.amount, s.currency, s.memo,
                a.name, a.type
            FROM splits s
            JOIN accounts a ON s.account_id = a.id
            WHERE s.transaction_id IN (` + string(placeholders) + `)
            ORDER BY s.transaction_id, s.id`

		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to query splits by transaction IDs: %w", err)
		}

		for rows.Next() {
			var d model.SplitDetail
			var txID int64
			if err := rows.Scan(
				&d.ID, &txID, &d.AccountID, &d.Amount, &d.Currency, &d.Memo,
				&d.AccountName, &d.AccountType,
			); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("failed to scan split with account: %w", err)
			}
			result[txID] = append(result[txID], d)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("rows iteration error: %w", err)
		}
		_ = rows.Close()
	}
	return result, nil
}

// normalizeListOpts applies defaults: limit defaults to 20 when ≤0, offset is
// clamped to 0 when negative.
func normalizeListOpts(opts model.ListOptions) model.ListOptions {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	return opts
}

func (s *Store) ListTransactions(ctx context.Context, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opts = normalizeListOpts(opts)

	result := &model.ListResult[*model.Transaction]{
		Limit:  opts.Limit,
		Offset: opts.Offset,
	}

	if opts.IncludeCount {
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&result.TotalCount); err != nil {
			return nil, fmt.Errorf("failed to count transactions: %w", err)
		}
	}

	rows, err := s.db.QueryContext(ctx, `
        SELECT id, timestamp, description, status, external_id, type, regular
        FROM transactions
        ORDER BY timestamp DESC, id DESC
        LIMIT ? OFFSET ?
    `, opts.Limit, opts.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items, err := s.scanTransactions(rows)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*model.Transaction{}
	}
	result.Items = items
	return result, nil
}

func (s *Store) ListTransactionsByAccount(ctx context.Context, accountID int64, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opts = normalizeListOpts(opts)

	result := &model.ListResult[*model.Transaction]{
		Limit:  opts.Limit,
		Offset: opts.Offset,
	}

	if opts.IncludeCount {
		if err := s.db.QueryRowContext(ctx, `
            SELECT COUNT(DISTINCT t.id)
            FROM transactions t
            INNER JOIN splits s ON t.id = s.transaction_id
            WHERE s.account_id = ?
        `, accountID).Scan(&result.TotalCount); err != nil {
			return nil, fmt.Errorf("failed to count transactions by account: %w", err)
		}
	}

	rows, err := s.db.QueryContext(ctx, `
        SELECT DISTINCT t.id, t.timestamp, t.description, t.status, t.external_id, t.type, t.regular
        FROM transactions t
        INNER JOIN splits s ON t.id = s.transaction_id
        WHERE s.account_id = ?
        ORDER BY t.timestamp DESC, t.id DESC
        LIMIT ? OFFSET ?
    `, accountID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions by account: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items, err := s.scanTransactions(rows)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*model.Transaction{}
	}
	result.Items = items
	return result, nil
}

func (s *Store) scanTransactions(rows *sql.Rows) ([]*model.Transaction, error) {
	var transactions []*model.Transaction
	for rows.Next() {
		tx := &model.Transaction{}
		err := rows.Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status, &tx.ExternalID, &tx.Type, &tx.Regular)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return transactions, nil
}

func (s *Store) FilterTransactions(ctx context.Context, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opts = normalizeListOpts(opts)

	needsJoin := filter.AccountID != nil

	var whereClauses []string
	var args []any

	if filter.AccountID != nil {
		whereClauses = append(whereClauses, "s.account_id = ?")
		args = append(args, *filter.AccountID)
	}
	if filter.Type != nil {
		whereClauses = append(whereClauses, "t.type = ?")
		args = append(args, string(*filter.Type))
	}
	if filter.Status != nil {
		whereClauses = append(whereClauses, "t.status = ?")
		args = append(args, int(*filter.Status))
	}
	if filter.StartTime != nil {
		whereClauses = append(whereClauses, "t.timestamp >= ?")
		args = append(args, *filter.StartTime)
	}
	if filter.EndTime != nil {
		whereClauses = append(whereClauses, "t.timestamp <= ?")
		args = append(args, *filter.EndTime)
	}
	if filter.Description != nil {
		escaped := strings.NewReplacer("%", `\%`, "_", `\_`).Replace(*filter.Description)
		whereClauses = append(whereClauses, `t.description LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escaped+"%")
	}
	if filter.Regular != nil {
		whereClauses = append(whereClauses, "t.regular = ?")
		if *filter.Regular {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	fromSQL := "FROM transactions t"
	selectDistinct := ""
	countExpr := "COUNT(*)"
	if needsJoin {
		fromSQL = "FROM transactions t INNER JOIN splits s ON t.id = s.transaction_id"
		selectDistinct = "DISTINCT "
		countExpr = "COUNT(DISTINCT t.id)"
	}

	result := &model.ListResult[*model.Transaction]{
		Limit:  opts.Limit,
		Offset: opts.Offset,
	}

	if opts.IncludeCount {
		countQuery := fmt.Sprintf("SELECT %s %s %s", countExpr, fromSQL, whereSQL)
		if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&result.TotalCount); err != nil {
			return nil, fmt.Errorf("failed to count filtered transactions: %w", err)
		}
	}

	query := fmt.Sprintf(
		"SELECT %st.id, t.timestamp, t.description, t.status, t.external_id, t.type, t.regular %s %s ORDER BY t.timestamp DESC, t.id DESC LIMIT ? OFFSET ?",
		selectDistinct, fromSQL, whereSQL,
	)
	queryArgs := append(args, opts.Limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query filtered transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items, err := s.scanTransactions(rows)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*model.Transaction{}
	}
	result.Items = items
	return result, nil
}

// GetMonthlySplitTotalsForAssetsAndLiabilities returns, for every
// (account_id, month) with activity on an Asset or Liability account,
// the sum of split amounts in that month. Months are UTC "YYYY-MM"
// strings derived from each transaction's Unix timestamp.
func (s *Store) GetMonthlySplitTotalsForAssetsAndLiabilities(ctx context.Context) ([]repository.MonthlySplitTotal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
        SELECT
            s.account_id,
            a.type,
            a.currency,
            strftime('%Y-%m', t.timestamp, 'unixepoch') AS month,
            SUM(s.amount) AS total
        FROM splits s
        JOIN transactions t ON t.id = s.transaction_id
        JOIN accounts     a ON a.id = s.account_id
        WHERE a.type IN ('A', 'L')
        GROUP BY s.account_id, month
        ORDER BY s.account_id, month
    `)
	if err != nil {
		return nil, fmt.Errorf("failed to query monthly split totals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []repository.MonthlySplitTotal
	for rows.Next() {
		var r repository.MonthlySplitTotal
		if err := rows.Scan(&r.AccountID, &r.AccountType, &r.Currency, &r.Month, &r.Amount); err != nil {
			return nil, fmt.Errorf("failed to scan monthly split total: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return result, nil
}
