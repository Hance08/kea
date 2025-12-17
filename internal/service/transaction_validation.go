package service

import (
	"fmt"

	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/internal/utils"
)

// ValidateSplitsBalance validates that all splits sum to zero (double-entry principle)
func (ts *TransactionService) ValidateSplitsBalance(splits []store.Split) error {
	var total int64 = 0

	for _, split := range splits {
		total += split.Amount
	}

	if total != 0 {
		return fmt.Errorf("splits do not balance: total is %d cents (%.2f), must be 0. "+
			"In double-entry bookkeeping, debits must equal credits",
			total, float64(total)/100.0)
	}

	return nil
}

// ValidateTransactionEdit validates a transaction edit without saving
func (ts *TransactionService) ValidateTransactionEdit(splits []TransactionSplitInput) error {
	// Check minimum splits
	if len(splits) < 2 {
		return fmt.Errorf("transaction must have at least 2 splits")
	}

	// Check balance
	var total int64
	for _, split := range splits {
		total += split.Amount
	}
	if total != 0 {
		return fmt.Errorf("splits do not balance (sum: %s)", utils.FormatFromCents(total))
	}

	// Validate accounts exist
	for i, split := range splits {
		_, err := ts.repo.GetAccountByID(split.AccountID)
		if err != nil {
			return fmt.Errorf("split #%d: account ID %d not found", i+1, split.AccountID)
		}
	}

	return nil
}
