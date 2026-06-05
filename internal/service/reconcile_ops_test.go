// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hance08/kea/internal/model"
)

// seedUnreconciled injects unreconciled entries into the mock for accountID.
func seedUnreconciled(txRepo *mockTransactionRepo, accountID int64, entries []*model.ReconcileEntry) {
	txRepo.unreconciledByAccount[accountID] = entries
}

// ── GetUnreconciledByAccount ──────────────────────────────────────────────────

func TestGetUnreconciledByAccount_ReturnsEntriesAndBalance(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Timestamp: 1000, Description: "Salary", Status: model.StatusCleared, Amount: 100000},
	})
	txRepo.lastReconciledBalances[1] = 50000

	entries, lastBal, err := svc.GetUnreconciledByAccount(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if lastBal != 50000 {
		t.Errorf("expected lastReconciledBalance 50000, got %d", lastBal)
	}
}

func TestGetUnreconciledByAccount_FirstTime_ReturnsZeroBalance(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	// no entry in lastReconciledBalances → mock returns 0

	_, lastBal, err := svc.GetUnreconciledByAccount(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lastBal != 0 {
		t.Errorf("expected 0 for first reconcile, got %d", lastBal)
	}
}

// ── ReconcileTransactions ─────────────────────────────────────────────────────

func TestReconcileTransactions_FirstReconcile_ZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
		{ID: 11, Amount: -50000},
	})
	// No prior balance (first reconcile) → lastReconciledBalance = 0.
	// Cleared = 100000 + (-50000) = 50000; statementBalance = 50000 → diff = 0.

	diff, err := svc.ReconcileTransactions(context.Background(), 1, 50000, []int64{10, 11})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected diff 0, got %d", diff)
	}
	// New last reconciled balance should be 0 + 50000 = 50000.
	if txRepo.lastReconciledBalances[1] != 50000 {
		t.Errorf("expected persisted balance 50000, got %d", txRepo.lastReconciledBalances[1])
	}
	if len(txRepo.setLastReconciledBalCalls) != 1 {
		t.Errorf("expected 1 SetLastReconciledBalance call, got %d", len(txRepo.setLastReconciledBalCalls))
	}
}

func TestReconcileTransactions_SubsequentReconcile_CarriedBalance(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 20, Amount: -15000}, // expense of $150
	})
	// lastReconciledBalance = 20000 ($200 from prior session).
	txRepo.lastReconciledBalances[1] = 20000
	// Bank now shows $50: diff = 5000 − (20000 + (−15000)) = 5000 − 5000 = 0.

	diff, err := svc.ReconcileTransactions(context.Background(), 1, 5000, []int64{20})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected diff 0, got %d", diff)
	}
	// New last reconciled balance = 20000 + (−15000) = 5000.
	if txRepo.lastReconciledBalances[1] != 5000 {
		t.Errorf("expected persisted balance 5000, got %d", txRepo.lastReconciledBalances[1])
	}
}

func TestReconcileTransactions_ForcedReconcile_PersistsActualCleared(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 30, Amount: 20000}, // $200 transactions
	})
	// No prior balance. User enters $150 (bank shows $150) but cleared $200 — forced.
	// diff = 15000 − (0 + 20000) = −5000 (OVER $50).
	// Regardless of force, the service always reconciles and persists lastBal+cleared = 20000.

	diff, err := svc.ReconcileTransactions(context.Background(), 1, 15000, []int64{30})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != -5000 {
		t.Errorf("expected diff -5000, got %d", diff)
	}
	// Persisted balance should be 0 + 20000 = 20000 (actual cleared, not statementBalance).
	if txRepo.lastReconciledBalances[1] != 20000 {
		t.Errorf("expected persisted balance 20000, got %d", txRepo.lastReconciledBalances[1])
	}
}

func TestReconcileTransactions_NonZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})
	// lastBal = 0; cleared = 100000; statementBalance = 120000 → diff = 20000 (SHORT).

	diff, err := svc.ReconcileTransactions(context.Background(), 1, 120000, []int64{10})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 20000 {
		t.Errorf("expected diff 20000, got %d", diff)
	}
	if len(txRepo.markSplitsReconciledCalls) != 1 {
		t.Fatalf("expected MarkSplitsReconciledByAccount to be called")
	}
}

