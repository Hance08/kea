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
