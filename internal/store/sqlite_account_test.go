// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.NewStore(dbPath, migrations.FS)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func accountNames(accounts []*model.Account) []string {
	names := make([]string, len(accounts))
	for i, a := range accounts {
		names[i] = a.Name
	}
	return names
}

// TestRenameAccount_LIKEWildcardsInName verifies that accounts whose names share
// a prefix but are not true descendants are not incorrectly renamed.
// Assets:AXB and Assets:AXB:Sub must be unaffected when Assets:A_B is renamed.
func TestRenameAccount_LIKEWildcardsInName(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	for _, name := range []string{"Assets:A_B", "Assets:A_B:Sub", "Assets:AXB", "Assets:AXB:Sub"} {
		_, err := s.CreateAccount(ctx, name, model.AccountTypeAsset, "USD", "", nil)
		require.NoError(t, err, "creating account %q", name)
	}

	err := s.RenameAccount(ctx, "Assets:A_B", "Assets:C")
	require.NoError(t, err)

	accounts, err := s.GetAllAccounts(ctx)
	require.NoError(t, err)
	names := accountNames(accounts)

	assert.Contains(t, names, "Assets:C", "renamed account should exist")
	assert.Contains(t, names, "Assets:C:Sub", "child of renamed account should be renamed")
	assert.Contains(t, names, "Assets:AXB", "sibling with similar prefix should be unchanged")
	assert.Contains(t, names, "Assets:AXB:Sub", "child of sibling with similar prefix should be unchanged")
	assert.NotContains(t, names, "Assets:A_B", "old account name should not exist")
	assert.NotContains(t, names, "Assets:A_B:Sub", "old child name should not exist")
}

// TestRenameAccount_DeepNesting verifies that a rename cascades correctly through
// multiple levels of nesting.
func TestRenameAccount_DeepNesting(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	for _, name := range []string{"Assets:Bank", "Assets:Bank:Checking", "Assets:Bank:Checking:Joint"} {
		_, err := s.CreateAccount(ctx, name, model.AccountTypeAsset, "USD", "", nil)
		require.NoError(t, err, "creating account %q", name)
	}

	err := s.RenameAccount(ctx, "Assets:Bank", "Assets:CU")
	require.NoError(t, err)

	accounts, err := s.GetAllAccounts(ctx)
	require.NoError(t, err)
	names := accountNames(accounts)

	assert.Contains(t, names, "Assets:CU", "renamed root account should exist")
	assert.Contains(t, names, "Assets:CU:Checking", "first-level descendant should be renamed")
	assert.Contains(t, names, "Assets:CU:Checking:Joint", "second-level descendant should be renamed")
	assert.NotContains(t, names, "Assets:Bank", "old root account name should not exist")
	assert.NotContains(t, names, "Assets:Bank:Checking", "old first-level descendant should not exist")
	assert.NotContains(t, names, "Assets:Bank:Checking:Joint", "old second-level descendant should not exist")
}

// TestRenameAccount_SiblingUnaffected verifies that accounts whose name starts
// with the old name but lack the ":" separator are not touched by the cascade.
func TestRenameAccount_SiblingUnaffected(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	for _, name := range []string{"Assets:Bank", "Assets:Bank:Checking", "Assets:Bankrupt"} {
		_, err := s.CreateAccount(ctx, name, model.AccountTypeAsset, "USD", "", nil)
		require.NoError(t, err, "creating account %q", name)
	}

	err := s.RenameAccount(ctx, "Assets:Bank", "Assets:CreditUnion")
	require.NoError(t, err)

	accounts, err := s.GetAllAccounts(ctx)
	require.NoError(t, err)
	names := accountNames(accounts)

	assert.Contains(t, names, "Assets:CreditUnion", "renamed account should exist")
	assert.Contains(t, names, "Assets:CreditUnion:Checking", "child of renamed account should be renamed")
	assert.Contains(t, names, "Assets:Bankrupt", "account with name starting with old prefix should be unchanged")
	assert.NotContains(t, names, "Assets:Bank", "old account name should not exist")
	assert.NotContains(t, names, "Assets:Bank:Checking", "old child name should not exist")
}