func TestReconcileTransactions_UnknownTxID(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	_, err := svc.ReconcileTransactions(context.Background(), 1, 100000, []int64{10, 99})

	if err == nil {
		t.Fatal("expected error for unknown transaction ID, got nil")
	}
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not be called when validation fails")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when validation fails")
	}
}

func TestReconcileTransactions_AlreadyReconciledID(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	_, err := svc.ReconcileTransactions(context.Background(), 1, 100000, []int64{10, 20})

	if err == nil {
		t.Fatal("expected error for already-reconciled ID, got nil")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when validation fails")
	}
}

func TestReconcileTransactions_EmptyIDs(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})

	_, err := svc.ReconcileTransactions(context.Background(), 1, 100000, []int64{})

	if err == nil {
		t.Fatal("expected error for empty IDs, got nil")
	}
}

func TestReconcileTransactions_AccountNotFound(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.ReconcileTransactions(context.Background(), 99, 100000, []int64{10})

	if err == nil {
		t.Fatal("expected error for unknown account, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected service.ErrNotFound, got %T: %v", err, err)
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when account lookup fails")
	}
}

func TestReconcileTransactions_GetLastReconciledBalance_Error(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	txRepo.getLastReconciledBalErr[1] = fmt.Errorf("db error")

	_, err := svc.ReconcileTransactions(context.Background(), 1, 50000, []int64{10})

	if err == nil {
		t.Fatal("expected error when GetLastReconciledBalance fails")
	}
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not be called on GetLastReconciledBalance error")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called on GetLastReconciledBalance error")
	}
}

func TestReconcileTransactions_SetLastReconciledBalance_Error(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 50000},
	})
	txRepo.setLastReconciledBalErr[1] = fmt.Errorf("persist error")

	// Note: under ExecTx the marks are rolled back atomically when SetLastReconciledBalance
	// fails. The mock does not simulate rollback, so we do not assert on markSplitsReconciledCalls.
	_, err := svc.ReconcileTransactions(context.Background(), 1, 50000, []int64{10})

	if err == nil {
		t.Fatal("expected error when SetLastReconciledBalance fails")
	}
}

func TestGetUnreconciledByAccount_GetLastReconciledBalance_Error(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	txRepo.getLastReconciledBalErr[1] = fmt.Errorf("db error")

	_, _, err := svc.GetUnreconciledByAccount(context.Background(), 1)

	if err == nil {
		t.Fatal("expected error when GetLastReconciledBalance fails in GetUnreconciledByAccount")
	}
}

func TestReconcileTransactions_MarkSplitsAffectsFewerRowsThanTxIDs_ReturnsError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
		{ID: 11, Amount: -50000},
	})
	// Simulate the store updating only 1 row even though 2 txIDs were requested —
	// meaning txID 11 had no split for this account.
	one := int64(1)
	txRepo.markSplitsRowsOverride = &one

	_, err := svc.ReconcileTransactions(context.Background(), 1, 50000, []int64{10, 11})

	if err == nil {
		t.Fatal("expected error when fewer rows were affected than txIDs, got nil")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when the rows guard fires")
	}
}

func TestReconcileTransactions_ExecTxFailure_ReturnsError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{{ID: 10, Amount: 50000}})

	tm := &mockTransactionManager{accRepo: accRepo, txRepo: txRepo, failTx: true}
	svc := NewTransactionService(txRepo, accRepo, tm, defaultConfig())

	_, err := svc.ReconcileTransactions(context.Background(), 1, 50000, []int64{10})

	if err == nil {
		t.Fatal("expected error when ExecTx fails, got nil")
	}
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not be called when ExecTx itself fails")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when ExecTx itself fails")
	}
}

// ── PreviewReconcile ─────────────────────────────────────────────────────────

