package service

import (
	"fmt"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
)

type TransactionService struct {
	txRepo  TransactionRepository
	accRepo AccountRepository
	tm      TransactionManager
	config  *config.Config
}

func NewTransactionService(txRepo TransactionRepository, accRepo AccountRepository, tm TransactionManager, cfg *config.Config) *TransactionService {
	return &TransactionService{txRepo: txRepo, accRepo: accRepo, tm: tm, config: cfg}
}

func (ts *TransactionService) GetTransactionRule(mode string) (TransactionRule, error) {
	switch mode {
	case "expense":
		return TransactionRule{
			Mode:        "expense",
			SourceTypes: []string{"A", "L"}, // Assets, Liabilities
			DestTypes:   []string{"E"},      // Expenses
		}, nil
	case "income":
		return TransactionRule{
			Mode:        "income",
			SourceTypes: []string{"R"},      // Revenue (Income)
			DestTypes:   []string{"A", "L"}, // Assets, Liabilities
		}, nil
	case "transfer":
		return TransactionRule{
			Mode:        "transfer",
			SourceTypes: []string{"A", "L"},
			DestTypes:   []string{"A", "L"},
		}, nil
	default:
		return TransactionRule{}, fmt.Errorf("unknown transaction mode: %s", mode)
	}
}

// GetTransactionByID retrieves a transaction with all split details
func (ts *TransactionService) GetTransactionByID(txID int64) (*TransactionDetail, error) {
	tx, splits, err := ts.txRepo.GetTransactionByID(txID)
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
		account, err := ts.accRepo.GetAccountByID(split.AccountID)
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
func (ts *TransactionService) GetRecentTransactions(limit int) ([]*model.Transaction, error) {
	transactions, err := ts.txRepo.GetAllTransactions(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent transactions: %w", err)
	}
	return transactions, nil
}

// GetTransactionHistory retrieves transaction history for a specific account
func (ts *TransactionService) GetTransactionHistory(accountName string, limit int) ([]*model.Transaction, error) {
	// Get account by name
	account, err := ts.accRepo.GetAccountByName(accountName)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	// Get transactions for this account
	transactions, err := ts.txRepo.GetTransactionsByAccount(account.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}

	return transactions, nil
}
