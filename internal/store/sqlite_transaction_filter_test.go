// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestFilterTransactions(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Seed accounts
	accBank, _ := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	accFood, _ := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	accSalary, _ := s.CreateAccount(ctx, "Revenue:Salary", model.AccountTypeRevenue, "USD", "", nil)

	// Seed transactions
	// tx1: Expense, Cleared, timestamp 1000
	s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "Groceries", Status: model.StatusCleared, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: accBank, Amount: -5000, Currency: "USD"},
		{AccountID: accFood, Amount: 5000, Currency: "USD"},
	})
	// tx2: Income, Pending, timestamp 2000
	s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "Paycheck Jan", Status: model.StatusPending, Type: model.TxTypeIncome,
	}, []model.Split{
		{AccountID: accBank, Amount: 100000, Currency: "USD"},
		{AccountID: accSalary, Amount: -100000, Currency: "USD"},
	})
	// tx3: Expense, Cleared, timestamp 3000
	s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 3000, Description: "Restaurant dinner", Status: model.StatusCleared, Type: model.TxTypeExpense,
	}, []model.Split{
		{AccountID: accBank, Amount: -3000, Currency: "USD"},
		{AccountID: accFood, Amount: 3000, Currency: "USD"},
	})

	t.Run("no filters returns all", func(t *testing.T) {
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 3 {
			t.Errorf("expected 3 total, got %d", result.TotalCount)
		}
		if len(result.Items) != 3 {
			t.Errorf("expected 3 items, got %d", len(result.Items))
		}
	})

	t.Run("filter by account", func(t *testing.T) {
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{AccountID: &accFood}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 2 {
			t.Errorf("expected 2, got %d", result.TotalCount)
		}
	})

	t.Run("filter by type", func(t *testing.T) {
		txType := model.TxTypeIncome
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{Type: &txType}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 1 {
			t.Errorf("expected 1, got %d", result.TotalCount)
		}
		if result.Items[0].Description != "Paycheck Jan" {
			t.Errorf("expected Paycheck Jan, got %s", result.Items[0].Description)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		status := model.StatusCleared
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{Status: &status}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 2 {
			t.Errorf("expected 2, got %d", result.TotalCount)
		}
	})

	t.Run("filter by date range", func(t *testing.T) {
		start := int64(1500)
		end := int64(2500)
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{StartTime: &start, EndTime: &end}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 1 {
			t.Errorf("expected 1, got %d", result.TotalCount)
		}
	})

	t.Run("filter by description", func(t *testing.T) {
		desc := "dinner"
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{Description: &desc}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 1 {
			t.Errorf("expected 1, got %d", result.TotalCount)
		}
		if result.Items[0].Description != "Restaurant dinner" {
			t.Errorf("expected Restaurant dinner, got %s", result.Items[0].Description)
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		status := model.StatusCleared
		txType := model.TxTypeExpense
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{
			AccountID: &accFood,
			Status:    &status,
			Type:      &txType,
		}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 2 {
			t.Errorf("expected 2, got %d", result.TotalCount)
		}
	})

	t.Run("pagination with filter", func(t *testing.T) {
		status := model.StatusCleared
		page1, err := s.FilterTransactions(ctx, model.TransactionFilter{Status: &status}, model.ListOptions{Limit: 1, Offset: 0, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(page1.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(page1.Items))
		}
		if page1.TotalCount != 2 {
			t.Errorf("expected total 2, got %d", page1.TotalCount)
		}
		if page1.Items[0].Timestamp != 3000 {
			t.Errorf("expected timestamp 3000, got %d", page1.Items[0].Timestamp)
		}

		page2, err := s.FilterTransactions(ctx, model.TransactionFilter{Status: &status}, model.ListOptions{Limit: 1, Offset: 1, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(page2.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(page2.Items))
		}
		if page2.Items[0].Timestamp != 1000 {
			t.Errorf("expected timestamp 1000, got %d", page2.Items[0].Timestamp)
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		txType := model.TxTypeTransfer
		result, err := s.FilterTransactions(ctx, model.TransactionFilter{Type: &txType}, model.ListOptions{Limit: 10, IncludeCount: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalCount != 0 {
			t.Errorf("expected 0, got %d", result.TotalCount)
		}
		if len(result.Items) != 0 {
			t.Errorf("expected empty items, got %d", len(result.Items))
		}
	})
}
