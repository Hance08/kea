package accounting

import (
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/store"
	"github.com/spf13/viper"
)

type AccountingLogic struct {
	store *store.Store
}

func NewLogic(s *store.Store) *AccountingLogic {
	return &AccountingLogic{store: s}
}

func (al *AccountingLogic) CreateAccount(name, accType, currency, description string, parentID *int64) (*store.Account, error) {
	newID, err := al.store.CreateAccount(name, accType, currency, description, parentID)
	if err != nil {
		return nil, err
	}

	return &store.Account{
		ID:          newID,
		Name:        name,
		Type:        accType,
		Currency:    currency,
		Description: description,
		ParentID:    parentID,
		IsHidden:    false,
	}, nil
}

func (al *AccountingLogic) GetAllAccounts() ([]*store.Account, error) {
	return al.store.GetAllAccounts()
}

func (al *AccountingLogic) GetAccountByName(name string) (*store.Account, error) {
	return al.store.GetAccountByName(name)
}

func (al *AccountingLogic) CheckAccountExists(name string) (bool, error) {
	return al.store.AccountExists(name)
}

func (al *AccountingLogic) GetAccountsByType(accType string) ([]*store.Account, error) {
	return al.store.GetAccountsByType(accType)
}

func (al *AccountingLogic) GetAccountBalanceFormatted(accountID int64) (string, error) {
	balance, err := al.store.GetAccountBalance(accountID)
	if err != nil {
		return "", err
	}

	balanceFloat := float64(balance) / 100
	return fmt.Sprintf("%.2f", balanceFloat), nil
}

func (al *AccountingLogic) GetRootNameByType(accType string) (string, error) {
	switch strings.ToUpper(accType) {
	case "A":
		return "Assets", nil
	case "L":
		return "Liabilities", nil
	case "E":
		return "Expenses", nil
	case "R":
		return "Revenue", nil
	case "C":
		return "Equity", nil
	default:
		return "", fmt.Errorf("invalid account type '%s' (must be A, L, C, R, E)", accType)
	}
}

func (al *AccountingLogic) SetBalance(account *store.Account, amountInCents int64) error {
	if amountInCents == 0 {
		return nil
	}

	openingBalanceAccount, err := al.store.GetAccountByName("Equity:OpeningBalances")
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
			Currency:  viper.GetString("defaults.currency"),
			Memo:      "Opening Balance",
		},
		{
			AccountID: openingBalanceAccount.ID,
			Amount:    equityAmount,
			Currency:  viper.GetString("defaults.currency"),
			Memo:      "Opening Balance",
		},
	}

	_, err = al.store.CreateTransactionWithSplits(tx, splits)
	return err
} // TransactionSplitInput represents a split entry with account name instead of ID
type TransactionSplitInput struct {
	AccountName string // e.g., "Assets:Bank:TaishinBank"
	Amount      int64  // Amount in cents
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
func (al *AccountingLogic) CreateTransaction(input TransactionInput) (int64, error) {
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
func (al *AccountingLogic) ValidateSplitsBalance(splits []store.Split) error {
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
func (al *AccountingLogic) GetTransactionByID(txID int64) (*TransactionDetail, error) {
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

// GetTransactionHistory retrieves transaction history for a specific account
func (al *AccountingLogic) GetTransactionHistory(accountName string, limit int) ([]*store.Transaction, error) {
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
func (al *AccountingLogic) DeleteTransaction(txID int64) error {
	return al.store.DeleteTransaction(txID)
}

// UpdateTransactionStatus updates the status of a transaction
func (al *AccountingLogic) UpdateTransactionStatus(txID int64, status int) error {
	if status != 0 && status != 1 {
		return fmt.Errorf("invalid status: must be 0 (Pending) or 1 (Cleared)")
	}
	return al.store.UpdateTransactionStatus(txID, status)
}

// FormatAmountFromCents converts cents to currency string
func (al *AccountingLogic) FormatAmountFromCents(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100.0)
}

// ParseAmountToCents converts currency string to cents
// e.g., "150.50" -> 15050, "150" -> 15000
func (al *AccountingLogic) ParseAmountToCents(amountStr string) (int64, error) {
	var dollars, cents int64

	// Handle formats: "150", "150.5", "150.50"
	parts := strings.Split(amountStr, ".")

	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount format: %s", amountStr)
	}

	// Parse dollar part
	if parts[0] != "" {
		_, err := fmt.Sscanf(parts[0], "%d", &dollars)
		if err != nil {
			return 0, fmt.Errorf("invalid amount: %s", amountStr)
		}
	}

	// Parse cents part if exists
	if len(parts) == 2 {
		centStr := parts[1]
		// Pad or truncate to 2 digits
		if len(centStr) == 1 {
			centStr += "0" // "150.5" -> "50"
		} else if len(centStr) > 2 {
			centStr = centStr[:2] // Truncate extra digits
		}

		_, err := fmt.Sscanf(centStr, "%d", &cents)
		if err != nil {
			return 0, fmt.Errorf("invalid cents: %s", amountStr)
		}
	}

	total := dollars*100 + cents
	return total, nil
}
