package store

import (
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
)

// GetUnreconciledTransactionsByAccount returns all transactions that have an
// unreconciled split on accountID (splits.reconciled = 0), together with the
// net split amount for that account (grouped to handle rare multi-split-per-account
// cases). Filtering on the split flag — rather than the transaction status —
// ensures multi-account transactions remain visible for other accounts after one
// account has already been reconciled. Results are ordered by timestamp ASC.
func (s *Store) GetUnreconciledTransactionsByAccount(accountID int64) ([]*model.ReconcileEntry, error) {
	rows, err := s.db.Query(`
        SELECT
            t.id, t.timestamp, t.description, t.status,
            SUM(s.amount) AS amount,
            CASE
                WHEN (
                    SELECT COUNT(DISTINCT s2.account_id)
                    FROM splits s2
                    WHERE s2.transaction_id = t.id AND s2.account_id != ?
                ) > 1
                THEN '(split)'
                ELSE COALESCE(
                    (
                        SELECT a.name
                        FROM splits s2
                        JOIN accounts a ON a.id = s2.account_id
                        WHERE s2.transaction_id = t.id AND s2.account_id != ?
                        LIMIT 1
                    ), ''
                )
            END AS offset_account
        FROM transactions t
        INNER JOIN splits s ON t.id = s.transaction_id
        WHERE s.account_id = ?
          AND s.reconciled = 0
        GROUP BY t.id, t.timestamp, t.description, t.status
        ORDER BY t.timestamp ASC, t.id ASC
    `, accountID, accountID, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to query unreconciled transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*model.ReconcileEntry
	for rows.Next() {
		e := &model.ReconcileEntry{}
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Description, &e.Status, &e.Amount, &e.OffsetAccount); err != nil {
			return nil, fmt.Errorf("failed to scan reconcile entry: %w", err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return result, nil
}

// MarkSplitsReconciledByAccount marks the splits for accountID in each of the
// listed transactions as reconciled (splits.reconciled = 1). After updating the
// splits it checks whether every split in each affected transaction is now
// reconciled; if so the transaction's status is upgraded to StatusReconciled.
// This is intentionally two separate statements (not wrapped in a store-level
// transaction) because the status upgrade is a derived convenience — the split
// flag is the source of truth.
func (s *Store) MarkSplitsReconciledByAccount(accountID int64, txIDs []int64) error {
	if len(txIDs) == 0 {
		return nil
	}

	placeholders := strings.Repeat("?,", len(txIDs))
	placeholders = placeholders[:len(placeholders)-1]

	// 1. Mark the account's splits as reconciled.
	splitArgs := make([]any, 0, len(txIDs)+1)
	splitArgs = append(splitArgs, accountID)
	for _, id := range txIDs {
		splitArgs = append(splitArgs, id)
	}
	splitQuery := fmt.Sprintf(
		"UPDATE splits SET reconciled = 1 WHERE account_id = ? AND transaction_id IN (%s)",
		placeholders,
	)
	if _, err := s.db.Exec(splitQuery, splitArgs...); err != nil {
		return fmt.Errorf("failed to mark splits as reconciled: %w", err)
	}

	// 2. Find transactions where ALL splits are now reconciled.
	txArgs := make([]any, len(txIDs))
	for i, id := range txIDs {
		txArgs[i] = id
	}
	fullyQuery := fmt.Sprintf(`
		SELECT transaction_id FROM splits
		WHERE transaction_id IN (%s)
		GROUP BY transaction_id
		HAVING MIN(reconciled) = 1
	`, placeholders)
	rows, err := s.db.Query(fullyQuery, txArgs...)
	if err != nil {
		return fmt.Errorf("failed to check fully reconciled transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fullyReconciled []int64
	for rows.Next() {
		var txID int64
		if err := rows.Scan(&txID); err != nil {
			return fmt.Errorf("failed to scan transaction ID: %w", err)
		}
		fullyReconciled = append(fullyReconciled, txID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration error: %w", err)
	}

	// 3. Upgrade the status of fully-reconciled transactions.
	if len(fullyReconciled) == 0 {
		return nil
	}
	return s.BulkUpdateTransactionStatus(fullyReconciled, model.StatusReconciled)
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
