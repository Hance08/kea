package ledger

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noopDBInit is a test double that creates an empty file (simulating DB init).
func noopDBInit(path string, _ fs.FS) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(""), 0644)
}

func TestAddRunner_AutoPath(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)

	runner := &addRunner{
		registry:   r,
		migrations: nil,
		appDir:     dir,
		name:       "personal",
		customPath: "",
		dbInitFn:   noopDBInit,
	}
	err = runner.Run()

	require.NoError(t, err)
	expectedPath := filepath.Join(dir, "ledgers", "personal.db")
	entry, ok := r.Ledgers["personal"]
	require.True(t, ok, "ledger should be registered")
	assert.Equal(t, expectedPath, entry.Path)
	_, statErr := os.Stat(expectedPath)
	assert.NoError(t, statErr, "DB file should exist")
}

func TestAddRunner_CustomPath(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	customPath := filepath.Join(dir, "custom.db")

	runner := &addRunner{
		registry:   r,
		migrations: nil,
		appDir:     dir,
		name:       "custom",
		customPath: customPath,
		dbInitFn:   noopDBInit,
	}
	err = runner.Run()

	require.NoError(t, err)
	entry, ok := r.Ledgers["custom"]
	require.True(t, ok)
	assert.Equal(t, customPath, entry.Path)
}

func TestAddRunner_CustomPath_DirNotExist(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)

	runner := &addRunner{
		registry:   r,
		migrations: nil,
		appDir:     dir,
		name:       "bad",
		customPath: "/nonexistent/dir/bad.db",
		dbInitFn:   noopDBInit,
	}
	err = runner.Run()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory does not exist")
}

func TestAddRunner_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("personal", "/tmp/personal.db"))

	runner := &addRunner{
		registry: r,
		appDir:   dir,
		name:     "personal",
		dbInitFn: noopDBInit,
	}
	err = runner.Run()

	require.Error(t, err)
	assert.True(t, errors.Is(err, internalled.ErrLedgerExists))
}
