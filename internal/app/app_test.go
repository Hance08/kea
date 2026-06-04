// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
)

// newTestApp returns an *App wired to two on-disk ledgers ("a", "b"), both
// registered, with "a" active. tempDir holds ledgers.yaml plus the two DB
// files.
func newTestApp(t *testing.T) (a *App, tempDir string, pathA, pathB string) {
	t.Helper()
	tempDir = t.TempDir()
	pathA = filepath.Join(tempDir, "a.db")
	pathB = filepath.Join(tempDir, "b.db")

	if err := InitLedgerDB(pathA, migrations.FS); err != nil {
		t.Fatalf("init a: %v", err)
	}
	if err := InitLedgerDB(pathB, migrations.FS); err != nil {
		t.Fatalf("init b: %v", err)
	}

	reg, err := ledger.Load(tempDir)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	// ledger.Load auto-seeds a "default" entry; drop it so the test starts
	// with exactly "a" and "b".
	delete(reg.Ledgers, "default")
	reg.ActiveLedger = ""
	if err := reg.Add("a", pathA); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if err := reg.Add("b", pathB); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if err := reg.Switch("a"); err != nil {
		t.Fatalf("switch a: %v", err)
	}

	cfg := config.NewDefault()
	cfg.Database.Path = pathA
	cfg.ActiveLedger = "a"

	st, err := store.NewStore(pathA, migrations.FS)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	return &App{
		Registry:   reg,
		store:      st,
		migrations: migrations.FS,
		cfg:        cfg,
	}, tempDir, pathA, pathB
}

func TestSwitchLedger_SwapsStoreAndUpdatesConfig(t *testing.T) {
	a, _, _, pathB := newTestApp(t)

	if err := a.SwitchLedger("b"); err != nil {
		t.Fatalf("SwitchLedger: %v", err)
	}

	if a.cfg.ActiveLedger != "b" {
		t.Errorf("cfg.ActiveLedger: got %q, want %q", a.cfg.ActiveLedger, "b")
	}
	if a.cfg.Database.Path != pathB {
		t.Errorf("cfg.Database.Path: got %q, want %q", a.cfg.Database.Path, pathB)
	}
	if a.Registry.ActiveLedger != "b" {
		t.Errorf("registry.ActiveLedger: got %q, want %q", a.Registry.ActiveLedger, "b")
	}

	// Verify the store now talks to pathB by writing through the swapped
	// connection.
	_, err := a.store.DB().ExecContext(t.Context(),
		"INSERT INTO accounts (name, type, currency) VALUES (?, ?, ?)",
		"Assets:Probe", "A", "USD",
	)
	if err != nil {
		t.Fatalf("insert into swapped store: %v", err)
	}
}

func TestSwitchLedger_UnknownNameReturnsErrLedgerNotFound(t *testing.T) {
	a, _, pathA, _ := newTestApp(t)

	err := a.SwitchLedger("nope")
	if !errors.Is(err, ledger.ErrLedgerNotFound) {
		t.Fatalf("err: got %v, want ErrLedgerNotFound", err)
	}
	if a.cfg.ActiveLedger != "a" {
		t.Errorf("cfg.ActiveLedger leaked: got %q, want %q", a.cfg.ActiveLedger, "a")
	}
	if a.cfg.Database.Path != pathA {
		t.Errorf("cfg.Database.Path leaked: got %q, want %q", a.cfg.Database.Path, pathA)
	}
	if a.Registry.ActiveLedger != "a" {
		t.Errorf("registry.ActiveLedger leaked: got %q, want %q", a.Registry.ActiveLedger, "a")
	}
}

func TestSwitchLedger_FailedSwapLeavesStateUnchanged(t *testing.T) {
	a, tempDir, pathA, _ := newTestApp(t)

	// Register a ledger whose path points at a file used as the parent
	// directory; os.MkdirAll will fail, so NewStore returns an error and Swap
	// never mutates state. The corresponding store-layer behavior is pinned by
	// store.TestSwap_FailedSwapKeepsOldConnection.
	blockingFile := filepath.Join(tempDir, "not-a-dir")
	if f, err := os.Create(blockingFile); err != nil {
		t.Fatalf("create blocking file: %v", err)
	} else {
		_ = f.Close()
	}
	badPath := filepath.Join(blockingFile, "bad.db")
	if err := a.Registry.Add("bad", badPath); err != nil {
		t.Fatalf("add bad: %v", err)
	}

	err := a.SwitchLedger("bad")
	if err == nil {
		t.Fatal("expected error from SwitchLedger to nonexistent path")
	}
	if a.cfg.ActiveLedger != "a" {
		t.Errorf("cfg.ActiveLedger leaked: got %q, want %q", a.cfg.ActiveLedger, "a")
	}
	if a.cfg.Database.Path != pathA {
		t.Errorf("cfg.Database.Path leaked: got %q, want %q", a.cfg.Database.Path, pathA)
	}
	if a.Registry.ActiveLedger != "a" {
		t.Errorf("registry.ActiveLedger leaked: got %q, want %q", a.Registry.ActiveLedger, "a")
	}
}
