package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveRunner_UnregisterOnly(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))
	require.NoError(t, r.Add("personal", filepath.Join(dir, "personal.db")))
	require.NoError(t, r.Switch("work"))

	runner := &removeRunner{registry: r, name: "personal", deleteFile: false, yes: true}
	err = runner.Run()

	require.NoError(t, err)
	assert.NotContains(t, r.Ledgers, "personal")
}

func TestRemoveRunner_DeleteFile(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	dbFile := filepath.Join(dir, "personal.db")
	require.NoError(t, os.WriteFile(dbFile, []byte(""), 0644))
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))
	require.NoError(t, r.Add("personal", dbFile))
	require.NoError(t, r.Switch("work"))

	runner := &removeRunner{registry: r, name: "personal", deleteFile: true, yes: true}
	err = runner.Run()

	require.NoError(t, err)
	_, statErr := os.Stat(dbFile)
	assert.True(t, errors.Is(statErr, os.ErrNotExist))
}

func TestRemoveRunner_RefusesActiveLedger(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))
	require.NoError(t, r.Switch("work"))

	runner := &removeRunner{registry: r, name: "work", deleteFile: false, yes: true}
	err = runner.Run()

	require.Error(t, err)
	assert.True(t, errors.Is(err, internalled.ErrRemoveActive))
}
