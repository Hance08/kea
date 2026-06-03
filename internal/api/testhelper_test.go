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

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(dbPath, migrations.FS)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := config.NewDefault()
	cfg.Defaults.Currency = "USD"

	svc := service.NewService(st, st, st, cfg)

	srv := NewServer(cfg, svc, discardLogger())
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

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

// seedTransaction creates a simple two-split transaction and returns its detail.
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
	return d
}
