package service

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

// ValidateSplitsBalance validates that all splits sum to zero (double-entry principle)
// and that all splits use the same currency.
func (ts *TransactionService) ValidateSplitsBalance(splits []model.Split) error {
	var total int64 = 0
	var firstCurrency string

	for _, split := range splits {
		if firstCurrency == "" {
			firstCurrency = split.Currency
		} else if split.Currency != firstCurrency {
			return fmt.Errorf("splits must all use the same currency (got %q and %q)", firstCurrency, split.Currency)
		}
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
func (ts *TransactionService) ValidateTransactionEdit(ctx context.Context, splits []model.SplitDetail) error {
	// Check minimum splits
	if len(splits) < model.MinSplitsCount {
		return fmt.Errorf("transaction must have at least 2 splits")
	}

	// Check balance
	var total int64
	for _, split := range splits {
		total += split.Amount
	}
	if total != 0 {
		return fmt.Errorf("splits do not balance (sum: %s)", utils.FormatAmount(total))
	}

	// Validate accounts exist
	for i, split := range splits {
		_, err := ts.accRepo.GetAccountByID(ctx, split.AccountID)
		if err != nil {
			return fmt.Errorf("split #%d: account ID %d not found", i+1, split.AccountID)
		}
	}

	return nil
}
