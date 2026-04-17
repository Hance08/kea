package service

import (
	"testing"

	"github.com/hance08/kea/internal/model"
)

// helper: build a ReconcileEntry slice for a given accountID in the mock
func seedUnreconciled(txRepo *mockTransactionRepo, accountID int64, entries []*model.ReconcileEntry) {
	txRepo.unreconciledByAccount[accountID] = entries
}

func TestReconcileTransactions_ZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Timestamp: 1000, Description: "Salary", Status: model.StatusCleared, Amount: 100000},
		{ID: 11, Timestamp: 1001, Description: "Rent", Status: model.StatusCleared, Amount: -50000},
	})

	// statementBalance = 50000 = 100000 + (-50000)
	diff, err := svc.ReconcileTransactions(1, 50000, []int64{10, 11})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected difference 0, got %d", diff)
	}
	if len(txRepo.markSplitsReconciledCalls) != 1 {
		t.Errorf("expected 1 MarkSplitsReconciledByAccount call, got %d", len(txRepo.markSplitsReconciledCalls))
	}
}

func TestReconcileTransactions_NonZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Timestamp: 1000, Description: "Salary", Status: model.StatusCleared, Amount: 100000},
	})

	// statementBalance = 120000, but we're only reconciling 100000 → diff = 20000
	diff, err := svc.ReconcileTransactions(1, 120000, []int64{10})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 20000 {
		t.Errorf("expected difference 20000, got %d", diff)
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
		{ID: 10, Timestamp: 1000, Description: "Salary", Status: model.StatusCleared, Amount: 100000},
	})

	_, err := svc.ReconcileTransactions(1, 100000, []int64{10, 99}) // 99 is unknown

	if err == nil {
		t.Fatal("expected error for unknown transaction ID, got nil")
	}
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not be called when validation fails")
	}
}

func TestReconcileTransactions_AlreadyReconciledID(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	// unreconciledByAccount does NOT contain ID 20 (it's already reconciled)
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Timestamp: 1000, Description: "Salary", Status: model.StatusCleared, Amount: 100000},
	})

	_, err := svc.ReconcileTransactions(1, 100000, []int64{10, 20}) // 20 not in unreconciled set

	if err == nil {
		t.Fatal("expected error for already-reconciled ID, got nil")
	}
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not be called when validation fails")
	}
}

func TestReconcileTransactions_EmptyIDs(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})

	_, err := svc.ReconcileTransactions(1, 100000, []int64{})

	if err == nil {
		t.Fatal("expected error for empty IDs, got nil")
	}
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not be called when IDs are empty")
	}
}

func TestReconcileTransactions_AccountNotFound(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	// account ID 99 does not exist in accRepo

	_, err := svc.ReconcileTransactions(99, 100000, []int64{10})

	if err == nil {
		t.Fatal("expected error for unknown account, got nil")
	}
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not be called when account lookup fails")
	}
}
