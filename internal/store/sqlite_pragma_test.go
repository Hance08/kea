// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore_EnablesWALMode(t *testing.T) {
	s := setupTestDB(t)

	var journalMode string
	err := s.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journalMode)
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode)
}

func TestNewStore_SetsBusyTimeout(t *testing.T) {
	s := setupTestDB(t)

	var timeout int
	err := s.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&timeout)
	require.NoError(t, err)
	assert.Equal(t, 5000, timeout)
}
