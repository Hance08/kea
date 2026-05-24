// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package backup

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// fakeClock lets tests control "now".
type fakeClock struct{ t time.Time }

func (f fakeClock) Now() time.Time { return f.t }

func fixedTime(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// TestRun_NoDBFile: when the DB does not exist, Run is a no-op.
func TestRun_NoDBFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")

	err := run(dbPath, fakeClock{t: fixedTime("2026-04-14")}, nil)

	require.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(dir, "backups"))
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "backups/ dir should not be created when DB is absent")
}

func TestDailyLabel(t *testing.T) {
	assert.Equal(t, "2026-04-14", dailyLabel(fixedTime("2026-04-14")))
	assert.Equal(t, "2026-01-01", dailyLabel(fixedTime("2026-01-01")))
}

func TestWeeklyLabel(t *testing.T) {
	// 2026-04-14 is in ISO week 16 of 2026
	assert.Equal(t, "2026-W16", weeklyLabel(fixedTime("2026-04-14")))
	// 2026-01-01 is in ISO week 1 of 2026
	assert.Equal(t, "2026-W01", weeklyLabel(fixedTime("2026-01-01")))
}

func TestMonthlyLabel(t *testing.T) {
	assert.Equal(t, "2026-04", monthlyLabel(fixedTime("2026-04-14")))
	assert.Equal(t, "2026-01", monthlyLabel(fixedTime("2026-01-01")))
}

func TestBackupFilename(t *testing.T) {
	assert.Equal(t, "kea_daily_2026-04-14.db", backupFilename("kea", "daily", "2026-04-14", ".db"))
	assert.Equal(t, "kea_weekly_2026-W16.db", backupFilename("kea", "weekly", "2026-W16", ".db"))
	assert.Equal(t, "kea_monthly_2026-04.db", backupFilename("kea", "monthly", "2026-04", ".db"))
}

func TestRotate_PrunesOldestBeyondRetention(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	// Create 9 daily backup files (retention is 7).
	files := []string{
		"kea_daily_2026-04-06.db",
		"kea_daily_2026-04-07.db",
		"kea_daily_2026-04-08.db",
		"kea_daily_2026-04-09.db",
		"kea_daily_2026-04-10.db",
		"kea_daily_2026-04-11.db",
		"kea_daily_2026-04-12.db",
		"kea_daily_2026-04-13.db",
		"kea_daily_2026-04-14.db",
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(backupDir, f), []byte("x"), 0644))
	}

	require.NoError(t, rotate(backupDir, "kea", "daily", ".db", 7))

	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Len(t, entries, 7)
	// Oldest two should be gone.
	_, err = os.Stat(filepath.Join(backupDir, "kea_daily_2026-04-06.db"))
	assert.True(t, errors.Is(err, os.ErrNotExist))
	_, err = os.Stat(filepath.Join(backupDir, "kea_daily_2026-04-07.db"))
	assert.True(t, errors.Is(err, os.ErrNotExist))
	// Newest should remain.
	_, err = os.Stat(filepath.Join(backupDir, "kea_daily_2026-04-14.db"))
	assert.NoError(t, err)
}

