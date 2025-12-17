package service

import (
	"fmt"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/store"
)

type TransactionService struct {
	repo   store.Repository
	config *config.Config
}

func NewTransactionService(repo store.Repository, cfg *config.Config) *TransactionService {
	return &TransactionService{repo: repo, config: cfg}
}

// GetTransactionByID retrieves a transaction with all split details
func (ts *TransactionService) GetTransactionByID(txID int64) (*TransactionDetail, error) {
	tx, splits, err := ts.repo.GetTransactionByID(txID)
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
		account, err := ts.repo.GetAccountByID(split.AccountID)
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
func (ts *TransactionService) GetRecentTransactions(limit int) ([]*store.Transaction, error) {
	transactions, err := ts.repo.GetAllTransactions(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent transactions: %w", err)
	}
	return transactions, nil
}

// GetTransactionHistory retrieves transaction history for a specific account
func (ts *TransactionService) GetTransactionHistory(accountName string, limit int) ([]*store.Transaction, error) {
	// Get account by name
	account, err := ts.repo.GetAccountByName(accountName)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	// Get transactions for this account
	transactions, err := ts.repo.GetTransactionsByAccount(account.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}

	return transactions, nil
}
