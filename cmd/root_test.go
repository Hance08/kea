// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	ledgercmd "github.com/hance08/kea/cmd/ledger"
	"github.com/hance08/kea/internal/config"
	internalled "github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// mockAccountMigrator
// ──────────────────────────────────────────────

type mockAccountMigrator struct {
	accounts     map[string]*model.Account
	hasTx        map[string]bool // accounts that have transactions
	renameCalls  []struct{ old, new string }
	deleteCalls  []string
	renameErr    error
	deleteErr    error
}

func newMockAccountMigrator() *mockAccountMigrator {
	return &mockAccountMigrator{
		accounts: make(map[string]*model.Account),
		hasTx:    make(map[string]bool),
	}
}

func (m *mockAccountMigrator) add(name string) {
	m.accounts[name] = &model.Account{Name: name}
}

func (m *mockAccountMigrator) GetAccountByName(_ context.Context, name string) (*model.Account, error) {
	acc, ok := m.accounts[name]
	if !ok {
		return nil, fmt.Errorf("not found: %w", service.ErrNotFound)
	}
	return acc, nil
}

func (m *mockAccountMigrator) RenameAccount(_ context.Context, oldName, newSegment string) error {
	if m.renameErr != nil {
		return m.renameErr
	}
	m.renameCalls = append(m.renameCalls, struct{ old, new string }{oldName, newSegment})
	acc := m.accounts[oldName]
	delete(m.accounts, oldName)
	acc.Name = newSegment
	m.accounts[newSegment] = acc
	return nil
}

func (m *mockAccountMigrator) DeleteAccountByName(_ context.Context, name string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if m.hasTx[name] {
		return fmt.Errorf("account %q has transactions and cannot be deleted", name)
	}
	m.deleteCalls = append(m.deleteCalls, name)
	delete(m.accounts, name)
	return nil
}

// ──────────────────────────────────────────────
// migrateLegacySysAccWith tests
// ──────────────────────────────────────────────

func usdConfig() *config.Config {
	return &config.Config{Defaults: config.DefaultsConfig{Currency: "USD"}}
}

func TestMigrateLegacySysAccWith(t *testing.T) {
	t.Run("renames legacy account to currency-suffixed leaf segment", func(t *testing.T) {
		mock := newMockAccountMigrator()
		mock.add(model.LegacyOpeningBalancesName)

		err := migrateLegacySysAccWith(mock, usdConfig())
		require.NoError(t, err)

		require.Len(t, mock.renameCalls, 1)
		assert.Equal(t, model.LegacyOpeningBalancesName, mock.renameCalls[0].old)
		assert.Equal(t, "OpeningBalances_USD", mock.renameCalls[0].new)
		assert.Empty(t, mock.deleteCalls)
	})

	t.Run("no-op when legacy account does not exist", func(t *testing.T) {
		mock := newMockAccountMigrator()

		err := migrateLegacySysAccWith(mock, usdConfig())
		require.NoError(t, err)
		assert.Empty(t, mock.renameCalls)
		assert.Empty(t, mock.deleteCalls)
	})

	t.Run("both accounts exist and legacy is empty: deletes legacy", func(t *testing.T) {
		mock := newMockAccountMigrator()
		mock.add(model.LegacyOpeningBalancesName)
		mock.add(model.OpeningBalancesAccountName("USD"))

		err := migrateLegacySysAccWith(mock, usdConfig())
		require.NoError(t, err)
		assert.Empty(t, mock.renameCalls)
		require.Len(t, mock.deleteCalls, 1)
		assert.Equal(t, model.LegacyOpeningBalancesName, mock.deleteCalls[0])
	})

	t.Run("both accounts exist and legacy has transactions: returns error", func(t *testing.T) {
		mock := newMockAccountMigrator()
		mock.add(model.LegacyOpeningBalancesName)
		mock.add(model.OpeningBalancesAccountName("USD"))
		mock.hasTx[model.LegacyOpeningBalancesName] = true

		err := migrateLegacySysAccWith(mock, usdConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), model.LegacyOpeningBalancesName)
		assert.Contains(t, err.Error(), model.OpeningBalancesAccountName("USD"))
		assert.Empty(t, mock.renameCalls)
		assert.Empty(t, mock.deleteCalls)
	})

	t.Run("propagates rename error", func(t *testing.T) {
		mock := newMockAccountMigrator()
		mock.add(model.LegacyOpeningBalancesName)
		mock.renameErr = errors.New("db error")

		err := migrateLegacySysAccWith(mock, usdConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to migrate legacy system account")
	})
}

func TestLedgerCommand_SkipsDBInit(t *testing.T) {
	appDir := t.TempDir()
	registry, err := internalled.Load(appDir)
	require.NoError(t, err)

	// Register a ledger pointing to a nonexistent DB file.
	// If DB init runs, it would fail or create the file.
	bogusPath := filepath.Join(appDir, "does-not-exist.db")
	require.NoError(t, registry.Add("phantom", bogusPath))
	require.NoError(t, registry.Switch("phantom"))

	// Build the same command tree that Execute builds.
	rootCmd := &cobra.Command{
		Use:           "kea",
		SilenceErrors: true,
	}
	rootCmd.AddCommand(ledgercmd.NewLedgerCmd(registry, nil, appDir))

	// Simulate: the guard detects "ledger list" and short-circuits.
	args := []string{"ledger", "list"}
	require.True(t, isLedgerCommand(args), "should detect ledger command")

	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	require.NoError(t, err)

	// The bogus DB file must not have been created or opened.
	_, statErr := os.Stat(bogusPath)
	assert.True(t, os.IsNotExist(statErr), "DB file should not exist — DB init must not have run")
}

func TestIsLedgerCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"ledger list", []string{"ledger", "list"}, true},
		{"ledger ls alias", []string{"ledger", "ls"}, true},
		{"ledger add", []string{"ledger", "add", "work"}, true},
		{"ledger switch", []string{"ledger", "switch", "work"}, true},
		{"ledger remove", []string{"ledger", "remove", "old"}, true},
		{"bare ledger", []string{"ledger"}, true},
		{"account list", []string{"account", "list"}, false},
		{"add transaction", []string{"add"}, false},
		{"no args", []string{}, false},
		{"ledger with global flags", []string{"--no-color", "ledger", "list"}, true},
		{"ledger with config flag", []string{"-c", "/tmp/cfg.yaml", "ledger", "add", "x"}, true},
		{"config with equals", []string{"--config=/tmp/cfg.yaml", "ledger", "list"}, true},
		{"double-dash before ledger", []string{"--", "ledger", "list"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isLedgerCommand(tt.args))
		})
	}
}
