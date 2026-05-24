// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
		return 0, validationErrorf("splits", "transaction must have at least 2 splits (got %d)", len(input.Splits))
	}

	if input.Type == "" {
		return 0, validationErrorf("type", "transaction type is required")
	}

	if input.Status != model.StatusPending && input.Status != model.StatusCleared {
		return 0, validationErrorf("status", "invalid status: new transactions must be Pending or Cleared")
	}

	if strings.TrimSpace(input.Description) == "" {
		return 0, validationErrorf("description", "description is required")
	}
	if len(input.Description) > model.DescriptionMaxLength {
		return 0, validationErrorf("description", "description too long (max %d characters)", model.DescriptionMaxLength)
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

		if err := ts.checkAccountSelectable(ctx, ts.accRepo, account); err != nil {
			return 0, fmt.Errorf("split #%d: %w", i+1, err)
		}

		// Step 2: Determine the currency for the split.
		// Prioritize the account's specific currency; otherwise, fall back to the system default.
		splitCurrency := currency
		if account.Currency != "" {
			splitCurrency = account.Currency
		}

		if len(splitInput.Memo) > model.MemoMaxLength {
			return 0, validationErrorf("memo", "split #%d memo too long (max %d characters)", i+1, model.MemoMaxLength)
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
			if errors.Is(err, repository.ErrAlreadyExists) {
				return fmt.Errorf("transaction already exists: %w", ErrAlreadyExists)
			}
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
func (ts *TransactionService) checkAccountSelectable(ctx context.Context, accRepo repository.AccountRepository, account *model.Account) error {
	if account.IsHidden {
		return validationErrorf("account", "account %q is hidden", account.Name)
	}
	hasChildren, err := accRepo.HasChildAccounts(ctx, account.ID)
	if err != nil {
		return err
	}
	if hasChildren {
		return validationErrorf("account", "account %q is a parent account; select a leaf account instead", account.Name)
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
// If input.Type is empty, it is inferred from the account types via DetermineType.
// Returns the constructed TransactionDetail (useful for UI rendering).
func (ts *TransactionService) CreateSimpleTransaction(ctx context.Context, input model.CreateSimpleTransactionInput) (model.TransactionDetail, error) {
	if input.FromAccount == input.ToAccount {
		return model.TransactionDetail{}, validationErrorf("account", "source and destination accounts cannot be the same")
	}
	if input.Amount <= 0 {
		return model.TransactionDetail{}, validationErrorf("amount", "amount must be positive")
	}

	// The local txType may be overwritten by type inference.
	txType := input.Type

	// If no type provided, infer from account types.
	if txType == "" {
		fromAcc, err := ts.accRepo.GetAccountByName(ctx, input.FromAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve from account: %w", err)
		}
		toAcc, err := ts.accRepo.GetAccountByName(ctx, input.ToAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve to account: %w", err)
		}
		inferred, err := ts.DetermineType(ctx, []model.SplitDetail{
			{AccountID: toAcc.ID, AccountName: input.ToAccount, AccountType: toAcc.Type, Amount: input.Amount},
			{AccountID: fromAcc.ID, AccountName: input.FromAccount, AccountType: fromAcc.Type, Amount: -input.Amount},
		})
		if err != nil {
			return model.TransactionDetail{}, err
		}
		txType = inferred
	}

	splits := []model.SplitDetail{
		{AccountName: input.ToAccount, Amount: input.Amount},
		{AccountName: input.FromAccount, Amount: -input.Amount},
	}
	txDetail := model.TransactionDetail{
		Timestamp:   input.Timestamp,
		Description: input.Description,
		Status:      input.Status,
		Type:        txType,
		Splits:      splits,
	}
	id, err := ts.CreateTransaction(ctx, txDetail)
	if err != nil {
		return model.TransactionDetail{}, err
	}
	txDetail.ID = id
	return txDetail, nil
}

// CreateTransactionFromSplits creates a transaction from an explicit slice of splits.
// All validation (balance, type match, ≥2 splits, account resolution) is handled
// by CreateTransaction.
func (ts *TransactionService) CreateTransactionFromSplits(
	ctx context.Context,
	input model.CreateTransactionFromSplitsInput,
) (model.TransactionDetail, error) {
	txDetail := model.TransactionDetail{
		Timestamp:   input.Timestamp,
		Description: input.Description,
		Status:      input.Status,
		Type:        input.Type,
		Splits:      input.Splits,
	}
	id, err := ts.CreateTransaction(ctx, txDetail)
	if err != nil {
		return model.TransactionDetail{}, err
	}
	txDetail.ID = id
	return txDetail, nil
}

// DeleteTransaction deletes a transaction
func (ts *TransactionService) DeleteTransaction(ctx context.Context, txID int64) error {
	tx, err := ts.txRepo.GetTransactionByID(ctx, txID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("transaction #%d: %w", txID, ErrNotFound)
		}
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
		return validationErrorf("status", "invalid status: must be 0 (Pending) or 1 (Cleared)")
	}

	oldTx, err := ts.txRepo.GetTransactionByID(ctx, txID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("transaction #%d: %w", txID, ErrNotFound)
		}
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if oldTx.Status == model.StatusReconciled {
		return fmt.Errorf("transaction #%d cannot be modified: %w", txID, ErrReconciled)
	}

	return ts.txRepo.UpdateTransactionStatus(ctx, txID, status)
}

// UpdateTransactionComplete performs a complete update of a transaction including splits
// This operation is atomic - either all changes succeed or all fail
func (ts *TransactionService) UpdateTransactionComplete(ctx context.Context, input model.UpdateTransactionInput) error {
	if input.Status != model.StatusPending && input.Status != model.StatusCleared {
		return validationErrorf("status", "invalid status: must be Pending or Cleared")
	}

	if strings.TrimSpace(input.Description) == "" {
		return validationErrorf("description", "description is required")
	}
	if len(input.Description) > model.DescriptionMaxLength {
		return validationErrorf("description", "description too long (max %d characters)", model.DescriptionMaxLength)
	}

	oldTx, err := ts.txRepo.GetTransactionByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("transaction #%d: %w", input.ID, ErrNotFound)
		}
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if oldTx.Status == model.StatusReconciled {
		return fmt.Errorf("transaction #%d cannot be modified: %w", input.ID, ErrReconciled)
	}

	// Validate that we have at least 2 splits
	if len(input.Splits) < 2 {
		return validationErrorf("splits", "transaction must have at least 2 splits for double-entry bookkeeping")
	}

	if err := ts.ValidateSplitDetailsBalance(input.Splits); err != nil {
		return err
	}

	if err := ts.ValidateSplitsMatchType(ctx, input.Type, input.Splits); err != nil {
		return fmt.Errorf("splits do not match transaction type %q: %w", input.Type, err)
	}

	for i, s := range input.Splits {
		if len(s.Memo) > model.MemoMaxLength {
			return validationErrorf("memo", "split #%d memo too long (max %d characters)", i+1, model.MemoMaxLength)
		}
	}

	return ts.tm.ExecTx(ctx, func(repo repository.Repository) error {
		if err := repo.UpdateTransactionBasic(ctx, input.ID, input.Description, input.Timestamp, input.Status, input.Type); err != nil {
			return err
		}

		existingSplits, err := repo.GetSplitsByTransaction(ctx, input.ID)
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
		seenSplitIDs := make(map[int64]bool, len(input.Splits))
		for _, split := range input.Splits {
			if split.ID == 0 {
				continue
			}
			if seenSplitIDs[split.ID] {
				return validationErrorf("splits", "duplicate split ID %d in input", split.ID)
			}
			seenSplitIDs[split.ID] = true
			if _, ok := existingAccountByID[split.ID]; !ok {
				return validationErrorf("splits", "split ID %d does not belong to transaction %d", split.ID, input.ID)
			}
		}

		// Validate accounts; enforce selectability only for new or changed splits.
		for _, split := range input.Splits {
			account, err := repo.GetAccountByID(ctx, split.AccountID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return validationErrorf("splits", "account ID %d not found", split.AccountID)
				}
				return err
			}
			isNew := split.ID == 0
			accountChanged := split.ID != 0 && existingAccountByID[split.ID] != split.AccountID
			if isNew || accountChanged {
				if err := ts.checkAccountSelectable(ctx, repo, account); err != nil {
					return fmt.Errorf("split (account ID %d): %w", split.AccountID, err)
				}
			}
		}

		newSplitMap := make(map[int64]bool)
		for _, split := range input.Splits {
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
		for _, split := range input.Splits {
			if split.ID == 0 {
				newSplit := &model.Split{
					TransactionID: input.ID,
					AccountID:     split.AccountID,
					Amount:        split.Amount,
					Currency:      split.Currency,
					Memo:          split.Memo,
				}
				_, err := repo.CreateSplit(ctx, input.ID, newSplit)
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
	NotEditableReconciled NotEditableReason = 1 // already reconciled
)

func (ts *TransactionService) IsEditable(detail *model.TransactionDetail) (bool, NotEditableReason) {
	if detail.Status == model.StatusReconciled {
		return false, NotEditableReconciled
	}
	return true, EditableOK
}

// ParseTransactionDate parses a date string in the format "2006-01-02" into a Unix timestamp.
// If dateStr is empty, it returns the current time as a Unix timestamp.
func (ts *TransactionService) ParseTransactionDate(dateStr string) (int64, error) {
	if dateStr == "" {
		return time.Now().Unix(), nil
	}
	t, err := time.ParseInLocation(model.DateFormat, dateStr, time.Local)
	if err != nil {
		return 0, validationWrap("date", fmt.Sprintf("invalid date format, use %s", model.DateFormat), err)
	}
	return t.Unix(), nil
}
