// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
)

// newServerWithStore builds a *Server backed by an in-memory SQLite store and
// returns an httptest.Server fronting its routes plus the *service.Service for
// seeding.
func newServerWithStore(t *testing.T) (*httptest.Server, *service.Service) {
	t.Helper()
	ts, svc, _ := newServerForWrite(t)
	return ts, svc
}

// seedAccount creates a leaf account and returns it.
func seedAccount(t *testing.T, svc *service.Service, name string, accType model.AccountType, balance int64) *model.Account {
	t.Helper()
	acc, err := svc.Account().CreateAccountWithBalance(t.Context(), model.CreateAccountInput{
		Name:     name,
		Type:     accType,
		Currency: "USD",
		Balance:  balance,
	})
	if err != nil {
		t.Fatalf("seedAccount %q: %v", name, err)
	}
	return acc
}

// seedTransaction creates a simple two-split transaction and returns its full detail
// (including split IDs and account IDs resolved from the database).
func seedTransaction(t *testing.T, svc *service.Service, from, to string, amount int64, timestamp int64, description string, txType model.TransactionType, status model.TransactionStatus) model.TransactionDetail {
	t.Helper()
	d, err := svc.Transaction().CreateSimpleTransaction(t.Context(), model.CreateSimpleTransactionInput{
		FromAccount: from,
		ToAccount:   to,
		Amount:      amount,
		Timestamp:   timestamp,
		Description: description,
		Type:        txType,
		Status:      status,
	})
	if err != nil {
		t.Fatalf("seedTransaction %q->%q: %v", from, to, err)
	}
	// Fetch full detail so that Splits carry database-assigned IDs and AccountIDs.
	full, err := svc.Transaction().GetTransactionByID(t.Context(), d.ID)
	if err != nil {
		t.Fatalf("seedTransaction get full detail: %v", err)
	}
	return *full
}

// newServerForWrite is a variant of newServerWithStore that also returns the
// underlying *store.Store, so write tests can manipulate reconcile state and
// inject a parent cycle directly via the repo (bypassing the service layer
// where convenient).
func newServerForWrite(t *testing.T) (*httptest.Server, *service.Service, *store.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(dbPath, migrations.FS)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := config.NewDefault()
	cfg.Defaults.Currency = "USD"

	svc := service.NewService(st, st, st, cfg)
	srv := NewServer(cfg, svc, nil, nil, "", nil, discardLogger())
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

	return ts, svc, st
}

// seedReconciledTransaction flips a Cleared transaction to Reconciled by
// directly invoking the reconcile-aware repo methods. Mirrors what
// service.ReconcileTransactions does internally, minus the balance bookkeeping
// (which write-endpoint tests do not need).
func seedReconciledTransaction(t *testing.T, st *store.Store, accountID int64, txID int64) {
	t.Helper()
	if _, err := st.MarkSplitsReconciledByAccount(t.Context(), accountID, []int64{txID}); err != nil {
		t.Fatalf("MarkSplitsReconciledByAccount: %v", err)
	}
	if err := st.BulkUpdateTransactionStatus(t.Context(), []int64{txID}, model.StatusReconciled); err != nil {
		t.Fatalf("BulkUpdateTransactionStatus: %v", err)
	}
}

// injectParentSelfCycle directly sets accounts.parent_id = id on the given
// account, going around the service-layer cycle validation. Used to exercise
// the ErrCircularParent code path from API tests.
func injectParentSelfCycle(t *testing.T, st *store.Store, accountID int64) {
	t.Helper()
	if _, err := st.DB().ExecContext(t.Context(),
		"UPDATE accounts SET parent_id = ? WHERE id = ?", accountID, accountID); err != nil {
		t.Fatalf("inject cycle: %v", err)
	}
}

func TestNewServerForWrite_ExposesStore(t *testing.T) {
	_, _, st := newServerForWrite(t)
	if st == nil {
		t.Fatal("store is nil")
	}
}

func TestSeedReconciledTransaction(t *testing.T) {
	_, svc, st := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 10000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	seedReconciledTransaction(t, st, src.ID, d.ID)

	got, err := svc.Transaction().GetTransactionByID(t.Context(), d.ID)
	if err != nil {
		t.Fatalf("get tx: %v", err)
	}
	if got.Status != model.StatusReconciled {
		t.Fatalf("status: got %v, want Reconciled", got.Status)
	}
}

func TestInjectParentSelfCycle(t *testing.T) {
	_, svc, st := newServerForWrite(t)
	acc := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

	injectParentSelfCycle(t, st, acc.ID)

	got, err := svc.Account().GetAccountByID(t.Context(), acc.ID)
	if err != nil {
		t.Fatalf("get acc: %v", err)
	}
	if got.ParentID == nil || *got.ParentID != acc.ID {
		t.Fatalf("parent_id: got %v, want self-pointer (%d)", got.ParentID, acc.ID)
	}
}
