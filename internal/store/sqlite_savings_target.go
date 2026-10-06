// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
)

// UpsertSavingsTarget inserts or replaces the savings target version for (accountID, month).
func (s *Store) UpsertSavingsTarget(ctx context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO savings_targets (account_id, effective_month, amount, stopped)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id, effective_month) DO UPDATE
		SET amount = excluded.amount, stopped = excluded.stopped
		RETURNING id`, accountID, month, amount, stopped).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert savings target: %w", err)
	}
	return id, nil
}

// ListSavingsTargets returns all savings target versions with their account names.
func (s *Store) ListSavingsTargets(ctx context.Context) ([]model.SavingsTarget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.account_id, a.name, t.effective_month, t.amount, t.stopped
		FROM savings_targets t
		JOIN accounts a ON a.id = t.account_id
		ORDER BY a.name, t.effective_month`)
	if err != nil {
		return nil, fmt.Errorf("failed to list savings targets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []model.SavingsTarget{}
	for rows.Next() {
		var t model.SavingsTarget
		if err := rows.Scan(&t.ID, &t.AccountID, &t.AccountName, &t.EffectiveMonth, &t.Amount, &t.Stopped); err != nil {
			return nil, fmt.Errorf("failed to scan savings target: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate savings targets: %w", err)
	}
	return out, nil
}

// DeleteSavingsTarget removes one savings target version.
func (s *Store) DeleteSavingsTarget(ctx context.Context, id int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM savings_targets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete savings target: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("savings target with ID %d not found: %w", id, ErrRecordNotFound)
	}
	return nil
}
