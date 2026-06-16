// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package repository

import (
	"context"

	"github.com/hance08/kea/internal/model"
)

type AccountRepository interface {
	CreateAccount(ctx context.Context, name string, accType model.AccountType, currency, description string, parentID *int64) (int64, error)
	GetAllAccounts(ctx context.Context) ([]*model.Account, error)
	GetAccountByName(ctx context.Context, name string) (*model.Account, error)
	GetAccountByID(ctx context.Context, id int64) (*model.Account, error)
	AccountExists(ctx context.Context, name string) (bool, error)
	GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error)
	GetAccountBalance(ctx context.Context, accountID int64) (int64, error)
	GetAllAccountBalances(ctx context.Context, asOf int64) (map[int64]int64, error)
	HasChildAccounts(ctx context.Context, accountID int64) (bool, error)
	AccountHasTransactions(ctx context.Context, accountID int64) (bool, error)
	DeleteAccount(ctx context.Context, accountID int64) error
	RenameAccount(ctx context.Context, oldName, newName string) error
	UpdateAccountMetadata(ctx context.Context, accountID int64, description string, isHidden bool) error
	SearchAccounts(ctx context.Context, filter model.AccountFilter, opts model.ListOptions) (*model.ListResult[*model.Account], error)
}

type TransactionRepository interface {
	CreateTransactionWithSplits(ctx context.Context, tx model.Transaction, splits []model.Split) (int64, error)
	GetTransactionByID(ctx context.Context, txID int64) (*model.Transaction, error)
	GetTransactionsByAccount(ctx context.Context, accountID int64, limit int) ([]*model.Transaction, error)
	GetTransactionsByDateRange(ctx context.Context, startTime, endTime int64) ([]*model.Transaction, error)
	GetAllTransactions(ctx context.Context, limit int) ([]*model.Transaction, error)

	UpdateTransactionStatus(ctx context.Context, txID int64, status model.TransactionStatus) error
	DeleteTransaction(ctx context.Context, txID int64) error
	UpdateTransactionBasic(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) error

	CreateSplit(ctx context.Context, txID int64, split *model.Split) (int64, error)
	UpdateSplit(ctx context.Context, splitID int64, accountID int64, amount int64, currency string, memo string) error
	DeleteSplit(ctx context.Context, splitID int64) error
	GetSplitsByTransaction(ctx context.Context, txID int64) ([]*model.Split, error)

	// GetSplitsWithAccountsByDateRange returns all splits (with account info) for
	// transactions in the given Unix time range, keyed by transaction ID.
	// Designed for bulk report queries to avoid N+1 patterns.
	GetSplitsWithAccountsByDateRange(ctx context.Context, startTime, endTime int64) (map[int64][]model.SplitDetail, error)

	// GetSplitsWithAccountsByTransaction returns all splits (with account info) for
	// a single transaction in one JOIN query.
	GetSplitsWithAccountsByTransaction(ctx context.Context, txID int64) ([]model.SplitDetail, error)

	// GetSplitsWithAccountsByTransactionIDs returns all splits (with account info)
	// for the given transaction IDs in a single query, keyed by transaction ID.
	GetSplitsWithAccountsByTransactionIDs(ctx context.Context, txIDs []int64) (map[int64][]model.SplitDetail, error)

	// GetUnreconciledTransactionsByAccount returns all Pending and Cleared
	// transactions that have a split touching accountID, together with the
	// net split amount for that account. Ordered by timestamp ASC.
	GetUnreconciledTransactionsByAccount(ctx context.Context, accountID int64) ([]*model.ReconcileEntry, error)

	// BulkUpdateTransactionStatus sets status on all listed transaction IDs.
	// Large ID lists are chunked internally. Returns an error if the affected
	// row count does not match len(txIDs).
	BulkUpdateTransactionStatus(ctx context.Context, txIDs []int64, status model.TransactionStatus) error

	// MarkSplitsReconciledByAccount marks the splits for accountID in each of
	// the listed transactions as reconciled. If all splits in a transaction are
	// now reconciled the transaction's status is upgraded to StatusReconciled.
	MarkSplitsReconciledByAccount(ctx context.Context, accountID int64, txIDs []int64) (int64, error)

	// GetLastReconciledBalance returns the running reconciled balance for
	// accountID as persisted after the most recent successful reconcile.
	// Returns 0 if the account has never been reconciled.
	GetLastReconciledBalance(ctx context.Context, accountID int64) (int64, error)

	// SetLastReconciledBalance persists the new running reconciled balance for
	// accountID. Uses an upsert so the first call creates the row.
	SetLastReconciledBalance(ctx context.Context, accountID int64, balance int64) error

	// ListTransactions returns a paginated list of all transactions ordered by
	// timestamp DESC, id DESC. When opts.IncludeCount is true the result also
	// carries the total row count.
	ListTransactions(ctx context.Context, opts model.ListOptions) (*model.ListResult[*model.Transaction], error)

	// ListTransactionsByAccount returns a paginated list of transactions that
	// have at least one split touching accountID, ordered by timestamp DESC,
	// id DESC. When opts.IncludeCount is true the result also carries the total
	// distinct transaction count.
	ListTransactionsByAccount(ctx context.Context, accountID int64, opts model.ListOptions) (*model.ListResult[*model.Transaction], error)

	// FilterTransactions returns a paginated list of transactions matching the
	// given filter criteria. Nil filter fields are ignored (no filtering on
	// that dimension). When filter.AccountID is set the query joins on splits.
	// Results are ordered by timestamp DESC, id DESC.
	FilterTransactions(ctx context.Context, filter model.TransactionFilter, opts model.ListOptions) (*model.ListResult[*model.Transaction], error)

	// GetMonthlySplitTotalsForAssetsAndLiabilities returns, for every
	// (account_id, month) with activity on an Asset or Liability account,
	// the sum of split amounts in that month. Months are UTC "YYYY-MM"
	// strings derived from transaction timestamps. Ordered by account_id
	// ASC, month ASC. Used to build per-account monthly balance series.
	GetMonthlySplitTotalsForAssetsAndLiabilities(ctx context.Context) ([]MonthlySplitTotal, error)
}

type Repository interface {
	AccountRepository
	TransactionRepository
}

type TransactionManager interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
}

// MonthlySplitTotal is one (account, month) bucket of summed split amounts.
// Amount is stored-sign (not natural-amount); the service layer applies the
// natural-amount conversion based on AccountType.
type MonthlySplitTotal struct {
	AccountID   int64
	AccountType model.AccountType
	Currency    string
	Month       string // "YYYY-MM" UTC
	Amount      int64  // cents, stored sign, sum within the month
}
