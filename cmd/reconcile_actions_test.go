// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIDs_ValidInput(t *testing.T) {
	ids, err := parseIDs("1, 2, 3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	if ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("expected [1 2 3], got %v", ids)
	}
}

func TestParseIDs_DuplicateIDs_ReturnsError(t *testing.T) {
	_, err := parseIDs("10,10")
	if err == nil {
		t.Fatal("expected error for duplicate IDs, got nil")
	}
}

func TestParseIDs_DuplicateIDs_NonAdjacent_ReturnsError(t *testing.T) {
	_, err := parseIDs("10,20,10")
	if err == nil {
		t.Fatal("expected error for duplicate IDs, got nil")
	}
}

func TestParseIDs_EmptyInput_ReturnsError(t *testing.T) {
	_, err := parseIDs("")
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestParseIDs_InvalidNumber_ReturnsError(t *testing.T) {
	_, err := parseIDs("10,abc,20")
	if err == nil {
		t.Fatal("expected error for invalid number, got nil")
	}
}

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
