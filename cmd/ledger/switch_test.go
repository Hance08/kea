package ledger

import (
	"errors"
	"path/filepath"
	"testing"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwitchRunner_HappyPath(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))

	runner := &switchRunner{registry: r, name: "work"}
	err = runner.Run()

	require.NoError(t, err)
	assert.Equal(t, "work", r.ActiveLedger)
}

func TestSwitchRunner_UnknownName(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)

	runner := &switchRunner{registry: r, name: "ghost"}
	err = runner.Run()

	require.Error(t, err)
	assert.True(t, errors.Is(err, internalled.ErrLedgerNotFound))
}
