// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package ledger

import (
	"bytes"
	"path/filepath"
	"testing"

	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRunner_NoLedgers(t *testing.T) {
	// Construct an empty registry directly (e.g. corrupted/manually edited ledgers.yaml).
	r := internalled.EmptyRegistry()

	var buf bytes.Buffer
	runner := &listRunner{registry: r, out: &buf}
	err := runner.Run()

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No ledgers registered")
}

func TestListRunner_ShowsLedgersWithActiveMarker(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("personal", filepath.Join(dir, "personal.db")))
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))
	require.NoError(t, r.Switch("work"))

	var buf bytes.Buffer
	runner := &listRunner{registry: r, out: &buf}
	err = runner.Run()

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "* work")
	assert.Contains(t, output, "  personal")
}

func TestListRunner_EnvVarActiveMarker(t *testing.T) {
	dir := t.TempDir()
	r, err := internalled.Load(dir)
	require.NoError(t, err)
	require.NoError(t, r.Add("personal", filepath.Join(dir, "personal.db")))
	require.NoError(t, r.Add("work", filepath.Join(dir, "work.db")))
	require.NoError(t, r.Switch("personal"))
	t.Setenv("KEA_LEDGER", "work")

	var buf bytes.Buffer
	runner := &listRunner{registry: r, out: &buf}
	err = runner.Run()

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "* work")
	assert.Contains(t, output, "  personal")
}
