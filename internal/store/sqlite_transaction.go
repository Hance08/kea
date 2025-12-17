package store

import (
	"database/sql"
	"errors"
	"fmt"

	sqlite "github.com/mattn/go-sqlite3"
)

// CreateTransactionWithSplits inserts a transaction and its splits.
// It relies on the caller (Service layer) to wrap it in ExecTx for atomicity.
func (s *Store) CreateTransactionWithSplits(tx Transaction, splits []Split) (int64, error) {
	stmtTx, err := s.db.Prepare(`
        INSERT INTO transactions (timestamp, description, status, external_id)
        VALUES (?, ?, ?, ?)
        RETURNING id;
    `)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare transaction SQL: %w", err)
	}
	defer stmtTx.Close()

	var newTxID int64
	err = stmtTx.QueryRow(tx.Timestamp, tx.Description, tx.Status, tx.ExternalID).Scan(&newTxID)

	if err != nil {
		var sqliteErr sqlite.Error
		if errors.As(err, &sqliteErr) {
			if sqliteErr.Code == sqlite.ErrConstraint || sqliteErr.ExtendedCode == sqlite.ErrConstraintUnique {
				return 0, fmt.Errorf("transaction already exists (duplicate external_id)")
			}
		}
		return 0, fmt.Errorf("failed to insert transaction: %w", err)
	}

	stmtSplit, err := s.db.Prepare(`
        INSERT INTO splits (transaction_id, account_id, amount, currency, memo)
        VALUES (?, ?, ?, ?, ?);
    `)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare split SQL: %w", err)
	}
	defer stmtSplit.Close()

	for _, split := range splits {
		_, err := stmtSplit.Exec(newTxID, split.AccountID, split.Amount, split.Currency, split.Memo)
		if err != nil {
			return 0, fmt.Errorf("failed to insert split (account_id: %d): %w", split.AccountID, err)
		}
	}

	return newTxID, nil
}

func (s *Store) GetTransactionByID(txID int64) (*Transaction, []*Split, error) {
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

func (s *Store) GetTransactionsByAccount(accountID int64, limit int) ([]*Transaction, error) {
	if limit <= 0 {
		limit = 100
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
		err := rows.Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status, &tx.ExternalID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, rows.Err()
}

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
		err := rows.Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status, &tx.ExternalID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, rows.Err()
}

func (s *Store) GetAllTransactions(limit int) ([]*Transaction, error) {
	if limit <= 0 {
		limit = 100
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
		err := rows.Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status, &tx.ExternalID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, rows.Err()
}

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
