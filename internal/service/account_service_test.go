// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAccountByID(t *testing.T) {
	t.Run("returns account when found", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.GetAccountByID(context.Background(), 1)

		require.NoError(t, err)
		assert.Equal(t, int64(1), acc.ID)
		assert.Equal(t, "Assets:Cash", acc.Name)
	})

	t.Run("wraps ErrNotFound from repo", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.GetAccountByID(context.Background(), 999)

		assert.Nil(t, acc)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.Contains(t, err.Error(), "999")
	})

	t.Run("passes through other errors", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		dbErr := fmt.Errorf("connection refused")
		accRepo.getByIDErr[42] = dbErr
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.GetAccountByID(context.Background(), 42)

		assert.Nil(t, acc)
		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrNotFound))
		assert.Equal(t, dbErr, err)
	})
}

func TestListAccounts(t *testing.T) {
	mkAccRepo := func() *mockAccountRepo {
		r := newMockAccountRepo()
		r.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		r.addAccount(&model.Account{ID: 2, Name: "Assets:Hidden", Type: model.AccountTypeAsset, Currency: "USD", IsHidden: true})
		r.addAccount(&model.Account{ID: 3, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"})
		return r
	}

	t.Run("hides hidden accounts by default", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{})

		require.NoError(t, err)
		assert.Len(t, accounts, 2)
		for _, acc := range accounts {
			assert.False(t, acc.IsHidden)
		}
	})

	t.Run("shows hidden accounts when ShowHidden is true", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{ShowHidden: true})

		require.NoError(t, err)
		assert.Len(t, accounts, 3)
	})

	t.Run("filters by type", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())
		at := model.AccountTypeAsset

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{Type: &at})

		require.NoError(t, err)
		assert.Len(t, accounts, 1)
		assert.Equal(t, "Assets:Cash", accounts[0].Name)
	})

	t.Run("filters by type with ShowHidden", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())
		at := model.AccountTypeAsset

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{
			Type:       &at,
			ShowHidden: true,
		})

		require.NoError(t, err)
		assert.Len(t, accounts, 2)
	})
}
