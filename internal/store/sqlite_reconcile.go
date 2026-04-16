package store

import (
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
)

// GetUnreconciledTransactionsByAccount returns all Pending (0) and Cleared (1)
// transactions that have a split on accountID, together with the net split
// amount for that account (grouped to handle rare multi-split-per-account cases).
// Results are ordered by timestamp ASC so the TUI shows chronological order.
func (s *Store) GetUnreconciledTransactionsByAccount(accountID int64) ([]*model.ReconcileEntry, error) {
	rows, err := s.db.Query(`
        SELECT t.id, t.timestamp, t.description, t.status, SUM(s.amount) AS amount
        FROM transactions t
        INNER JOIN splits s ON t.id = s.transaction_id
        WHERE s.account_id = ?
          AND t.status IN (0, 1)
        GROUP BY t.id, t.timestamp, t.description, t.status
        ORDER BY t.timestamp ASC, t.id ASC
    `, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to query unreconciled transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*model.ReconcileEntry
	for rows.Next() {
		e := &model.ReconcileEntry{}
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Description, &e.Status, &e.Amount); err != nil {
			return nil, fmt.Errorf("failed to scan reconcile entry: %w", err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return result, nil
}

// BulkUpdateTransactionStatus sets the status of all listed transaction IDs
// in a single UPDATE statement. Returns an error if the affected row count
// does not match len(txIDs) — indicating one or more IDs did not exist.
func (s *Store) BulkUpdateTransactionStatus(txIDs []int64, status model.TransactionStatus) error {
	if len(txIDs) == 0 {
		return nil
	}

	placeholders := strings.Repeat("?,", len(txIDs))
	placeholders = placeholders[:len(placeholders)-1] // trim trailing comma

	query := fmt.Sprintf(
		"UPDATE transactions SET status = ? WHERE id IN (%s)",
		placeholders,
	)

	args := make([]any, 0, len(txIDs)+1)
	args = append(args, status)
	for _, id := range txIDs {
		args = append(args, id)
	}

	result, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to bulk update transaction status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected != int64(len(txIDs)) {
		return fmt.Errorf("expected to update %d transactions, updated %d", len(txIDs), rowsAffected)
	}
	return nil
}
