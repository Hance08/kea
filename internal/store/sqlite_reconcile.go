// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"context"
	"database/sql"
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
func (s *Store) GetUnreconciledTransactionsByAccount(ctx context.Context, accountID int64) ([]*model.ReconcileEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `
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
// listed transactions as reconciled (splits.reconciled = 1), then upgrades
// every affected transaction's status to StatusReconciled.
//
// The transaction status is upgraded as soon as any split is reconciled — not
// only when all splits are reconciled. Expense, Revenue, and Equity accounts
// are never reconciled against statements, so requiring all splits to be marked
// would mean ordinary expense transactions could never reach StatusReconciled.
// The reconcile TUI filters on s.reconciled = 0 (not t.status), so marking the
// transaction Reconciled here does not hide it from other accounts that still
// need to reconcile their own splits.
func (s *Store) MarkSplitsReconciledByAccount(ctx context.Context, accountID int64, txIDs []int64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(txIDs) == 0 {
		return 0, nil
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
	result, err := s.db.ExecContext(ctx, splitQuery, splitArgs...)
	if err != nil {
		return 0, fmt.Errorf("failed to mark splits as reconciled: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected from splits update: %w", err)
	}

	// 2. Upgrade all affected transactions to StatusReconciled.
	return rowsAffected, s.BulkUpdateTransactionStatus(ctx, txIDs, model.StatusReconciled)
}

// BulkUpdateTransactionStatus sets the status of all listed transaction IDs
// in a single UPDATE statement. Returns an error if the affected row count
// does not match len(txIDs) — indicating one or more IDs did not exist.
func (s *Store) BulkUpdateTransactionStatus(ctx context.Context, txIDs []int64, status model.TransactionStatus) error {
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

	result, err := s.db.ExecContext(ctx, query, args...)
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

// GetLastReconciledBalance returns the running reconciled balance for
// accountID. Returns 0 if the account has never been reconciled (no row in
// account_reconcile_state for this account yet).
func (s *Store) GetLastReconciledBalance(ctx context.Context, accountID int64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var balance int64
	err := s.db.QueryRowContext(ctx,
		"SELECT last_reconciled_balance FROM account_reconcile_state WHERE account_id = ?",
		accountID,
	).Scan(&balance)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get last reconciled balance: %w", err)
	}
	return balance, nil
}

// SetLastReconciledBalance persists the new running reconciled balance for
// accountID. An upsert is used so that the first reconcile for an account
// inserts a row; subsequent reconciles update it.
func (s *Store) SetLastReconciledBalance(ctx context.Context, accountID int64, balance int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO account_reconcile_state (account_id, last_reconciled_balance)
        VALUES (?, ?)
        ON CONFLICT(account_id) DO UPDATE SET last_reconciled_balance = excluded.last_reconciled_balance
    `, accountID, balance)
	if err != nil {
		return fmt.Errorf("failed to set last reconciled balance: %w", err)
	}
	return nil
}
