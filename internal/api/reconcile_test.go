// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestHandleListUnreconciled_Empty(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	acc := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

	status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, acc.ID))
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", status, body)
	}
	var got struct {
		Entries               []model.ReconcileEntry `json:"entries"`
		LastReconciledBalance int64                  `json:"last_reconciled_balance"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if len(got.Entries) != 0 {
		t.Errorf("entries: got %v, want []", got.Entries)
	}
	if got.LastReconciledBalance != 0 {
		t.Errorf("last_reconciled_balance: got %d, want 0", got.LastReconciledBalance)
	}
}

func TestHandleListUnreconciled_WithTransactions(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	seedAccount(t, svc, "Revenue:Salary", model.AccountTypeRevenue, 0)

	seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)
	seedTransaction(t, svc, "Revenue:Salary", "Assets:Bank", 500000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

	status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, src.ID))
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", status, body)
	}
	var got struct {
		Entries               []model.ReconcileEntry `json:"entries"`
		LastReconciledBalance int64                  `json:"last_reconciled_balance"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("entries length: got %d, want 2", len(got.Entries))
	}
	if got.LastReconciledBalance != 0 {
		t.Errorf("last_reconciled_balance: got %d, want 0", got.LastReconciledBalance)
	}
}

func TestHandleListUnreconciled_ExcludesReconciled(t *testing.T) {
	ts, svc, st := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	pinned := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Pinned", model.TxTypeExpense, model.StatusCleared)
	open := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 200, 1735776000, "Open", model.TxTypeExpense, model.StatusCleared)

	seedReconciledTransaction(t, st, src.ID, pinned.ID)
	if err := st.SetLastReconciledBalance(t.Context(), src.ID, 250000); err != nil {
		t.Fatalf("SetLastReconciledBalance: %v", err)
	}

	status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, src.ID))
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", status, body)
	}
	var got struct {
		Entries               []model.ReconcileEntry `json:"entries"`
		LastReconciledBalance int64                  `json:"last_reconciled_balance"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("entries length: got %d, want 1 (pinned excluded)", len(got.Entries))
	}
	if got.Entries[0].ID != open.ID {
		t.Errorf("entry id: got %d, want %d (open)", got.Entries[0].ID, open.ID)
	}
	if got.LastReconciledBalance != 250000 {
		t.Errorf("last_reconciled_balance: got %d, want 250000", got.LastReconciledBalance)
	}
}

func TestHandleListUnreconciled_IncludesPending(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Pending", model.TxTypeExpense, model.StatusPending)

	status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, src.ID))
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", status, body)
	}
	var got struct {
		Entries []model.ReconcileEntry `json:"entries"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("entries length: got %d, want 1", len(got.Entries))
	}
	if got.Entries[0].Status != model.StatusPending {
		t.Errorf("entry status: got %v, want Pending", got.Entries[0].Status)
	}
}

func TestHandleListUnreconciled_UnknownAccount(t *testing.T) {
	ts, _, _ := newServerForWrite(t)

	status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/99999/unreconciled", ts.URL))
	if status != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404; body=%s", status, body)
	}
	var got errorBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.Error != "not_found" {
		t.Errorf("error code: got %q, want %q", got.Error, "not_found")
	}
}

func TestHandleListUnreconciled_BadPathParam(t *testing.T) {
	ts, _, _ := newServerForWrite(t)

	status, body := getJSON(t, ts.URL+"/api/accounts/not-a-number/unreconciled")
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400; body=%s", status, body)
	}
}
