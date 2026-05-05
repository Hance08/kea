// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// mockAccountRenamer
// ──────────────────────────────────────────────

type mockAccountRenamer struct {
	accounts     map[string]*model.Account
	renameCalls  []struct{ old, new string }
	renameErr    error
}

func newMockAccountRenamer() *mockAccountRenamer {
	return &mockAccountRenamer{accounts: make(map[string]*model.Account)}
}

func (m *mockAccountRenamer) add(name string) {
	m.accounts[name] = &model.Account{Name: name}
}

func (m *mockAccountRenamer) GetAccountByName(_ context.Context, name string) (*model.Account, error) {
	acc, ok := m.accounts[name]
	if !ok {
		return nil, fmt.Errorf("not found: %w", service.ErrNotFound)
	}
	return acc, nil
}

func (m *mockAccountRenamer) RenameAccount(_ context.Context, oldName, newSegment string) error {
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

// ──────────────────────────────────────────────
// migrateLegacySysAccWith tests
// ──────────────────────────────────────────────

func usdConfig() *config.Config {
	return &config.Config{Defaults: config.DefaultsConfig{Currency: "USD"}}
}

func TestMigrateLegacySysAccWith(t *testing.T) {
	t.Run("renames legacy account to currency-suffixed full path", func(t *testing.T) {
		mock := newMockAccountRenamer()
		mock.add(model.LegacyOpeningBalancesName)

		err := migrateLegacySysAccWith(mock, usdConfig())
		require.NoError(t, err)

		require.Len(t, mock.renameCalls, 1)
		assert.Equal(t, model.LegacyOpeningBalancesName, mock.renameCalls[0].old)
		assert.Equal(t, "OpeningBalances_USD", mock.renameCalls[0].new)
	})

	t.Run("no-op when legacy account does not exist", func(t *testing.T) {
		mock := newMockAccountRenamer()

		err := migrateLegacySysAccWith(mock, usdConfig())
		require.NoError(t, err)
		assert.Empty(t, mock.renameCalls)
	})

	t.Run("no-op when target already exists (idempotent)", func(t *testing.T) {
		mock := newMockAccountRenamer()
		mock.add(model.LegacyOpeningBalancesName)
		mock.add(model.OpeningBalancesAccountName("USD"))

		err := migrateLegacySysAccWith(mock, usdConfig())
		require.NoError(t, err)
		assert.Empty(t, mock.renameCalls)
	})

	t.Run("propagates rename error", func(t *testing.T) {
		mock := newMockAccountRenamer()
		mock.add(model.LegacyOpeningBalancesName)
		mock.renameErr = errors.New("db error")

		err := migrateLegacySysAccWith(mock, usdConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to migrate legacy system account")
	})
}
