// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestGetTransactionDetailsByIDs(t *testing.T) {
	t.Run("returns details for multiple transactions", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		txRepo.splitsWithAccts[1] = []model.SplitDetail{
			{ID: 10, AccountID: 1, AccountName: "Assets:Bank", AccountType: "A", Amount: 100, Currency: "USD"},
			{ID: 11, AccountID: 2, AccountName: "Expenses:Food", AccountType: "E", Amount: -100, Currency: "USD"},
		}
		txRepo.splitsWithAccts[2] = []model.SplitDetail{
			{ID: 20, AccountID: 1, AccountName: "Assets:Bank", AccountType: "A", Amount: -50, Currency: "USD"},
			{ID: 21, AccountID: 3, AccountName: "Revenue:Salary", AccountType: "R", Amount: 50, Currency: "USD"},
		}

		txs := []*model.Transaction{
			{ID: 1, Timestamp: 1000, Description: "Lunch", Status: model.StatusCleared, Type: model.TxTypeExpense},
			{ID: 2, Timestamp: 2000, Description: "Pay", Status: model.StatusPending, Type: model.TxTypeIncome},
		}

		result, err := svc.GetTransactionDetailsByIDs(context.Background(), txs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) != 2 {
			t.Fatalf("expected 2 details, got %d", len(result))
		}

		detail1 := result[1]
		if detail1.Description != "Lunch" {
			t.Errorf("expected description 'Lunch', got %q", detail1.Description)
		}
		if len(detail1.Splits) != 2 {
			t.Errorf("expected 2 splits for tx 1, got %d", len(detail1.Splits))
		}

		detail2 := result[2]
		if detail2.Type != model.TxTypeIncome {
			t.Errorf("expected type Income, got %v", detail2.Type)
		}
	})

	t.Run("empty input returns empty map", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		result, err := svc.GetTransactionDetailsByIDs(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("repo error propagates", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		svc := newTestTransactionService(accRepo, txRepo)

		txRepo.splitsByTxIDsErr = errors.New("db failure")

		txs := []*model.Transaction{
			{ID: 1, Timestamp: 1000, Description: "Test", Status: model.StatusPending, Type: model.TxTypeExpense},
		}

		_, err := svc.GetTransactionDetailsByIDs(context.Background(), txs)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
