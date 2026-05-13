// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupOnline_CopiesData(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")

	srcDB, err := sql.Open("sqlite3", srcPath+"?_journal_mode=WAL")
	require.NoError(t, err)
	defer srcDB.Close()

	_, err = srcDB.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	_, err = srcDB.Exec("INSERT INTO items (name) VALUES ('alpha'), ('beta')")
	require.NoError(t, err)

	dstPath := filepath.Join(dir, "backup.db")
	err = backupOnline(context.Background(), srcDB, dstPath)
	require.NoError(t, err)

	_, err = os.Stat(dstPath)
	require.NoError(t, err)

	dstDB, err := sql.Open("sqlite3", dstPath)
	require.NoError(t, err)
	defer dstDB.Close()

	var count int
	err = dstDB.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestBackupOnline_AtomicWithTmpFile(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")

	srcDB, err := sql.Open("sqlite3", srcPath)
	require.NoError(t, err)
	defer srcDB.Close()
	_, err = srcDB.Exec("CREATE TABLE t (id INTEGER)")
	require.NoError(t, err)

	dstPath := filepath.Join(dir, "backup.db")
	err = backupOnline(context.Background(), srcDB, dstPath)
	require.NoError(t, err)

	_, err = os.Stat(dstPath + ".tmp")
	assert.True(t, os.IsNotExist(err), "temp file should be cleaned up")
}

func TestBackupOnline_FailsOnBadDest(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")

	srcDB, err := sql.Open("sqlite3", srcPath)
	require.NoError(t, err)
	defer srcDB.Close()
	_, err = srcDB.Exec("CREATE TABLE t (id INTEGER)")
	require.NoError(t, err)

	dstPath := filepath.Join(dir, "no-such-dir", "backup.db")
	err = backupOnline(context.Background(), srcDB, dstPath)
	assert.Error(t, err)
}

func TestBackupOnline_ConsistentDuringWrites(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")

	srcDB, err := sql.Open("sqlite3", srcPath+"?_journal_mode=WAL&_busy_timeout=5000")
	require.NoError(t, err)
	defer srcDB.Close()

	_, err = srcDB.Exec("CREATE TABLE counter (id INTEGER PRIMARY KEY, n INTEGER)")
	require.NoError(t, err)
	_, err = srcDB.Exec("INSERT INTO counter (n) VALUES (0)")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				srcDB.Exec("UPDATE counter SET n = n + 1 WHERE id = 1")
			}
		}
	}()

	dstPath := filepath.Join(dir, "backup.db")
	err = backupOnline(context.Background(), srcDB, dstPath)
	cancel()
	<-done
	require.NoError(t, err)

	dstDB, err := sql.Open("sqlite3", dstPath)
	require.NoError(t, err)
	defer dstDB.Close()

	var n int
	err = dstDB.QueryRow("SELECT n FROM counter WHERE id = 1").Scan(&n)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 0, "counter should have a valid value")

	var result string
	err = dstDB.QueryRow("PRAGMA integrity_check").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}
