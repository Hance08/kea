package service

import (
	"fmt"

	"github.com/hance08/kea/internal/model"
)

// GetUnreconciledByAccount returns all Pending/Cleared transactions that
// have a split touching the given account, including the split amount for
// that account. Used to populate the reconciliation TUI.
func (ts *TransactionService) GetUnreconciledByAccount(accountID int64) ([]*model.ReconcileEntry, error) {
	entries, err := ts.txRepo.GetUnreconciledTransactionsByAccount(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to load unreconciled transactions: %w", err)
	}
	return entries, nil
}

// ReconcileTransactions marks the given transaction IDs as reconciled for
// accountID. It validates that every requested ID is in the unreconciled set
// for that account before writing anything.
//
// Returns the difference between statementBalance and the sum of the selected
// split amounts for accountID. A non-zero difference is informational — the
// method always commits if the IDs are valid. The caller decides whether to
// warn (non-zero diff) or proceed silently (zero diff).
func (ts *TransactionService) ReconcileTransactions(accountID int64, statementBalance int64, txIDs []int64) (int64, error) {
	if len(txIDs) == 0 {
		return 0, fmt.Errorf("no transactions selected for reconciliation")
	}

	// 1. Verify account exists.
	if _, err := ts.accRepo.GetAccountByID(accountID); err != nil {
		return 0, fmt.Errorf("account not found: %w", err)
	}

	// 2. Fetch unreconciled transactions and build a valid-ID → amount map.
	entries, err := ts.txRepo.GetUnreconciledTransactionsByAccount(accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to load unreconciled transactions: %w", err)
	}

	validAmounts := make(map[int64]int64, len(entries)) // txID → split amount
	for _, e := range entries {
		validAmounts[e.ID] = e.Amount
	}

	// 3. Validate every requested ID and accumulate the cleared balance.
	var clearedBalance int64
	for _, id := range txIDs {
		amount, ok := validAmounts[id]
		if !ok {
			return 0, fmt.Errorf("transaction ID %d is not in the unreconciled set for this account", id)
		}
		clearedBalance += amount
	}

	// 4. Mark the account's splits as reconciled (split-level tracking so that
	// multi-account transactions remain visible for other accounts).
	if err := ts.txRepo.MarkSplitsReconciledByAccount(accountID, txIDs); err != nil {
		return 0, fmt.Errorf("failed to reconcile transactions: %w", err)
	}

	return statementBalance - clearedBalance, nil
}
