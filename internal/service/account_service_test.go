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
