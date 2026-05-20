// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestFilterTransactions_Service(t *testing.T) {
	t.Run("delegates to repo with filter", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: 1000, Description: "Groceries", Status: model.StatusCleared, Type: model.TxTypeExpense}, nil)
		txRepo.addTransaction(&model.Transaction{ID: 2, Timestamp: 2000, Description: "Salary", Status: model.StatusPending, Type: model.TxTypeIncome}, nil)

		txType := model.TxTypeExpense
		result, err := svc.FilterTransactions(context.Background(), model.TransactionFilter{Type: &txType}, model.ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Items) != 1 {
			t.Errorf("expected 1 item, got %d", len(result.Items))
		}
		if result.Items[0].Description != "Groceries" {
			t.Errorf("expected Groceries, got %s", result.Items[0].Description)
		}
	})

	t.Run("repo error propagates", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		txRepo.filterErr = errors.New("db failure")

		_, err := svc.FilterTransactions(context.Background(), model.TransactionFilter{}, model.ListOptions{Limit: 10})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestFilterTransactionsByAccountName(t *testing.T) {
	t.Run("resolves account name and filters", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		accRepo.addAccount(&model.Account{ID: 5, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"})

		txRepo.addTransaction(&model.Transaction{ID: 1, Timestamp: 1000, Description: "ATM", Status: model.StatusCleared, Type: model.TxTypeExpense},
			[]*model.Split{{ID: 1, TransactionID: 1, AccountID: 5, Amount: -1000, Currency: "USD"}})
		txRepo.addTransaction(&model.Transaction{ID: 2, Timestamp: 2000, Description: "Salary", Status: model.StatusPending, Type: model.TxTypeIncome},
			[]*model.Split{{ID: 2, TransactionID: 2, AccountID: 99, Amount: 5000, Currency: "USD"}})

		result, err := svc.FilterTransactionsByAccountName(context.Background(), "Assets:Bank", model.TransactionFilter{}, model.ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Items) != 1 {
			t.Errorf("expected 1 item, got %d", len(result.Items))
		}
	})

	t.Run("unknown account returns error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		_, err := svc.FilterTransactionsByAccountName(context.Background(), "Nonexistent", model.TransactionFilter{}, model.ListOptions{Limit: 10})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
