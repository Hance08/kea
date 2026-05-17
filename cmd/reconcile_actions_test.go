// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveMode(t *testing.T) {
	t.Run("no flags means interactive", func(t *testing.T) {
		mode, err := resolveMode(false, false, false)
		require.NoError(t, err)
		assert.False(t, mode)
	})

	t.Run("balance and ids means non-interactive", func(t *testing.T) {
		mode, err := resolveMode(true, true, false)
		require.NoError(t, err)
		assert.True(t, mode)
	})

	t.Run("balance and ids and force means non-interactive", func(t *testing.T) {
		mode, err := resolveMode(true, true, true)
		require.NoError(t, err)
		assert.True(t, mode)
	})

	t.Run("balance only is an error", func(t *testing.T) {
		_, err := resolveMode(true, false, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--ids")
	})

	t.Run("ids only is an error", func(t *testing.T) {
		_, err := resolveMode(false, true, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--balance")
	})

	t.Run("force only is an error", func(t *testing.T) {
		_, err := resolveMode(false, false, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--balance")
		assert.Contains(t, err.Error(), "--ids")
	})

	t.Run("force with balance only is an error", func(t *testing.T) {
		_, err := resolveMode(true, false, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--ids")
	})

	t.Run("force with ids only is an error", func(t *testing.T) {
		_, err := resolveMode(false, true, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--balance")
	})
}