func TestRotate_NoOpWhenUnderRetention(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	// Create 3 daily files (retention is 7) — nothing should be deleted.
	for _, f := range []string{"kea_daily_2026-04-12.db", "kea_daily_2026-04-13.db", "kea_daily_2026-04-14.db"} {
		require.NoError(t, os.WriteFile(filepath.Join(backupDir, f), []byte("x"), 0644))
	}

	require.NoError(t, rotate(backupDir, "kea", "daily", ".db", 7))

	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestRotate_OnlyTouchesMatchingTier(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	// 9 daily + 1 weekly — rotate daily with retention 7, weekly must be untouched.
	for i := 6; i <= 14; i++ {
		name := fmt.Sprintf("kea_daily_2026-04-%02d.db", i)
		require.NoError(t, os.WriteFile(filepath.Join(backupDir, name), []byte("x"), 0644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "kea_weekly_2026-W15.db"), []byte("x"), 0644))

	require.NoError(t, rotate(backupDir, "kea", "daily", ".db", 7))

	_, err := os.Stat(filepath.Join(backupDir, "kea_weekly_2026-W15.db"))
	assert.NoError(t, err, "weekly backup must not be deleted by daily rotation")
}

// mkDB creates a real SQLite database file at dbPath.
func mkDB(t *testing.T, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE _backup_marker (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	require.NoError(t, db.Close())
}

func TestRun_FirstRun_AllThreeTiersCreated(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")
	mkDB(t, dbPath)
	clk := fakeClock{t: fixedTime("2026-04-14")}

	require.NoError(t, run(dbPath, clk, nil))

	backupDir := filepath.Join(dir, "backups")
	assertFileExists(t, backupDir, "kea_daily_2026-04-14.db")
	assertFileExists(t, backupDir, "kea_weekly_2026-W16.db")
	assertFileExists(t, backupDir, "kea_monthly_2026-04.db")
}

func TestRun_AlreadyCurrent_NoNewFiles(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")
	mkDB(t, dbPath)
	backupDir := filepath.Join(dir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	// Pre-populate all three tiers for today.
	clk := fakeClock{t: fixedTime("2026-04-14")}
	for _, name := range []string{
		"kea_daily_2026-04-14.db",
		"kea_weekly_2026-W16.db",
		"kea_monthly_2026-04.db",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(backupDir, name), []byte("old"), 0644))
	}

	require.NoError(t, run(dbPath, clk, nil))

	// Files should still have "old" contents — not overwritten.
	for _, name := range []string{
		"kea_daily_2026-04-14.db",
		"kea_weekly_2026-W16.db",
		"kea_monthly_2026-04.db",
	} {
		got, err := os.ReadFile(filepath.Join(backupDir, name))
		require.NoError(t, err)
		assert.Equal(t, []byte("old"), got, "file %s should not be overwritten", name)
	}
}

func TestRun_DailyDue_OtherTiersCurrent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")
	mkDB(t, dbPath)
	backupDir := filepath.Join(dir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	clk := fakeClock{t: fixedTime("2026-04-14")}
	// Weekly and monthly already present for this period; daily is missing.
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "kea_weekly_2026-W16.db"), []byte("old"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "kea_monthly_2026-04.db"), []byte("old"), 0644))

	require.NoError(t, run(dbPath, clk, nil))

	assertFileExists(t, backupDir, "kea_daily_2026-04-14.db")
	// Others untouched.
	weekly, _ := os.ReadFile(filepath.Join(backupDir, "kea_weekly_2026-W16.db"))
	assert.Equal(t, []byte("old"), weekly)
}

func TestRun_WeeklyDue_OtherTiersCurrent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")
	mkDB(t, dbPath)
	backupDir := filepath.Join(dir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	clk := fakeClock{t: fixedTime("2026-04-14")}
	// Daily and monthly present; weekly is missing.
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "kea_daily_2026-04-14.db"), []byte("old"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "kea_monthly_2026-04.db"), []byte("old"), 0644))

	require.NoError(t, run(dbPath, clk, nil))

	assertFileExists(t, backupDir, "kea_weekly_2026-W16.db")
	daily, _ := os.ReadFile(filepath.Join(backupDir, "kea_daily_2026-04-14.db"))
	assert.Equal(t, []byte("old"), daily, "daily should not be overwritten")
}

func TestRun_MonthlyDue_OtherTiersCurrent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")
	mkDB(t, dbPath)
	backupDir := filepath.Join(dir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	clk := fakeClock{t: fixedTime("2026-04-14")}
	// Daily and weekly present; monthly is missing.
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "kea_daily_2026-04-14.db"), []byte("old"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "kea_weekly_2026-W16.db"), []byte("old"), 0644))

	require.NoError(t, run(dbPath, clk, nil))

	assertFileExists(t, backupDir, "kea_monthly_2026-04.db")
	daily, _ := os.ReadFile(filepath.Join(backupDir, "kea_daily_2026-04-14.db"))
	assert.Equal(t, []byte("old"), daily, "daily should not be overwritten")
}

func TestRun_RotationTriggered(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")
	mkDB(t, dbPath)
	backupDir := filepath.Join(dir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	// Create 7 existing daily backups (at retention limit).
	for i := 7; i <= 13; i++ {
		name := fmt.Sprintf("kea_daily_2026-04-%02d.db", i)
		require.NoError(t, os.WriteFile(filepath.Join(backupDir, name), []byte("x"), 0644))
	}

	clk := fakeClock{t: fixedTime("2026-04-14")}
	require.NoError(t, run(dbPath, clk, nil))

	entries, _ := os.ReadDir(backupDir)
	var dailyCount int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "kea_daily_") {
			dailyCount++
		}
	}
	assert.Equal(t, 7, dailyCount, "should have exactly 7 daily backups after rotation")

	// Oldest (Apr 7) removed, newest (Apr 14) present.
	_, err := os.Stat(filepath.Join(backupDir, "kea_daily_2026-04-07.db"))
	assert.True(t, errors.Is(err, os.ErrNotExist))
	assertFileExists(t, backupDir, "kea_daily_2026-04-14.db")
}

