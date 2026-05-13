// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"path/filepath"
	"testing"

	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStore_DB verifies that the DB() accessor returns a non-nil *sql.DB
// that can be used to interact with the underlying database.
func TestStore_DB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.NewStore(dbPath, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	db := s.DB()
	assert.NotNil(t, db, "DB() should return a non-nil *sql.DB")

	// Verify the returned *sql.DB responds to Ping
	err = db.Ping()
	assert.NoError(t, err, "should be able to Ping the returned *sql.DB")
}
