// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/hance08/kea/internal/repository"
	_ "github.com/mattn/go-sqlite3"
)

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Store struct {
	db    DBTX
	rawDB *sql.DB
}

func NewStore(dbPath string, migrationsFS fs.FS) (*Store, error) {
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("can not create database directory %s: %w", dbDir, err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000")
	success := false
	defer func() {
		if !success {
			_ = db.Close()
		}
	}()

	if err != nil {
		return nil, fmt.Errorf("can not open database : %w", err)
	}

	db.SetMaxOpenConns(1)

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("can not connect with database : %w", err)
	}
	if err := runMigrations(db, migrationsFS); err != nil {
		return nil, fmt.Errorf("failed to migrate database : %w", err)
	}

	success = true
	return &Store{db: db, rawDB: db}, nil
}

func (s *Store) ExecTx(ctx context.Context, fn func(repository.Repository) error) error {
	db, ok := s.db.(*sql.DB)
	if !ok {
		return fmt.Errorf("store is already in a transaction")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	txStore := &Store{db: tx}

	err = fn(txStore)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

func (s *Store) Close() error {
	if db, ok := s.db.(*sql.DB); ok {
		return db.Close()
	}
	return nil
}

// DB returns the underlying *sql.DB. Returns nil for transaction-scoped Stores.
func (s *Store) DB() *sql.DB {
	return s.rawDB
}

func runMigrations(db *sql.DB, migrationsFS fs.FS) error {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{NoTxWrap: true})
	if err != nil {
		return fmt.Errorf("failed to set up migrate driver : %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("failed to create iofs source driver : %w", err)
	}

	defer func() {
		_ = sourceDriver.Close()
	}()

	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		"sqlite3",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to set up migrate instance : %w", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migration(up) : %w", err)
	}

	return nil
}
