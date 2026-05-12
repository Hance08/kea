// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
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

// ValidateSplitDetailsCurrency checks that all SplitDetail entries use the same currency.
func (ts *TransactionService) ValidateSplitDetailsCurrency(splits []model.SplitDetail) error {
	var firstCurrency string
	var initialized bool
	for _, split := range splits {
		if !initialized {
			firstCurrency = split.Currency
			initialized = true
		} else if split.Currency != firstCurrency {
			return fmt.Errorf("splits must all use the same currency (got %q and %q)", firstCurrency, split.Currency)
		}
	}
	return nil
}

// ValidateSplitDetailsBalance validates that SplitDetail entries sum to zero
// and all use the same currency.
func (ts *TransactionService) ValidateSplitDetailsBalance(splits []model.SplitDetail) error {
	var total int64
	var firstCurrency string
	var initialized bool

	for _, split := range splits {
		if !initialized {
			firstCurrency = split.Currency
			initialized = true
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

	if err := ts.ValidateSplitDetailsBalance(splits); err != nil {
		return err
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
