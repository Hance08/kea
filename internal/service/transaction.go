package service

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/store"
	"github.com/spf13/viper"
)

// TransactionSplitInput represents a split entry with account name instead of ID
type TransactionSplitInput struct {
	ID          int64  // Split ID (0 for new splits)
	AccountName string // e.g., "Assets:Bank:TaishinBank"
	AccountID   int64  // Account ID (used in edit mode)
	Amount      int64  // Amount in cents
	Currency    string // Currency code
	Memo        string // Optional memo for this split
}

// TransactionInput represents user input for creating a transaction
type TransactionInput struct {
	Timestamp   int64                   // Unix timestamp, 0 means current time
	Description string                  // Transaction description
	Splits      []TransactionSplitInput // List of splits (must balance to 0)
	Status      int                     // 0=Pending, 1=Cleared
}

// CreateTransaction creates a new transaction with validation
// It validates that:
// 1. All accounts exist
// 2. Splits balance to zero (double-entry bookkeeping)
// 3. At least 2 splits are provided
func (al *AccountingService) CreateTransaction(input TransactionInput) (int64, error) {
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
	currency := viper.GetString("defaults.currency")

	for i, splitInput := range input.Splits {
		// Validate account exists
		account, err := al.store.GetAccountByName(splitInput.AccountName)
		if err != nil {
			return 0, fmt.Errorf("split #%d: %w", i+1, err)
		}

		// Use account's currency if available, otherwise use default
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
	if err := al.ValidateSplitsBalance(splits); err != nil {
		return 0, err
	}

	// Create transaction
	tx := store.Transaction{
		Timestamp:   input.Timestamp,
		Description: input.Description,
		Status:      input.Status,
	}

	// Use store method to create transaction with splits
	txID, err := al.store.CreateTransactionWithSplits(tx, splits)
	if err != nil {
		return 0, fmt.Errorf("failed to create transaction: %w", err)
	}

	return txID, nil
}

// ValidateSplitsBalance validates that all splits sum to zero (double-entry principle)
func (al *AccountingService) ValidateSplitsBalance(splits []store.Split) error {
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

// TransactionDetail represents a transaction with full split details including account names
type TransactionDetail struct {
	ID          int64
	Timestamp   int64
	Description string
	Status      int
	Splits      []SplitDetail
}

// SplitDetail represents a split with account name included
type SplitDetail struct {
	ID          int64
	AccountID   int64
	AccountName string
	Amount      int64
	Currency    string
	Memo        string
}

// GetTransactionByID retrieves a transaction with all split details
func (al *AccountingService) GetTransactionByID(txID int64) (*TransactionDetail, error) {
	tx, splits, err := al.store.GetTransactionByID(txID)
	if err != nil {
		return nil, err
	}

	// Convert to detail format with account names
	detail := &TransactionDetail{
		ID:          tx.ID,
		Timestamp:   tx.Timestamp,
		Description: tx.Description,
		Status:      tx.Status,
		Splits:      make([]SplitDetail, 0, len(splits)),
	}

	for _, split := range splits {
		// Get account name by ID
		account, err := al.store.GetAccountByID(split.AccountID)
		if err != nil {
			return nil, fmt.Errorf("failed to get account for split: %w", err)
		}

		splitDetail := SplitDetail{
			ID:          split.ID,
			AccountID:   split.AccountID,
			AccountName: account.Name,
			Amount:      split.Amount,
			Currency:    split.Currency,
			Memo:        split.Memo,
		}
		detail.Splits = append(detail.Splits, splitDetail)
	}

	return detail, nil
}

// GetRecentTransactions retrieves recent transactions across all accounts
func (al *AccountingService) GetRecentTransactions(limit int) ([]*store.Transaction, error) {
	transactions, err := al.store.GetAllTransactions(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent transactions: %w", err)
	}
	return transactions, nil
}

// GetTransactionHistory retrieves transaction history for a specific account
func (al *AccountingService) GetTransactionHistory(accountName string, limit int) ([]*store.Transaction, error) {
	// Get account by name
	account, err := al.store.GetAccountByName(accountName)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	// Get transactions for this account
	transactions, err := al.store.GetTransactionsByAccount(account.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}

	return transactions, nil
}

// DeleteTransaction deletes a transaction
func (al *AccountingService) DeleteTransaction(txID int64) error {
	return al.store.DeleteTransaction(txID)
}

// UpdateTransactionStatus updates the status of a transaction
func (al *AccountingService) UpdateTransactionStatus(txID int64, status int) error {
	if status != 0 && status != 1 {
		return fmt.Errorf("invalid status: must be 0 (Pending) or 1 (Cleared)")
	}
	return al.store.UpdateTransactionStatus(txID, status)
}

// UpdateTransactionComplete performs a complete update of a transaction including splits
// This operation is atomic - either all changes succeed or all fail
func (al *AccountingService) UpdateTransactionComplete(txID int64, description string, timestamp int64, status int, splits []TransactionSplitInput) error {
	// Validate status
	if status != 0 && status != 1 {
		return fmt.Errorf("invalid status: must be 0 (Pending) or 1 (Cleared)")
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
		_, err := al.store.GetAccountByID(split.AccountID)
		if err != nil {
			return fmt.Errorf("account ID %d not found", split.AccountID)
		}
	}

	// Check transaction exists
	_, _, err := al.store.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	// Update basic transaction info
	if err := al.store.UpdateTransactionBasic(txID, description, timestamp, status); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Get existing splits
	existingSplits, err := al.store.GetSplitsByTransaction(txID)
	if err != nil {
		return fmt.Errorf("failed to get existing splits: %w", err)
	}

	// Create maps for comparison
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
			if err := al.store.DeleteSplit(id); err != nil {
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
			_, err := al.store.CreateSplit(txID, newSplit)
			if err != nil {
				return fmt.Errorf("failed to create split: %w", err)
			}
		} else {
			// Update existing split
			if err := al.store.UpdateSplit(split.ID, split.AccountID, split.Amount, split.Currency, split.Memo); err != nil {
				return fmt.Errorf("failed to update split: %w", err)
			}
		}
	}

	return nil
}

// ValidateTransactionEdit validates a transaction edit without saving
func (al *AccountingService) ValidateTransactionEdit(splits []TransactionSplitInput) error {
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
		return fmt.Errorf("splits do not balance (sum: %s)", al.FormatAmountFromCents(total))
	}

	// Validate accounts exist
	for i, split := range splits {
		_, err := al.store.GetAccountByID(split.AccountID)
		if err != nil {
			return fmt.Errorf("split #%d: account ID %d not found", i+1, split.AccountID)
		}
	}

	return nil
}
