// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore_SetsMaxOpenConns(t *testing.T) {
	s := setupTestDB(t)
	assert.Equal(t, 4, s.DB().Stats().MaxOpenConnections)
}

func TestNewStore_UsesTxLockImmediate(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	db := s.DB()

	// Acquire two dedicated connections from the pool so we can control each independently.
	conn1, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn1.Close()

	conn2, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn2.Close()

	// Set busy_timeout to 0 on each pinned connection so BeginTx fails immediately.
	_, err = conn1.ExecContext(ctx, "PRAGMA busy_timeout = 0")
	require.NoError(t, err)
	_, err = conn2.ExecContext(ctx, "PRAGMA busy_timeout = 0")
	require.NoError(t, err)

	// Start a write transaction on connection 1
	tx1, err := conn1.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx1.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS _txlock_test (id INTEGER)")
	require.NoError(t, err)

	// With _txlock=immediate, a second write transaction should fail
	// immediately (SQLITE_BUSY) rather than succeeding and failing later.
	tx2, err := conn2.BeginTx(ctx, nil)
	if err == nil {
		_ = tx2.Rollback()
	}
	// The second BEGIN IMMEDIATE should return busy error since tx1 holds the write lock.
	require.Error(t, err, "second immediate transaction should fail while first holds write lock")

	_ = tx1.Rollback()
}

func TestNewStore_EnablesWALMode(t *testing.T) {
	s := setupTestDB(t)

	var journalMode string
	err := s.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journalMode)
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode)
}

func TestNewStore_SetsBusyTimeout(t *testing.T) {
	s := setupTestDB(t)

	var timeout int
	err := s.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&timeout)
	require.NoError(t, err)
	assert.Equal(t, 5000, timeout)
}
