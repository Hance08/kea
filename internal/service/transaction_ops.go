// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

// CreateTransaction validates and persists a new transaction along with its associated splits.
//
// The process includes:
// 1. Validating that at least 2 splits exist (Double-entry principle).
// 2. Setting a default timestamp if one is not provided.
// 3. Resolving account names to IDs and determining the appropriate currency.
// 4. Verifying that the total amount of all splits balances to zero.
// 5. executing the write operation within an atomic database transaction.
func (ts *TransactionService) CreateTransaction(ctx context.Context, input model.TransactionDetail) (int64, error) {
	defaultCurrency := ts.config.Defaults.Currency

	// Validate: According to double-entry bookkeeping principles,
	// a transaction must consist of at least 2 splits.
	if len(input.Splits) < 2 {
		return 0, fmt.Errorf("transaction must have at least 2 splits (got %d)", len(input.Splits))
	}

	if input.Type == "" {
		return 0, fmt.Errorf("transaction type is required")
	}

	// Set default timestamp: Use current system time if not provided.
	if input.Timestamp == 0 {
		input.Timestamp = time.Now().Unix()
	}

	// Prepare to resolve account names to IDs and build split entities.
	var splits []model.Split
	currency := defaultCurrency

	for i, splitInput := range input.Splits {
		// Step 1: Validate account existence and retrieve account details.
		account, err := ts.accRepo.GetAccountByName(ctx, splitInput.AccountName)
		if err != nil {
			return 0, fmt.Errorf("split #%d: %w", i+1, err)
		}

		if err := ts.checkAccountSelectable(ctx, account); err != nil {
			return 0, fmt.Errorf("split #%d: %w", i+1, err)
		}

		// Step 2: Determine the currency for the split.
		// Prioritize the account's specific currency; otherwise, fall back to the system default.
		splitCurrency := currency
		if account.Currency != "" {
			splitCurrency = account.Currency
		}

		splits = append(splits, model.Split{
			AccountID: account.ID,
			Amount:    splitInput.Amount,
			Currency:  splitCurrency,
			Memo:      splitInput.Memo,
		})
	}

	if err := ts.ValidateSplitsMatchType(ctx, input.Type, input.Splits); err != nil {
		return 0, fmt.Errorf("splits do not match transaction type %q: %w", input.Type, err)
	}

	// Validate: Ensure the sum of all splits balances to zero.
	if err := ts.ValidateSplitsBalance(splits); err != nil {
		return 0, err
	}

	// Prepare the transaction object.
	tx := model.Transaction{
		Timestamp:   input.Timestamp,
		Description: input.Description,
		Status:      input.Status,
		Type:        input.Type,
	}

	var newTxID int64

	// Execute Database Transaction:
	// Ensure atomicity when writing the transaction and its splits.
	err := ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
		var err error

		newTxID, err = repo.CreateTransactionWithSplits(ctx, tx, splits)
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

// checkAccountSelectable returns an error if the account is hidden or has child accounts.
func (ts *TransactionService) checkAccountSelectable(ctx context.Context, account *model.Account) error {
	if account.IsHidden {
		return fmt.Errorf("account %q is hidden", account.Name)
	}
	hasChildren, err := ts.accRepo.HasChildAccounts(ctx, account.ID)
	if err != nil {
		return err
	}
	if hasChildren {
		return fmt.Errorf("account %q is a parent account; select a leaf account instead", account.Name)
	}
	return nil
}

// CreateSimpleTransaction simplifies the creation of a double-entry transaction by
// abstracting the "Credit/Debit" logic into a directional "From -> To" flow.
//
// It automatically generates two balanced splits:
//   - A Credit (negative) to the fromAccount (Source).
//   - A Debit (positive) to the toAccount (Destination).
//
// If txType is empty, it is inferred from the account types via DetermineType.
// Returns the constructed TransactionDetail (useful for UI rendering).
func (ts *TransactionService) CreateSimpleTransaction(ctx context.Context, fromAccount, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error) {
	if fromAccount == toAccount {
		return model.TransactionDetail{}, fmt.Errorf("source and destination accounts cannot be the same")
	}
	if amount <= 0 {
		return model.TransactionDetail{}, fmt.Errorf("amount must be positive")
	}

	// If no type provided, infer from account types.
	if txType == "" {
		fromAcc, err := ts.accRepo.GetAccountByName(ctx, fromAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve from account: %w", err)
		}
		toAcc, err := ts.accRepo.GetAccountByName(ctx, toAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve to account: %w", err)
		}
		inferred, err := ts.DetermineType(ctx, []model.SplitDetail{
			{AccountType: toAcc.Type, Amount: amount},
			{AccountType: fromAcc.Type, Amount: -amount},
		})
		if err != nil {
			return model.TransactionDetail{}, err
		}
		txType = inferred
	}

	splits := []model.SplitDetail{
		{AccountName: toAccount, Amount: amount},
		{AccountName: fromAccount, Amount: -amount},
	}
	input := model.TransactionDetail{
		Timestamp:   timestamp,
		Description: desc,
		Status:      status,
		Type:        txType,
		Splits:      splits,
	}
	id, err := ts.CreateTransaction(ctx, input)
	if err != nil {
		return model.TransactionDetail{}, err
	}
	input.ID = id
	return input, nil
}

// CreateTransactionFromSplits creates a transaction from an explicit slice of splits.
// All validation (balance, type match, ≥2 splits, account resolution) is handled
// by CreateTransaction.
func (ts *TransactionService) CreateTransactionFromSplits(
	ctx context.Context,
	splits []model.SplitDetail,
	desc string,
	timestamp int64,
	status model.TransactionStatus,
	txType model.TransactionType,
) (model.TransactionDetail, error) {
	input := model.TransactionDetail{
		Timestamp:   timestamp,
		Description: desc,
		Status:      status,
		Type:        txType,
		Splits:      splits,
	}
	id, err := ts.CreateTransaction(ctx, input)
	if err != nil {
		return model.TransactionDetail{}, err
	}
	input.ID = id
	return input, nil
}

// DeleteTransaction deletes a transaction
func (ts *TransactionService) DeleteTransaction(ctx context.Context, txID int64) error {
	if txID == model.SystemTransactionID {
		return fmt.Errorf("cannot delete the initial opening transaction: %w", ErrNotEditable)
	}

	tx, err := ts.txRepo.GetTransactionByID(ctx, txID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if tx.Status == model.StatusReconciled {
		return fmt.Errorf("transaction #%d cannot be deleted: %w", tx.ID, ErrReconciled)
	}
	return ts.txRepo.DeleteTransaction(ctx, txID)
}

// UpdateTransactionStatus updates the lifecycle state of a transaction identified by its ID.
// It validates that the provided status is a legal value (Pending or Cleared) before persisting.
func (ts *TransactionService) UpdateTransactionStatus(ctx context.Context, txID int64, status model.TransactionStatus) error {

	// Business Rule: Restrict status updates to valid enum constant to ensure data integrity.
	if status != model.StatusPending && status != model.StatusCleared {
		return fmt.Errorf("invalid status: must be 0 (Pending) or 1 (Cleared)")
	}

	if txID == model.SystemTransactionID {
		return fmt.Errorf("transaction #%d cannot be modified: %w", txID, ErrNotEditable)
	}

	oldTx, err := ts.txRepo.GetTransactionByID(ctx, txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if oldTx.Status == model.StatusReconciled {
		return fmt.Errorf("transaction #%d cannot be modified: %w", txID, ErrReconciled)
	}

	return ts.txRepo.UpdateTransactionStatus(ctx, txID, status)
}

// UpdateTransactionComplete performs a complete update of a transaction including splits
// This operation is atomic - either all changes succeed or all fail
func (ts *TransactionService) UpdateTransactionComplete(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, splits []model.SplitDetail) error {
	// Validate status
	if status != model.StatusPending && status != model.StatusCleared && status != model.StatusReconciled {
		return fmt.Errorf("invalid status: must be 0 (Pending), 1 (Cleared) or 2 (Reconciled)")
	}

	if txID == model.SystemTransactionID {
		return fmt.Errorf("cannot modify the initial opening transaction: %w", ErrNotEditable)
	}

	oldTx, err := ts.txRepo.GetTransactionByID(ctx, txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if oldTx.Status == model.StatusReconciled {
		return fmt.Errorf("transaction #%d cannot be modified: %w", txID, ErrReconciled)
	}

	// Validate that we have at least 2 splits
	if len(splits) < 2 {
		return fmt.Errorf("transaction must have at least 2 splits for double-entry bookkeeping")
	}

	if err := ts.ValidateSplitDetailsBalance(splits); err != nil {
		return err
	}

	if err := ts.ValidateSplitsMatchType(ctx, txType, splits); err != nil {
		return fmt.Errorf("splits do not match transaction type %q: %w", txType, err)
	}

	return ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
		if err := repo.UpdateTransactionBasic(ctx, txID, description, timestamp, status, txType); err != nil {
			return err
		}

		existingSplits, err := repo.GetSplitsByTransaction(ctx, txID)
		if err != nil {
			return err
		}

		// Build maps from the single GetSplitsByTransaction call inside the transaction.
		existingAccountByID := make(map[int64]int64, len(existingSplits))
		existingSplitMap := make(map[int64]*model.Split)
		for _, s := range existingSplits {
			existingAccountByID[s.ID] = s.AccountID
			existingSplitMap[s.ID] = s
		}

		// Reject duplicate or foreign split IDs.
		seenSplitIDs := make(map[int64]bool, len(splits))
		for _, split := range splits {
			if split.ID == 0 {
				continue
			}
			if seenSplitIDs[split.ID] {
				return fmt.Errorf("duplicate split ID %d in input", split.ID)
			}
			seenSplitIDs[split.ID] = true
			if _, ok := existingAccountByID[split.ID]; !ok {
				return fmt.Errorf("split ID %d does not belong to transaction %d", split.ID, txID)
			}
		}

		// Validate accounts; enforce selectability only for new or changed splits.
		for _, split := range splits {
			account, err := ts.accRepo.GetAccountByID(ctx, split.AccountID)
			if err != nil {
				return fmt.Errorf("account ID %d not found", split.AccountID)
			}
			isNew := split.ID == 0
			accountChanged := split.ID != 0 && existingAccountByID[split.ID] != split.AccountID
			if isNew || accountChanged {
				if err := ts.checkAccountSelectable(ctx, account); err != nil {
					return fmt.Errorf("split (account ID %d): %w", split.AccountID, err)
				}
			}
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
				if err := repo.DeleteSplit(ctx, id); err != nil {
					return fmt.Errorf("failed to delete split: %w", err)
				}
			}
		}

		// Update existing splits or create new ones
		for _, split := range splits {
			if split.ID == 0 {
				newSplit := &model.Split{
					TransactionID: txID,
					AccountID:     split.AccountID,
					Amount:        split.Amount,
					Currency:      split.Currency,
					Memo:          split.Memo,
				}
				_, err := repo.CreateSplit(ctx, txID, newSplit)
				if err != nil {
					return err
				}
			} else {
				if err := repo.UpdateSplit(ctx, split.ID, split.AccountID, split.Amount, split.Currency, split.Memo); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// NotEditableReason identifies why a transaction cannot be edited.
type NotEditableReason int

const (
	EditableOK            NotEditableReason = 0 // transaction may be edited
	NotEditableSystemTx   NotEditableReason = 1 // opening-balance system transaction
	NotEditableReconciled NotEditableReason = 2 // already reconciled
)

func (ts *TransactionService) IsEditable(detail *model.TransactionDetail) (bool, NotEditableReason) {
	if detail.ID == model.SystemTransactionID {
		return false, NotEditableSystemTx
	}

	if detail.Status == model.StatusReconciled {
		return false, NotEditableReconciled
	}

	return true, EditableOK
}