func TestPreviewReconcile_ZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
		{ID: 11, Amount: -50000},
	})

	diff, err := svc.PreviewReconcile(context.Background(), 1, 50000, []int64{10, 11})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected diff 0, got %d", diff)
	}
}

func TestPreviewReconcile_NonZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	diff, err := svc.PreviewReconcile(context.Background(), 1, 120000, []int64{10})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 20000 {
		t.Errorf("expected diff 20000, got %d", diff)
	}
}

func TestPreviewReconcile_InvalidID(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	_, err := svc.PreviewReconcile(context.Background(), 1, 100000, []int64{10, 99})

	if err == nil {
		t.Fatal("expected error for unknown ID, got nil")
	}
}

func TestPreviewReconcile_EmptyIDs(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})

	_, err := svc.PreviewReconcile(context.Background(), 1, 100000, []int64{})

	if err == nil {
		t.Fatal("expected error for empty IDs, got nil")
	}
}

func TestPreviewReconcile_AccountNotFound(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.PreviewReconcile(context.Background(), 99, 100000, []int64{10})

	if err == nil {
		t.Fatal("expected error for missing account, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected service.ErrNotFound, got %T: %v", err, err)
	}
}

func TestPreviewReconcile_WithPriorBalance(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 20, Amount: 3000},
	})
	txRepo.lastReconciledBalances[1] = 2000

	// diff = statementBalance - (lastBalance + cleared) = 5000 - (2000 + 3000) = 0
	diff, err := svc.PreviewReconcile(context.Background(), 1, 5000, []int64{20})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected diff 0, got %d", diff)
	}
}

func TestPreviewReconcile_DuplicateIDs_ReturnsError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	_, err := svc.PreviewReconcile(context.Background(), 1, 200000, []int64{10, 10})

	if err == nil {
		t.Fatal("expected error for duplicate transaction IDs, got nil")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "transactions" {
		t.Errorf("expected field 'transactions', got %q", ve.Field)
	}
}

func TestReconcileTransactions_DuplicateIDs_ReturnsError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	_, err := svc.ReconcileTransactions(context.Background(), 1, 200000, []int64{10, 10})

	if err == nil {
		t.Fatal("expected error for duplicate transaction IDs, got nil")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "transactions" {
		t.Errorf("expected field 'transactions', got %q", ve.Field)
	}
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not be called when validation fails")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when validation fails")
	}
}

func TestPreviewReconcile_DoesNotMutate(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 50000},
	})

	_, err := svc.PreviewReconcile(context.Background(), 1, 50000, []int64{10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no writes happened: markSplitsReconciledCalls should be empty.
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Errorf("expected no markSplits calls, got %d", len(txRepo.markSplitsReconciledCalls))
	}
	// lastReconciledBalances should be unchanged (still 0 / absent).
	if bal := txRepo.lastReconciledBalances[1]; bal != 0 {
		t.Errorf("expected last reconciled balance unchanged at 0, got %d", bal)
	}
}

func TestReconcileTransactions_ReadsInsideExecTx(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 50000},
	})

	diff, err := svc.ReconcileTransactions(context.Background(), 1, 50000, []int64{10})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected diff 0, got %d", diff)
	}
	if txRepo.lastReconciledBalances[1] != 50000 {
		t.Errorf("expected persisted balance 50000, got %d", txRepo.lastReconciledBalances[1])
	}
}

func TestReconcileTransactions_UnknownID_InsideExecTx_NoWrites(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 50000},
	})
	txRepo.lastReconciledBalances[1] = 20000

	_, err := svc.ReconcileTransactions(context.Background(), 1, 70000, []int64{10, 99})

	if err == nil {
		t.Fatal("expected error for unknown transaction ID, got nil")
	}
	// Validation error from inside ExecTx must roll back: no marks, no balance change.
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not persist when validation fails inside ExecTx")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not persist when validation fails inside ExecTx")
	}
	// Original balance must be unchanged.
	if txRepo.lastReconciledBalances[1] != 20000 {
		t.Errorf("expected last reconciled balance unchanged at 20000, got %d", txRepo.lastReconciledBalances[1])
	}
}
