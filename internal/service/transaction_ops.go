package service

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/store"
)

func (ts *TransactionService) CreateOpeningBalance(account *model.Account, amountInCents int64) error {
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

	tx := model.Transaction{
		Timestamp:   time.Now().Unix(),
		Description: "Opening Balance",
		Status:      1,
	}

	splits := []model.Split{
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

// CreateTransaction validates and persists a new transaction along with its associated splits.
//
// The process includes:
// 1. Validating that at least 2 splits exist (Double-entry principle).
// 2. Setting a default timestamp if one is not provided.
// 3. Resolving account names to IDs and determining the appropriate currency.
// 4. Verifying that the total amount of all splits balances to zero.
// 5. executing the write operation within an atomic database transaction.
func (ts *TransactionService) CreateTransaction(input TransactionInput) (int64, error) {
	defaultCurrency := ts.config.Defaults.Currency

	// Validate: According to double-entry bookkeeping principles,
	// a transaction must consist of at least 2 splits.
	if len(input.Splits) < 2 {
		return 0, fmt.Errorf("transaction must have at least 2 splits (got %d)", len(input.Splits))
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
		account, err := ts.repo.GetAccountByName(splitInput.AccountName)
		if err != nil {
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

	// Validate: Ensure the sum of all splits balances to zero.
	if err := ts.ValidateSplitsBalance(splits); err != nil {
		return 0, err
	}

	// Prepare the transaction object.
	tx := model.Transaction{
		Timestamp:   input.Timestamp,
		Description: input.Description,
		Status:      input.Status,
	}

	var newTxID int64

	// Execute Database Transaction:
	// Ensure atomicity when writing the transaction and its splits.
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

// CreateSimpleTransaction simplifies the creation of a double-entry transaction by
// abstracting the "Credit/Debit" logic into a directional "From -> To" flow.
//
// It automatically generates two balanced splits:
//   - A Credit (negative) to the fromAccount (Source).
//   - A Debit (positive) to the toAccount (Destination).
//
// Returns the new TransactionID and the constructed TransactionInput (useful for UI rendering).
func (ts *TransactionService) CreateSimpleTransaction(fromAccount, toAccount string, amount int64, desc string, timestamp int64, status int) (int64, TransactionInput, error) {
	if fromAccount == toAccount {
		return 0, TransactionInput{}, fmt.Errorf("source and destination accounts cannot be the same")
	}

	if amount <= 0 {
		return 0, TransactionInput{}, fmt.Errorf("amount must be positive")
	}

	splits := []TransactionSplitInput{
		{
			AccountName: toAccount,
			Amount:      amount,
			Memo:        "",
		},
		{
			AccountName: fromAccount,
			Amount:      -amount,
			Memo:        "",
		},
	}

	input := TransactionInput{
		Timestamp:   timestamp,
		Description: desc,
		Status:      status,
		Splits:      splits,
	}

	id, err := ts.CreateTransaction(input)
	if err != nil {
		return 0, TransactionInput{}, err
	}

	return id, input, nil
}

// DeleteTransaction deletes a transaction
func (ts *TransactionService) DeleteTransaction(txID int64) error {
	if txID == 1 {
		return fmt.Errorf("operation denied: cannot delete the initial opening transaction")
	}

	tx, _, err := ts.repo.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if tx.Status == model.StatusReconciled {
		return fmt.Errorf("operation Denied: Transaction #%d has been reconciled and cannot be deleted", tx.ID)
	}
	return ts.repo.DeleteTransaction(txID)
}

// UpdateTransactionStatus updates the lifecycle state of a transaction identified by its ID.
// It validates that the provided status is a legal value (Pending or Cleared) before persisting.
func (ts *TransactionService) UpdateTransactionStatus(txID int64, status int) error {

	// Business Rule: Restrict status updates to valid enum constants to ensure data integrity.
	if status != model.StatusPending && status != model.StatusCleared {
		return fmt.Errorf("invalid status: must be 0 (Pending) or 1 (Cleared)")
	}
	return ts.repo.UpdateTransactionStatus(txID, status)
}

// UpdateTransactionComplete performs a complete update of a transaction including splits
// This operation is atomic - either all changes succeed or all fail
func (ts *TransactionService) UpdateTransactionComplete(txID int64, description string, timestamp int64, status int, splits []TransactionSplitInput) error {
	// Validate status
	if status != model.StatusPending && status != model.StatusCleared && status != model.StatusReconciled {
		return fmt.Errorf("invalid status: must be 0 (Pending), 1 (Cleared) or 2 (Reconciled)")
	}

	oldTx, _, err := ts.repo.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if oldTx.Status == model.StatusReconciled {
		if status == model.StatusReconciled {
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

		existingSplitMap := make(map[int64]*model.Split)
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
				newSplit := &model.Split{
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

func (ts *TransactionService) IsEditable(detail *TransactionDetail) bool {
	if detail.ID == constants.OpeningBalanceTransactionID {
		return false
	}

	// (Future feature)
	// if detail.Status == model.StatusReconciled {
	//     return false
	// }

	return true
}
