package ledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FreshInstall(t *testing.T) {
	dir := t.TempDir()

	r, err := Load(dir)

	require.NoError(t, err)
	assert.Empty(t, r.Ledgers)
	assert.Empty(t, r.ActiveLedger)
	_, statErr := os.Stat(filepath.Join(dir, "ledgers.yaml"))
	assert.NoError(t, statErr, "ledgers.yaml should be created on fresh install")
}

func TestLoad_AutoMigratesLegacyDB(t *testing.T) {
	dir := t.TempDir()
	legacyDB := filepath.Join(dir, "kea.db")
	require.NoError(t, os.WriteFile(legacyDB, []byte(""), 0644))

	r, err := Load(dir)

	require.NoError(t, err)
	require.Contains(t, r.Ledgers, "default")
	assert.Equal(t, legacyDB, r.Ledgers["default"].Path)
	assert.Equal(t, "default", r.ActiveLedger)
	assert.True(t, r.MigratedLegacy, "should signal migration to caller")
}

func TestLoad_NormalLoad(t *testing.T) {
	dir := t.TempDir()
	r1, err := Load(dir)
	require.NoError(t, err)
	r1.Ledgers["work"] = Entry{Path: "/tmp/work.db"}
	r1.ActiveLedger = "work"
	require.NoError(t, r1.Save())

	r2, err := Load(dir)

	require.NoError(t, err)
	require.Contains(t, r2.Ledgers, "work")
	assert.Equal(t, "/tmp/work.db", r2.Ledgers["work"].Path)
	assert.Equal(t, "work", r2.ActiveLedger)
}

func TestAdd_HappyPath(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)

	err = r.Add("personal", "/tmp/personal.db")

	require.NoError(t, err)
	assert.Equal(t, "/tmp/personal.db", r.Ledgers["personal"].Path)
}

func TestAdd_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))

	err = r.Add("personal", "/tmp/other.db")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLedgerExists)
	assert.Equal(t, "/tmp/personal.db", r.Ledgers["personal"].Path, "existing entry must not be overwritten")
}

func TestAdd_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	r2, err := Load(dir)
	require.NoError(t, err)
	assert.Contains(t, r2.Ledgers, "work")
}

func TestSwitch_HappyPath(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))

	err = r.Switch("work")

	require.NoError(t, err)
	assert.Equal(t, "work", r.ActiveLedger)
}

func TestSwitch_UnknownName(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)

	err = r.Switch("nonexistent")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLedgerNotFound)
}

func TestSwitch_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", "/tmp/work.db"))
	require.NoError(t, r.Switch("work"))

	r2, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "work", r2.ActiveLedger)
}
