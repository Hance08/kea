// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
)

// UpsertBudget inserts or replaces the budget version for (accountID, month).
func (s *Store) UpsertBudget(ctx context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO budgets (account_id, effective_month, amount, stopped)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id, effective_month) DO UPDATE
		SET amount = excluded.amount, stopped = excluded.stopped
		RETURNING id`, accountID, month, amount, stopped).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert budget: %w", err)
	}
	return id, nil
}

// ListBudgets returns all budget versions with their account names.
func (s *Store) ListBudgets(ctx context.Context) ([]model.Budget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.id, b.account_id, a.name, b.effective_month, b.amount, b.stopped
		FROM budgets b
		JOIN accounts a ON a.id = b.account_id
		ORDER BY a.name, b.effective_month`)
	if err != nil {
		return nil, fmt.Errorf("failed to list budgets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []model.Budget{}
	for rows.Next() {
		var b model.Budget
		if err := rows.Scan(&b.ID, &b.AccountID, &b.AccountName, &b.EffectiveMonth, &b.Amount, &b.Stopped); err != nil {
			return nil, fmt.Errorf("failed to scan budget: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate budgets: %w", err)
	}
	return out, nil
}

// DeleteBudget removes one budget version.
func (s *Store) DeleteBudget(ctx context.Context, id int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM budgets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete budget: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("budget with ID %d not found: %w", id, ErrRecordNotFound)
	}
	return nil
}
