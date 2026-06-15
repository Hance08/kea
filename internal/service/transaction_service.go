// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

type TransactionService struct {
	txRepo  repository.TransactionRepository
	accRepo repository.AccountRepository
	tm      repository.TransactionManager
	config  *config.Config
}

func NewTransactionService(txRepo repository.TransactionRepository, accRepo repository.AccountRepository, tm repository.TransactionManager, cfg *config.Config) *TransactionService {
	return &TransactionService{txRepo: txRepo, accRepo: accRepo, tm: tm, config: cfg}
}

func (ts *TransactionService) GetTransactionRule(txType model.TransactionType) (model.TransactionRule, error) {
	switch txType {
	case model.TxTypeExpense:
		return model.TransactionRule{
			TxType:      model.TxTypeExpense,
			SourceTypes: []string{"A", "L"}, // Assets, Liabilities
			DestTypes:   []string{"E"},      // Expenses
		}, nil
	case model.TxTypeIncome:
		return model.TransactionRule{
			TxType:      model.TxTypeIncome,
			SourceTypes: []string{"R"},      // Revenue (Income)
			DestTypes:   []string{"A", "L"}, // Assets, Liabilities
		}, nil
	case model.TxTypeTransfer:
		return model.TransactionRule{
			TxType:      model.TxTypeTransfer,
			SourceTypes: []string{"A", "L"},
			DestTypes:   []string{"A", "L"},
		}, nil
	default:
		return model.TransactionRule{}, validationErrorf("type", "unknown transaction mode: %s", txType)
	}
}

// GetTransactionByID retrieves a transaction with all split details
func (ts *TransactionService) GetTransactionByID(ctx context.Context, txID int64) (*model.TransactionDetail, error) {
	tx, err := ts.txRepo.GetTransactionByID(ctx, txID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("transaction #%d: %w", txID, ErrNotFound)
		}
		return nil, fmt.Errorf("failed to retrieve transaction %d: %w", txID, err)
	}

	splits, err := ts.txRepo.GetSplitsWithAccountsByTransaction(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get splits for transaction: %w", err)
	}

	return &model.TransactionDetail{
		ID:          tx.ID,
		Timestamp:   tx.Timestamp,
		Description: tx.Description,
		Status:      tx.Status,
		Type:        tx.Type,
		Splits:      splits,
	}, nil
}

// GetTransactionDetailsByIDs fetches TransactionDetail for each transaction in txs using a single bulk repo call.
func (ts *TransactionService) GetTransactionDetailsByIDs(ctx context.Context, txs []*model.Transaction) (map[int64]*model.TransactionDetail, error) {
	if len(txs) == 0 {
		return make(map[int64]*model.TransactionDetail), nil
	}

	ids := make([]int64, len(txs))
	for i, tx := range txs {
		ids[i] = tx.ID
	}

	splitsMap, err := ts.txRepo.GetSplitsWithAccountsByTransactionIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk-fetch splits: %w", err)
	}

	result := make(map[int64]*model.TransactionDetail, len(txs))
	for _, tx := range txs {
		result[tx.ID] = &model.TransactionDetail{
			ID:          tx.ID,
			Timestamp:   tx.Timestamp,
			Description: tx.Description,
			Status:      tx.Status,
			Type:        tx.Type,
			Splits:      splitsMap[tx.ID],
		}
	}
	return result, nil
}

// GetRecentTransactions retrieves recent transactions across all accounts
func (ts *TransactionService) GetRecentTransactions(ctx context.Context, limit int) ([]*model.Transaction, error) {
	transactions, err := ts.txRepo.GetAllTransactions(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent transactions: %w", err)
	}
	return transactions, nil
}

// ListRecentTransactions returns a paginated list of all transactions.
func (ts *TransactionService) ListRecentTransactions(ctx context.Context, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	return ts.txRepo.ListTransactions(ctx, opts)
}

// ListTransactionHistory returns a paginated transaction history for the named account.
func (ts *TransactionService) ListTransactionHistory(ctx context.Context, accountName string, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	account, err := ts.accRepo.GetAccountByName(ctx, accountName)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}
	return ts.txRepo.ListTransactionsByAccount(ctx, account.ID, opts)
}

// FilterTransactions returns a paginated, multi-dimensionally filtered transaction list.
func (ts *TransactionService) FilterTransactions(ctx context.Context, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	return ts.txRepo.FilterTransactions(ctx, filter, opts)
}

// FilterTransactionsByAccountName resolves an account name to its ID, then delegates to FilterTransactions.
func (ts *TransactionService) FilterTransactionsByAccountName(ctx context.Context, accountName string, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error) {
	account, err := ts.accRepo.GetAccountByName(ctx, accountName)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}
	filter.AccountID = &account.ID
	return ts.txRepo.FilterTransactions(ctx, filter, opts)
}

// GetTransactionHistory retrieves transaction history for a specific account
func (ts *TransactionService) GetTransactionHistory(ctx context.Context, accountName string, limit int) ([]*model.Transaction, error) {
	// Get account by name
	account, err := ts.accRepo.GetAccountByName(ctx, accountName)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	// Get transactions for this account
	transactions, err := ts.txRepo.GetTransactionsByAccount(ctx, account.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}

	return transactions, nil
}

// GetMonthlyBalanceHistory returns, for every Asset and Liability account
// with activity, the full monthly end-of-month balance series in the
// account's natural-amount direction. Accounts with no activity are omitted.
// Series are ordered by AccountID ASC; points within each series ascend by month.
func (ts *TransactionService) GetMonthlyBalanceHistory(ctx context.Context) ([]model.AccountMonthlySeries, error) {
	rows, err := ts.txRepo.GetMonthlySplitTotalsForAssetsAndLiabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly split totals: %w", err)
	}

	// rows are already ordered by account_id, month ASC.
	var out []model.AccountMonthlySeries
	var cur *model.AccountMonthlySeries
	var running int64
	for _, r := range rows {
		if cur == nil || cur.AccountID != r.AccountID {
			out = append(out, model.AccountMonthlySeries{
				AccountID: r.AccountID,
				Currency:  r.Currency,
				Points:    nil,
			})
			cur = &out[len(out)-1]
			running = 0
		}
		running += r.Amount
		balance := running
		// Liability: stored is negative when the debt grows, so natural-amount = -stored.
		if r.AccountType == model.AccountTypeLiability {
			balance = -balance
		}
		cur.Points = append(cur.Points, model.MonthlyBalancePoint{
			Month:   r.Month,
			Balance: balance,
		})
	}
	return out, nil
}
