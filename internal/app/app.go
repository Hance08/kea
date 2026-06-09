// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/hance08/kea/internal/backup"
	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/store"
)

type App struct {
	Service    *service.Service
	Registry   *ledger.Registry
	store      *store.Store
	migrations fs.FS
	cfg        *config.Config

	runtimeMu sync.RWMutex
	runtime   RuntimeState
}

// RuntimeState is a value-type snapshot of the active ledger name and the
// database path the store is currently open against. Returned by value from
// App.RuntimeState so callers get a consistent pair under one lock.
type RuntimeState struct {
	ActiveLedger string
	DatabasePath string
}

// RuntimeState returns a snapshot of the currently-active ledger name and
// the database path the store is open against. Safe for concurrent callers.
func (a *App) RuntimeState() RuntimeState {
	a.runtimeMu.RLock()
	defer a.runtimeMu.RUnlock()
	return a.runtime
}

// setRuntime atomically replaces the runtime state. Internal — called from
// NewApp's OnSwitch callback and from SwitchLedger.
func (a *App) setRuntime(s RuntimeState) {
	a.runtimeMu.Lock()
	a.runtime = s
	a.runtimeMu.Unlock()
}

// NewApp initialize config, database and core logic, then return App entity
func NewApp(cfg *config.Config, registry *ledger.Registry, migrationFS fs.FS) (*App, func(), error) {
	dbPath, err := registry.Active()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve active ledger: %w", err)
	}

	if err := backup.Run(dbPath, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: backup failed: %v\n", err)
	}

	dbStore, err := store.NewStore(dbPath, migrationFS)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	svc := service.NewService(dbStore, dbStore, dbStore, cfg)

	app := &App{
		Service:    svc,
		Registry:   registry,
		store:      dbStore,
		migrations: migrationFS,
		cfg:        cfg,
		runtime:    RuntimeState{ActiveLedger: registry.ActiveName(), DatabasePath: dbPath},
	}

	registry.OnSwitch(func(name, path string) {
		if err := dbStore.Swap(path, migrationFS); err != nil {
			fmt.Fprintf(os.Stderr, "ledger switch failed: %v\n", err)
			return
		}
		app.setRuntime(RuntimeState{ActiveLedger: name, DatabasePath: path})
	})

	cleanup := func() {
		registry.StopWatch()
		if err := dbStore.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing DB: %v\n", err)
		}
	}

	return app, cleanup, nil
}

func GetAppDataDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("unable to determine user home directory: %w", err)
		}
		return filepath.Join(home, ".kea"), nil
	}

	return filepath.Join(configDir, "kea"), nil
}

// SwitchLedger swaps the active store to the named ledger, updates
// RuntimeState, and persists the active-name change to ledgers.yaml. The
// registry's fsnotify watcher subsequently fires reload(), which short-
// circuits because the active name already matches. If the final persist
// fails the runtime state still reflects the new ledger; the next process
// start reseeds from the registry on disk.
func (a *App) SwitchLedger(name string) error {
	entry, ok := a.Registry.EntryFor(name)
	if !ok {
		return fmt.Errorf("%w: %q", ledger.ErrLedgerNotFound, name)
	}
	if err := a.store.Swap(entry.Path, a.migrations); err != nil {
		return fmt.Errorf("swap store: %w", err)
	}
	a.setRuntime(RuntimeState{ActiveLedger: name, DatabasePath: entry.Path})
	return a.Registry.Switch(name)
}

// Config returns the config in use. Used by callers that build subcommands
// from *App and need the same cfg pointer NewApp captured.
func (a *App) Config() *config.Config { return a.cfg }

func InitLedgerDB(path string, migrations fs.FS) error {
	s, err := store.NewStore(path, migrations)
	if err != nil {
		return err
	}
	return s.Close()
}
