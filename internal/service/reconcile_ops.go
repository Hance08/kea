// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

// GetUnreconciledByAccount returns all Pending/Cleared transactions that have
// a split touching the given account, together with the account's last
// reconciled balance. Used to populate the reconciliation TUI.
//
// The last reconciled balance is 0 if the account has never been reconciled.
func (ts *TransactionService) GetUnreconciledByAccount(ctx context.Context, accountID int64) ([]*model.ReconcileEntry, int64, error) {
	entries, err := ts.txRepo.GetUnreconciledTransactionsByAccount(ctx, accountID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load unreconciled transactions: %w", err)
	}
	lastBalance, err := ts.txRepo.GetLastReconciledBalance(ctx, accountID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load last reconciled balance: %w", err)
	}
	return entries, lastBalance, nil
}

// PreviewReconcile validates the given transaction IDs against the
// unreconciled set for accountID and computes the balance difference without
// persisting any changes. Use this to check whether --force is needed before
// committing to a write.
//
// The returned difference is:
//
//	statementBalance − (lastReconciledBalance + clearedBalance)
func (ts *TransactionService) PreviewReconcile(ctx context.Context, accountID int64, statementBalance int64, txIDs []int64) (int64, error) {
	if len(txIDs) == 0 {
		return 0, validationErrorf("transactions", "no transactions selected for reconciliation")
	}

	if _, err := ts.accRepo.GetAccountByID(ctx, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, fmt.Errorf("account #%d: %w", accountID, ErrNotFound)
		}
		return 0, fmt.Errorf("account lookup: %w", err)
	}

	lastBalance, err := ts.txRepo.GetLastReconciledBalance(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to load last reconciled balance: %w", err)
	}

	entries, err := ts.txRepo.GetUnreconciledTransactionsByAccount(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to load unreconciled transactions: %w", err)
	}

	validAmounts := make(map[int64]int64, len(entries))
	for _, e := range entries {
		validAmounts[e.ID] = e.Amount
	}

	var clearedBalance int64
	seen := make(map[int64]bool, len(txIDs))
	for _, id := range txIDs {
		if seen[id] {
			return 0, validationErrorf("transactions", "duplicate transaction ID %d", id)
		}
		seen[id] = true
		amount, ok := validAmounts[id]
		if !ok {
			return 0, validationErrorf("transactions", "transaction ID %d is not in the unreconciled set for this account", id)
		}
		clearedBalance += amount
	}

	return statementBalance - (lastBalance + clearedBalance), nil
}

// ReconcileTransactions marks the given transaction IDs as reconciled for
// accountID. It validates that every requested ID is in the unreconciled set
// for that account before writing anything.
//
// The returned difference is:
//
//	statementBalance − (lastReconciledBalance + clearedBalance)
//
// where clearedBalance is the sum of splits the caller selected in this
// session. A zero difference means the selected transactions exactly bridge
// the gap between the last reconciled balance and the new bank statement.
//
// The new running reconciled balance (lastReconciledBalance + clearedBalance)
// is always persisted after a successful reconcile — regardless of whether the
// difference is zero. The caller decides whether to warn on a non-zero diff.
func (ts *TransactionService) ReconcileTransactions(ctx context.Context, accountID int64, statementBalance int64, txIDs []int64) (int64, error) {
	if len(txIDs) == 0 {
		return 0, validationErrorf("transactions", "no transactions selected for reconciliation")
	}

	// Fast-fail: verify account exists before opening a transaction.
	if _, err := ts.accRepo.GetAccountByID(ctx, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, fmt.Errorf("account #%d: %w", accountID, ErrNotFound)
		}
		return 0, fmt.Errorf("account lookup: %w", err)
	}

	// Duplicate-ID check is pure input validation — no DB needed.
	seen := make(map[int64]bool, len(txIDs))
	for _, id := range txIDs {
		if seen[id] {
			return 0, validationErrorf("transactions", "duplicate transaction ID %d", id)
		}
		seen[id] = true
	}

	// Everything else runs atomically inside a single DB transaction:
	// read last balance → read unreconciled set → validate IDs → mark splits → persist balance.
	var diff int64
	if err := ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
		lastBalance, err := repo.GetLastReconciledBalance(ctx, accountID)
		if err != nil {
			return fmt.Errorf("failed to load last reconciled balance: %w", err)
		}

		entries, err := repo.GetUnreconciledTransactionsByAccount(ctx, accountID)
		if err != nil {
			return fmt.Errorf("failed to load unreconciled transactions: %w", err)
		}

		validAmounts := make(map[int64]int64, len(entries))
		for _, e := range entries {
			validAmounts[e.ID] = e.Amount
		}

		var clearedBalance int64
		for _, id := range txIDs {
			amount, ok := validAmounts[id]
			if !ok {
				return validationErrorf("transactions", "transaction ID %d is not in the unreconciled set for this account", id)
			}
			clearedBalance += amount
		}

		newBalance := lastBalance + clearedBalance

		rowsAffected, err := repo.MarkSplitsReconciledByAccount(ctx, accountID, txIDs)
		if err != nil {
			return fmt.Errorf("failed to reconcile transactions: %w", err)
		}
		if rowsAffected < int64(len(txIDs)) {
			return fmt.Errorf(
				"reconcile: expected splits for %d transactions to be marked, but only %d rows were affected; transaction IDs may not have a split for this account",
				len(txIDs), rowsAffected,
			)
		}

		if err := repo.SetLastReconciledBalance(ctx, accountID, newBalance); err != nil {
			return fmt.Errorf("failed to persist reconciled balance: %w", err)
		}

		diff = statementBalance - newBalance
		return nil
	}); err != nil {
		return 0, err
	}

	return diff, nil
}
