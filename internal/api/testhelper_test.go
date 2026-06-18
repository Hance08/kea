// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/ledger"
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

// newServerForWriteWithCurrency is a variant of newServerForWrite that lets the
// caller choose cfg.Defaults.Currency. Used by /api/config tests that exercise
// both populated and empty defaults.
func newServerForWriteWithCurrency(t *testing.T, currency string) (*httptest.Server, *service.Service, *store.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(dbPath, migrations.FS)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := config.NewDefault()
	cfg.Defaults.Currency = currency

	svc := service.NewService(st, st, st, cfg)
	srv := NewServer(cfg, svc, nil, nil, "", nil, func() error { return nil }, discardLogger())
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

	return ts, svc, st
}

// newServerForWriteWithDisplay is a variant of newServerForWriteWithCurrency
// that also lets the caller set cfg.Display.HideDecimals. Used by /api/config
// tests that exercise the display option.
func newServerForWriteWithDisplay(t *testing.T, currency string, hideDecimals bool) (*httptest.Server, *service.Service, *store.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(dbPath, migrations.FS)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := config.NewDefault()
	cfg.Defaults.Currency = currency
	cfg.Display.HideDecimals = hideDecimals

	svc := service.NewService(st, st, st, cfg)
	srv := NewServer(cfg, svc, nil, nil, "", nil, func() error { return nil }, discardLogger())
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

	return ts, svc, st
}

// newServerForPatchConfig builds a server whose saveConfig closure records
// every call into the returned counter and returns the *config.Config so
// tests can assert in-memory mutation independently of persistence. The
// saveConfig stub returns nil unless tests inject an error via *saveErr.
func newServerForPatchConfig(t *testing.T, currency string, hideDecimals bool) (
	ts *httptest.Server,
	cfg *config.Config,
	saveCalls *int,
	saveErr *error,
) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(dbPath, migrations.FS)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg = config.NewDefault()
	cfg.Defaults.Currency = currency
	cfg.Display.HideDecimals = hideDecimals

	svc := service.NewService(st, st, st, cfg)

	calls := 0
	var injErr error
	save := func() error {
		calls++
		return injErr
	}
	srv := NewServer(cfg, svc, nil, nil, "", nil, save, discardLogger())
	ts = httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	return ts, cfg, &calls, &injErr
}

// newServerForWrite is a variant of newServerWithStore that also returns the
// underlying *store.Store, so write tests can manipulate reconcile state and
// inject a parent cycle directly via the repo (bypassing the service layer
// where convenient).
func newServerForWrite(t *testing.T) (*httptest.Server, *service.Service, *store.Store) {
	t.Helper()
	return newServerForWriteWithCurrency(t, "USD")
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

// newTestServerWithLedger wires a fresh Server backed by a real on-disk
// *ledger.Registry plus a fake switchLedger fn that records calls and
// updates the registry's active name without invoking dbStore.Swap.
//
// The real store-swap behavior is exercised in internal/app/app_test.go;
// duplicating it at the HTTP boundary does not pin additional behavior.
func newTestServerWithLedger(t *testing.T) (
	ts *httptest.Server,
	reg *ledger.Registry,
	appDir string,
	switchCalls *[]string,
) {
	t.Helper()

	appDir = t.TempDir()

	var err error
	reg, err = ledger.Load(appDir)
	if err != nil {
		t.Fatalf("ledger.Load: %v", err)
	}

	// ledger.Load auto-creates a "default" entry pointing at <appDir>/kea.db
	// and sets it active. Remove it so tests start from a clean slate; tests
	// that need a default-active ledger seed one explicitly.
	delete(reg.Ledgers, "default")
	reg.ActiveLedger = ""
	if err := reg.Save(); err != nil {
		t.Fatalf("clear default: %v", err)
	}

	calls := []string{}
	switchLedger := func(name string) error {
		calls = append(calls, name)
		if _, ok := reg.EntryFor(name); !ok {
			return fmt.Errorf("%w: %q", ledger.ErrLedgerNotFound, name)
		}
		return reg.Switch(name)
	}

	cfg := config.NewDefault()
	srv := NewServer(cfg, nil, reg, migrations.FS, appDir, switchLedger, func() error { return nil }, discardLogger())
	ts = httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

	return ts, reg, appDir, &calls
}

func TestNewTestServerWithLedger_StartsClean(t *testing.T) {
	_, reg, appDir, calls := newTestServerWithLedger(t)
	if reg == nil {
		t.Fatal("registry is nil")
	}
	if got := reg.ActiveName(); got != "" {
		t.Errorf("ActiveName: got %q, want \"\"", got)
	}
	if len(reg.Names()) != 0 {
		t.Errorf("Names: got %v, want empty", reg.Names())
	}
	if appDir == "" {
		t.Error("appDir is empty")
	}
	if calls == nil {
		t.Error("switchCalls slice is nil")
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