func TestRun_CopyFailure_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")
	mkDB(t, dbPath)

	// Make the backups dir a file so MkdirAll fails, triggering an error path.
	backupsPath := filepath.Join(dir, "backups")
	require.NoError(t, os.WriteFile(backupsPath, []byte("not-a-dir"), 0644))

	clk := fakeClock{t: fixedTime("2026-04-14")}
	err := run(dbPath, clk, nil)
	assert.Error(t, err)
}

func TestRotate_PrunesOldestWeekly(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	// Create 6 weekly backup files (retention is 4).
	files := []string{
		"kea_weekly_2026-W10.db",
		"kea_weekly_2026-W11.db",
		"kea_weekly_2026-W12.db",
		"kea_weekly_2026-W13.db",
		"kea_weekly_2026-W14.db",
		"kea_weekly_2026-W15.db",
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(backupDir, f), []byte("x"), 0644))
	}

	require.NoError(t, rotate(backupDir, "kea", "weekly", ".db", 4))

	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Len(t, entries, 4)
	_, err = os.Stat(filepath.Join(backupDir, "kea_weekly_2026-W10.db"))
	assert.True(t, errors.Is(err, os.ErrNotExist))
	_, err = os.Stat(filepath.Join(backupDir, "kea_weekly_2026-W11.db"))
	assert.True(t, errors.Is(err, os.ErrNotExist))
	_, err = os.Stat(filepath.Join(backupDir, "kea_weekly_2026-W15.db"))
	assert.NoError(t, err)
}

func TestRotate_PrunesOldestMonthly(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	// Create 14 monthly backup files (retention is 12).
	files := []string{
		"kea_monthly_2025-01.db",
		"kea_monthly_2025-02.db",
		"kea_monthly_2025-03.db",
		"kea_monthly_2025-04.db",
		"kea_monthly_2025-05.db",
		"kea_monthly_2025-06.db",
		"kea_monthly_2025-07.db",
		"kea_monthly_2025-08.db",
		"kea_monthly_2025-09.db",
		"kea_monthly_2025-10.db",
		"kea_monthly_2025-11.db",
		"kea_monthly_2025-12.db",
		"kea_monthly_2026-01.db",
		"kea_monthly_2026-02.db",
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(backupDir, f), []byte("x"), 0644))
	}

	require.NoError(t, rotate(backupDir, "kea", "monthly", ".db", 12))

	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Len(t, entries, 12)
	_, err = os.Stat(filepath.Join(backupDir, "kea_monthly_2025-01.db"))
	assert.True(t, errors.Is(err, os.ErrNotExist))
	_, err = os.Stat(filepath.Join(backupDir, "kea_monthly_2025-02.db"))
	assert.True(t, errors.Is(err, os.ErrNotExist))
	_, err = os.Stat(filepath.Join(backupDir, "kea_monthly_2026-02.db"))
	assert.NoError(t, err)
}

func TestRun_WithDB_UsesOnlineBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, val TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test (val) VALUES ('hello')")
	require.NoError(t, err)

	clk := fakeClock{t: fixedTime("2026-04-14")}
	err = run(dbPath, clk, db)
	require.NoError(t, err)

	backupDir := filepath.Join(dir, "backups")
	backupPath := filepath.Join(backupDir, "kea_daily_2026-04-14.db")
	backupDB, err := sql.Open("sqlite3", backupPath)
	require.NoError(t, err)
	defer backupDB.Close()

	var val string
	err = backupDB.QueryRow("SELECT val FROM test WHERE id = 1").Scan(&val)
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
}

func TestDoBackup_NilDB_ProducesConsistentSnapshot(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")

	srcDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	require.NoError(t, err)

	_, err = srcDB.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	_, err = srcDB.Exec("INSERT INTO items (name) VALUES ('alpha'), ('beta')")
	require.NoError(t, err)

	// Leave data in WAL by NOT closing or checkpointing.
	// Write to WAL then keep the connection open to simulate a running web server.
	_, err = srcDB.Exec("INSERT INTO items (name) VALUES ('gamma')")
	require.NoError(t, err)
	defer srcDB.Close()

	dst := filepath.Join(dir, "backup.db")
	err = doBackup(dbPath, dst, nil)
	require.NoError(t, err)

	backupDB, err := sql.Open("sqlite3", dst)
	require.NoError(t, err)
	defer backupDB.Close()

	var result string
	err = backupDB.QueryRow("PRAGMA integrity_check").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result)

	var count int
	err = backupDB.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "backup should contain all committed rows")
}

// assertFileExists is a helper that checks a file exists in dir.
func assertFileExists(t *testing.T, dir, name string) {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, name))
	assert.NoError(t, err, "expected file %s to exist in %s", name, dir)
}
