package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	err := run(dbPath, fakeClock{t: fixedTime("2026-04-14")})

	require.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(dir, "backups"))
	assert.True(t, os.IsNotExist(statErr), "backups/ dir should not be created when DB is absent")
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

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()

	src := filepath.Join(dir, "kea.db")
	require.NoError(t, os.WriteFile(src, []byte("db-contents"), 0644))

	dst := filepath.Join(dir, "backups", "kea_daily_2026-04-14.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0755))

	require.NoError(t, copyFile(src, dst))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, []byte("db-contents"), got)
}

func TestCopyFile_LeavesNoTmpOnFailure(t *testing.T) {
	dir := t.TempDir()
	// Source does not exist — copy must fail and leave no .tmp behind.
	src := filepath.Join(dir, "missing.db")
	dst := filepath.Join(dir, "backups", "out.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0755))

	err := copyFile(src, dst)
	assert.Error(t, err)

	// No .tmp file left behind.
	_, statErr := os.Stat(dst + ".tmp")
	assert.True(t, os.IsNotExist(statErr))
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
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(backupDir, "kea_daily_2026-04-07.db"))
	assert.True(t, os.IsNotExist(err))
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
