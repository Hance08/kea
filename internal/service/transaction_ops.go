package service

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/store"
)

func (ts *TransactionService) CreateOpeningBalance(account *store.Account, amountInCents int64) error {
	currency := ts.config.Defaults.Currency

	if amountInCents == 0 {
		return nil
	}

	openingBalanceAccount, err := ts.repo.GetAccountByName("Equity:OpeningBalances")
	if err != nil {
		return fmt.Errorf("error : can not find 'Equity:OpeningBalances' account, failed to set initial balance")
	}

	var balanceAmount int64
	var equityAmount int64

	switch account.Type {
	case "A":
		balanceAmount = amountInCents
		equityAmount = -amountInCents
	case "L":
		balanceAmount = -amountInCents
		equityAmount = amountInCents
	default:
		return fmt.Errorf("only Assets(A) and Liabilities(L) account can set balance")
	}

	tx := store.Transaction{
		Timestamp:   time.Now().Unix(),
		Description: "Opening Balance",
		Status:      1,
	}

	splits := []store.Split{
		{
			AccountID: account.ID,
			Amount:    balanceAmount,
			Currency:  currency,
			Memo:      "Opening Balance",
		},
		{
			AccountID: openingBalanceAccount.ID,
			Amount:    equityAmount,
			Currency:  currency,
			Memo:      "Opening Balance",
		},
	}

	return ts.repo.ExecTx(func(repo store.Repository) error {
		_, err = ts.repo.CreateTransactionWithSplits(tx, splits)
		return err
	})

}

// CreateTransaction creates a new transaction with validation
// It validates that:
// 1. All accounts exist
// 2. Splits balance to zero (double-entry bookkeeping)
// 3. At least 2 splits are provided
func (ts *TransactionService) CreateTransaction(input TransactionInput) (int64, error) {
	defaultCurrency := ts.config.Defaults.Currency
	// Validate: at least 2 splits required
	if len(input.Splits) < 2 {
		return 0, fmt.Errorf("transaction must have at least 2 splits (got %d)", len(input.Splits))
	}

	// Set default timestamp if not provided
	if input.Timestamp == 0 {
		input.Timestamp = time.Now().Unix()
	}

	// Convert account names to account IDs and build splits
	var splits []store.Split
	currency := defaultCurrency

	for i, splitInput := range input.Splits {
		// Validate account exists
		account, err := ts.repo.GetAccountByName(splitInput.AccountName)
		if err != nil {
			return 0, fmt.Errorf("split #%d: %w", i+1, err)
		}

		// Use account'ts currency if available, otherwise use default
		splitCurrency := currency
		if account.Currency != "" {
			splitCurrency = account.Currency
		}

		splits = append(splits, store.Split{
			AccountID: account.ID,
			Amount:    splitInput.Amount,
			Currency:  splitCurrency,
			Memo:      splitInput.Memo,
		})
	}

	// Validate: splits must balance to zero
	if err := ts.ValidateSplitsBalance(splits); err != nil {
		return 0, err
	}

	// Create transaction
	tx := store.Transaction{
		Timestamp:   input.Timestamp,
		Description: input.Description,
		Status:      input.Status,
	}

	var newTxID int64

	err := ts.repo.ExecTx(func(repo store.Repository) error {
		var err error

		newTxID, err = repo.CreateTransactionWithSplits(tx, splits)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return newTxID, nil
}

// DeleteTransaction deletes a transaction
func (ts *TransactionService) DeleteTransaction(txID int64) error {
	tx, _, err := ts.repo.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if tx.Status == store.StatusReconciled {
		return fmt.Errorf("operation Denied: Transaction #%d has been reconciled and cannot be deleted", tx.ID)
	}
	return ts.repo.DeleteTransaction(txID)
}

// UpdateTransactionStatus updates the status of a transaction
func (ts *TransactionService) UpdateTransactionStatus(txID int64, status int) error {
	if status != store.StatusPending && status != store.StatusCleared {
		return fmt.Errorf("invalid status: must be 0 (Pending) or 1 (Cleared)")
	}
	return ts.repo.UpdateTransactionStatus(txID, status)
}

// UpdateTransactionComplete performs a complete update of a transaction including splits
// This operation is atomic - either all changes succeed or all fail
func (ts *TransactionService) UpdateTransactionComplete(txID int64, description string, timestamp int64, status int, splits []TransactionSplitInput) error {
	// Validate status
	if status != store.StatusPending && status != store.StatusCleared && status != store.StatusReconciled {
		return fmt.Errorf("invalid status: must be 0 (Pending), 1 (Cleared) or 2 (Reconciled)")
	}

	oldTx, _, err := ts.repo.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if oldTx.Status == store.StatusReconciled {
		if status == store.StatusReconciled {
			return fmt.Errorf("operation denied: transaction #%d has been reconciled", txID)
		}
	}

	// Validate that we have at least 2 splits
	if len(splits) < 2 {
		return fmt.Errorf("transaction must have at least 2 splits for double-entry bookkeeping")
	}

	// Validate splits balance
	var total int64
	for _, split := range splits {
		total += split.Amount
	}
	if total != 0 {
		return fmt.Errorf("splits must balance to zero (current sum: %d)", total)
	}

	// Validate all accounts exist
	for _, split := range splits {
		_, err := ts.repo.GetAccountByID(split.AccountID)
		if err != nil {
			return fmt.Errorf("account ID %d not found", split.AccountID)
		}
	}

	// Check transaction exists
	_, _, err = ts.repo.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	return ts.repo.ExecTx(func(repo store.Repository) error {
		if err := repo.UpdateTransactionBasic(txID, description, timestamp, status); err != nil {
			return err
		}

		existingSplits, err := repo.GetSplitsByTransaction(txID)
		if err != nil {
			return err
		}

		existingSplitMap := make(map[int64]*store.Split)
		for _, split := range existingSplits {
			existingSplitMap[split.ID] = split
		}

		newSplitMap := make(map[int64]bool)
		for _, split := range splits {
			if split.ID != 0 {
				newSplitMap[split.ID] = true
			}
		}

		// Delete splits that are no longer present
		for id := range existingSplitMap {
			if !newSplitMap[id] {
				if err := repo.DeleteSplit(id); err != nil {
					return fmt.Errorf("failed to delete split: %w", err)
				}
			}
		}

		// Update existing splits or create new ones
		for _, split := range splits {
			if split.ID == 0 {
				// Create new split
				newSplit := &store.Split{
					TransactionID: txID,
					AccountID:     split.AccountID,
					Amount:        split.Amount,
					Currency:      split.Currency,
					Memo:          split.Memo,
				}
				_, err := repo.CreateSplit(txID, newSplit)
				if err != nil {
					return err
				}
			} else {
				// Update existing split
				if err := repo.UpdateSplit(split.ID, split.AccountID, split.Amount, split.Currency, split.Memo); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
