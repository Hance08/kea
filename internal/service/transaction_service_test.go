// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

func TestGetTransactionByID_RepoError_WrapsWithContext(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	repoErr := fmt.Errorf("db connection lost")
	txRepo.getByIDErr[99] = repoErr

	_, err := svc.GetTransactionByID(context.Background(), 99)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("expected error to wrap %v, got %v", repoErr, err)
	}
	if !strings.Contains(err.Error(), "failed to retrieve transaction") {
		t.Errorf("expected context message in error, got: %s", err.Error())
	}
}

func TestGetTransactionByID_NotFound_WrapsServiceErrNotFound(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	// Simulate repository.ErrNotFound
	txRepo.getByIDErr[999] = repository.ErrNotFound

	_, err := svc.GetTransactionByID(context.Background(), 999)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected error to wrap service.ErrNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "transaction") {
		t.Errorf("expected 'transaction' in error message, got: %s", err.Error())
	}
}

func TestGetMonthlyBalanceHistory_AssetRunningBalance(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	// Asset account id=10
	accRepo.accountsByID[10] = &model.Account{
		ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
	}
	txRepo.monthlySplitTotals = []repository.MonthlySplitTotal{
		{AccountID: 10, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-01", Amount: 100000},
		{AccountID: 10, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-02", Amount: 50000},
		{AccountID: 10, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-03", Amount: -20000},
	}

	got, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err != nil {
		t.Fatalf("GetMonthlyBalanceHistory: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("series count: got %d, want 1", len(got))
	}
	if got[0].AccountID != 10 {
		t.Errorf("AccountID: got %d, want 10", got[0].AccountID)
	}
	if got[0].Currency != "USD" {
		t.Errorf("Currency: got %q, want USD", got[0].Currency)
	}
	want := []model.MonthlyBalancePoint{
		{Month: "2024-01", Balance: 100000},
		{Month: "2024-02", Balance: 150000},
		{Month: "2024-03", Balance: 130000},
	}
	if !reflect.DeepEqual(got[0].Points, want) {
		t.Errorf("Points: got %+v, want %+v", got[0].Points, want)
	}
}

func TestGetMonthlyBalanceHistory_LiabilityNaturalSign(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)
	accRepo.accountsByID[20] = &model.Account{
		ID: 20, Name: "Liabilities:Card", Type: model.AccountTypeLiability, Currency: "USD",
	}
	txRepo.monthlySplitTotals = []repository.MonthlySplitTotal{
		// Liability balances are stored negative when the debt grows.
		{AccountID: 20, AccountType: model.AccountTypeLiability, Currency: "USD", Month: "2024-01", Amount: -50000},
		{AccountID: 20, AccountType: model.AccountTypeLiability, Currency: "USD", Month: "2024-02", Amount: -30000},
	}

	got, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []model.MonthlyBalancePoint{
		{Month: "2024-01", Balance: 50000}, // -(-50000)
		{Month: "2024-02", Balance: 80000}, // -(-50000 + -30000)
	}
	if !reflect.DeepEqual(got[0].Points, want) {
		t.Errorf("Points: got %+v, want %+v", got[0].Points, want)
	}
}

func TestGetMonthlyBalanceHistory_SingleMonth(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)
	accRepo.accountsByID[30] = &model.Account{
		ID: 30, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD",
	}
	txRepo.monthlySplitTotals = []repository.MonthlySplitTotal{
		{AccountID: 30, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-05", Amount: 1234},
	}
	got, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || len(got[0].Points) != 1 {
		t.Fatalf("expected 1 series with 1 point, got %+v", got)
	}
	if got[0].Points[0].Balance != 1234 {
		t.Errorf("Balance: got %d, want 1234", got[0].Points[0].Balance)
	}
}

func TestGetMonthlyBalanceHistory_EmptyLedger(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)
	txRepo.monthlySplitTotals = nil

	got, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty series list, got %+v", got)
	}
}

func TestGetMonthlyBalanceHistory_RepoError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)
	txRepo.monthlySplitTotalsErr = errors.New("boom")

	_, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetMonthlyBalanceHistory_SkipsAllZeroSeries(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)
	accRepo.accountsByID[40] = &model.Account{
		ID: 40, Name: "Assets:Wash", Type: model.AccountTypeAsset, Currency: "USD",
	}
	accRepo.accountsByID[41] = &model.Account{
		ID: 41, Name: "Assets:Real", Type: model.AccountTypeAsset, Currency: "USD",
	}
	txRepo.monthlySplitTotals = []repository.MonthlySplitTotal{
		// Account 40: every cumulative balance is zero — series should be skipped.
		{AccountID: 40, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-01", Amount: 0},
		{AccountID: 40, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-02", Amount: 0},
		// Account 41: real activity — should appear.
		{AccountID: 41, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-01", Amount: 5000},
	}

	got, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("series count: got %d, want 1 (account 40 should be skipped): %+v", len(got), got)
	}
	if got[0].AccountID != 41 {
		t.Errorf("AccountID: got %d, want 41", got[0].AccountID)
	}
}
