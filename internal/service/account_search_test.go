// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestSearchAccounts(t *testing.T) {
	t.Run("filters by query substring", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestAccountService(accRepo, txRepo)

		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank:Savings", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"})

		query := "Bank"
		result, err := svc.SearchAccounts(context.Background(), model.AccountFilter{Query: &query}, model.ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Items) != 2 {
			t.Fatalf("expected 2 results, got %d", len(result.Items))
		}
	})

	t.Run("filters hidden and system accounts by default", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestAccountService(accRepo, txRepo)

		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Hidden", Type: model.AccountTypeAsset, Currency: "USD", IsHidden: true})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Equity:OpeningBalances_USD", Type: model.AccountTypeEquity, Currency: "USD"})

		query := ""
		result, err := svc.SearchAccounts(context.Background(), model.AccountFilter{Query: &query}, model.ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Items) != 1 {
			t.Fatalf("expected 1 result (hidden and system excluded), got %d", len(result.Items))
		}
		if result.Items[0].Name != "Assets:Bank:Checking" {
			t.Fatalf("expected Assets:Bank:Checking, got %s", result.Items[0].Name)
		}
	})

	t.Run("returns repo error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestAccountService(accRepo, txRepo)
		accRepo.searchErr = errors.New("db error")

		result, err := svc.SearchAccounts(context.Background(), model.AccountFilter{}, model.ListOptions{Limit: 10})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if result != nil {
			t.Fatalf("expected nil result on error")
		}
	})
}
