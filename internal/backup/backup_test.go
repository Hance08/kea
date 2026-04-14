package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// mkDB creates a fake DB file at dbPath with contents "fake-db".
func mkDB(t *testing.T, dbPath string) {
	t.Helper()
	require.NoError(t, os.WriteFile(dbPath, []byte("fake-db"), 0644))
}

func TestRun_FirstRun_AllThreeTiersCreated(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kea.db")
	mkDB(t, dbPath)
	clk := fakeClock{t: fixedTime("2026-04-14")}

	require.NoError(t, run(dbPath, clk))

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

	require.NoError(t, run(dbPath, clk))

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

	require.NoError(t, run(dbPath, clk))

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

	require.NoError(t, run(dbPath, clk))

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

	require.NoError(t, run(dbPath, clk))

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
	require.NoError(t, run(dbPath, clk))

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
	assert.True(t, os.IsNotExist(err))
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
	err := run(dbPath, clk)
	assert.Error(t, err)
}

// assertFileExists is a helper that checks a file exists in dir.
func assertFileExists(t *testing.T, dir, name string) {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, name))
	assert.NoError(t, err, "expected file %s to exist in %s", name, dir)
}
