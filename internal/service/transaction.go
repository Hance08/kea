package service

import (
	"fmt"
	"time"

	"github.com/hance08/kea/internal/currency"
	"github.com/hance08/kea/internal/store"
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

type TransactionService struct {
	repo   store.Repository
	config Config
}

func NewTransactionService(repo store.Repository, cfg Config) *TransactionService {
	return &TransactionService{repo: repo, config: cfg}
}

// CreateTransaction creates a new transaction with validation
// It validates that:
// 1. All accounts exist
// 2. Splits balance to zero (double-entry bookkeeping)
// 3. At least 2 splits are provided
func (al *TransactionService) CreateTransaction(input TransactionInput) (int64, error) {
	defaultCurrency := al.config.DefaultCurrency
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
		account, err := al.repo.GetAccountByName(splitInput.AccountName)
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
	txID, err := al.repo.CreateTransactionWithSplits(tx, splits)
	if err != nil {
		return 0, fmt.Errorf("failed to create transaction: %w", err)
	}

	return txID, nil
}

// ValidateSplitsBalance validates that all splits sum to zero (double-entry principle)
func (al *TransactionService) ValidateSplitsBalance(splits []store.Split) error {
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
func (al *TransactionService) GetTransactionByID(txID int64) (*TransactionDetail, error) {
	tx, splits, err := al.repo.GetTransactionByID(txID)
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
		account, err := al.repo.GetAccountByID(split.AccountID)
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
func (al *TransactionService) GetRecentTransactions(limit int) ([]*store.Transaction, error) {
	transactions, err := al.repo.GetAllTransactions(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent transactions: %w", err)
	}
	return transactions, nil
}

// GetTransactionHistory retrieves transaction history for a specific account
func (al *TransactionService) GetTransactionHistory(accountName string, limit int) ([]*store.Transaction, error) {
	// Get account by name
	account, err := al.repo.GetAccountByName(accountName)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	// Get transactions for this account
	transactions, err := al.repo.GetTransactionsByAccount(account.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}

	return transactions, nil
}

// DeleteTransaction deletes a transaction
func (al *TransactionService) DeleteTransaction(txID int64) error {
	tx, _, err := al.repo.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if tx.Status == store.StatusReconciled {
		return fmt.Errorf("operation Denied: Transaction #%d has been reconciled and cannot be deleted", tx.ID)
	}
	return al.repo.DeleteTransaction(txID)
}

// UpdateTransactionStatus updates the status of a transaction
func (al *TransactionService) UpdateTransactionStatus(txID int64, status int) error {
	if status != store.StatusPending && status != store.StatusCleared {
		return fmt.Errorf("invalid status: must be 0 (Pending) or 1 (Cleared)")
	}
	return al.repo.UpdateTransactionStatus(txID, status)
}

// UpdateTransactionComplete performs a complete update of a transaction including splits
// This operation is atomic - either all changes succeed or all fail
func (al *TransactionService) UpdateTransactionComplete(txID int64, description string, timestamp int64, status int, splits []TransactionSplitInput) error {
	// Validate status
	if status != store.StatusPending && status != store.StatusCleared && status != store.StatusReconciled {
		return fmt.Errorf("invalid status: must be 0 (Pending), 1 (Cleared) or 2 (Reconciled)")
	}

	oldTx, _, err := al.repo.GetTransactionByID(txID)
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
		_, err := al.repo.GetAccountByID(split.AccountID)
		if err != nil {
			return fmt.Errorf("account ID %d not found", split.AccountID)
		}
	}

	// Check transaction exists
	_, _, err = al.repo.GetTransactionByID(txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	return al.repo.ExecTx(func(repo store.Repository) error {
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

// ValidateTransactionEdit validates a transaction edit without saving
func (al *TransactionService) ValidateTransactionEdit(splits []TransactionSplitInput) error {
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
		return fmt.Errorf("splits do not balance (sum: %s)", currency.FormatFromCents(total))
	}

	// Validate accounts exist
	for i, split := range splits {
		_, err := al.repo.GetAccountByID(split.AccountID)
		if err != nil {
			return fmt.Errorf("split #%d: account ID %d not found", i+1, split.AccountID)
		}
	}

	return nil
}
