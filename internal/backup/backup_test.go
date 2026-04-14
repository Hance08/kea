package backup

import (
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
