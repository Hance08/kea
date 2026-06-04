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

	coffee := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)
	salary := seedTransaction(t, svc, "Revenue:Salary", "Assets:Bank", 500000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

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

	byID := make(map[int64]model.ReconcileEntry, len(got.Entries))
	for _, e := range got.Entries {
		byID[e.ID] = e
	}

	coffeeEntry, ok := byID[coffee.ID]
	if !ok {
		t.Fatalf("coffee transaction id %d missing from response", coffee.ID)
	}
	if coffeeEntry.Amount != -450 {
		t.Errorf("coffee amount: got %d, want -450", coffeeEntry.Amount)
	}
	if coffeeEntry.OffsetAccount != "Expenses:Coffee" {
		t.Errorf("coffee offset_account: got %q, want %q", coffeeEntry.OffsetAccount, "Expenses:Coffee")
	}

	salaryEntry, ok := byID[salary.ID]
	if !ok {
		t.Fatalf("salary transaction id %d missing from response", salary.ID)
	}
	if salaryEntry.Amount != 500000 {
		t.Errorf("salary amount: got %d, want 500000", salaryEntry.Amount)
	}
	if salaryEntry.OffsetAccount != "Revenue:Salary" {
		t.Errorf("salary offset_account: got %q, want %q", salaryEntry.OffsetAccount, "Revenue:Salary")
	}
}

func TestHandleListUnreconciled_AccountIsolation(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	bank := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	savings := seedAccount(t, svc, "Assets:Savings", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	bankTx := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 100, 1735689600, "Bank coffee", model.TxTypeExpense, model.StatusCleared)
	savingsTx := seedTransaction(t, svc, "Assets:Savings", "Expenses:Coffee", 200, 1735776000, "Savings coffee", model.TxTypeExpense, model.StatusCleared)

	// Bank account should only see its own transaction.
	status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, bank.ID))
	if status != http.StatusOK {
		t.Fatalf("bank status: got %d, want 200; body=%s", status, body)
	}
	var bankResp struct {
		Entries []model.ReconcileEntry `json:"entries"`
	}
	if err := json.Unmarshal(body, &bankResp); err != nil {
		t.Fatalf("bank unmarshal: %v", err)
	}
	if len(bankResp.Entries) != 1 {
		t.Fatalf("bank entries: got %d, want 1", len(bankResp.Entries))
	}
	if bankResp.Entries[0].ID != bankTx.ID {
		t.Errorf("bank entry id: got %d, want %d", bankResp.Entries[0].ID, bankTx.ID)
	}

	// Savings account should only see its own transaction.
	status, body = getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, savings.ID))
	if status != http.StatusOK {
		t.Fatalf("savings status: got %d, want 200; body=%s", status, body)
	}
	var savingsResp struct {
		Entries []model.ReconcileEntry `json:"entries"`
	}
	if err := json.Unmarshal(body, &savingsResp); err != nil {
		t.Fatalf("savings unmarshal: %v", err)
	}
	if len(savingsResp.Entries) != 1 {
		t.Fatalf("savings entries: got %d, want 1", len(savingsResp.Entries))
	}
	if savingsResp.Entries[0].ID != savingsTx.ID {
		t.Errorf("savings entry id: got %d, want %d", savingsResp.Entries[0].ID, savingsTx.ID)
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
		Entries               []model.ReconcileEntry `json:"entries"`
		LastReconciledBalance int64                  `json:"last_reconciled_balance"`
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
	if got.LastReconciledBalance != 0 {
		t.Errorf("last_reconciled_balance: got %d, want 0", got.LastReconciledBalance)
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

func TestHandleReconcilePreview_ZeroDiff(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	seedAccount(t, svc, "Revenue:Salary", model.AccountTypeRevenue, 0)

	t1 := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)
	t2 := seedTransaction(t, svc, "Revenue:Salary", "Assets:Bank", 500000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

	// Net for Assets:Bank: -450 + 500000 = 499550. With lastBalance=0, statement 499550 → diff 0.
	payload := map[string]any{"statement_balance": 499550, "transaction_ids": []int64{t1.ID, t2.ID}}
	status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", status, body)
	}
	var got struct {
		Difference int64 `json:"difference"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.Difference != 0 {
		t.Errorf("difference: got %d, want 0", got.Difference)
	}
}

func TestHandleReconcilePreview_UnderShoot(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Revenue:Salary", model.AccountTypeRevenue, 0)
	tx := seedTransaction(t, svc, "Revenue:Salary", "Assets:Bank", 100000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

	payload := map[string]any{"statement_balance": 105000, "transaction_ids": []int64{tx.ID}}
	status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", status, body)
	}
	var got struct {
		Difference int64 `json:"difference"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.Difference != 5000 {
		t.Errorf("difference: got %d, want 5000", got.Difference)
	}
}

func TestHandleReconcilePreview_OverShoot(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Revenue:Salary", model.AccountTypeRevenue, 0)
	tx := seedTransaction(t, svc, "Revenue:Salary", "Assets:Bank", 100000, 1735776000, "Salary", model.TxTypeIncome, model.StatusCleared)

	payload := map[string]any{"statement_balance": 90000, "transaction_ids": []int64{tx.ID}}
	status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", status, body)
	}
	var got struct {
		Difference int64 `json:"difference"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.Difference != -10000 {
		t.Errorf("difference: got %d, want -10000", got.Difference)
	}
}

func TestHandleReconcilePreview_EmptyIDs(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

	payload := map[string]any{"statement_balance": 0, "transaction_ids": []int64{}}
	status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400; body=%s", status, body)
	}
	var got errorBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.Error != "validation_failed" {
		t.Errorf("error code: got %q, want validation_failed", got.Error)
	}
	if got.Field != "transactions" {
		t.Errorf("field: got %q, want transactions", got.Field)
	}
}

func TestHandleReconcilePreview_DuplicateIDs(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	tx := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)

	payload := map[string]any{"statement_balance": -450, "transaction_ids": []int64{tx.ID, tx.ID}}
	status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400; body=%s", status, body)
	}
	var got errorBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.Field != "transactions" {
		t.Errorf("field: got %q, want transactions", got.Field)
	}
}

func TestHandleReconcilePreview_IDNotInUnreconciledSet(t *testing.T) {
	ts, svc, st := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	tx := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)
	seedReconciledTransaction(t, st, src.ID, tx.ID)

	payload := map[string]any{"statement_balance": -450, "transaction_ids": []int64{tx.ID}}
	status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400; body=%s", status, body)
	}
	var got errorBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.Field != "transactions" {
		t.Errorf("field: got %q, want transactions", got.Field)
	}
}

func TestHandleReconcilePreview_UnknownAccount(t *testing.T) {
	ts, _, _ := newServerForWrite(t)

	payload := map[string]any{"statement_balance": 0, "transaction_ids": []int64{1}}
	status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/99999/reconcile/preview", ts.URL), payload)
	if status != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404; body=%s", status, body)
	}
	var got errorBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.Error != "not_found" {
		t.Errorf("error code: got %q, want not_found", got.Error)
	}
}

func TestHandleReconcilePreview_UnknownJSONField(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

	payload := map[string]any{
		"statement_balance": 0,
		"transaction_ids":   []int64{1},
		"extra_field":       "nope",
	}
	status, _ := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload)
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", status)
	}
}

func TestHandleReconcilePreview_DoesNotWrite(t *testing.T) {
	ts, svc, _ := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	tx := seedTransaction(t, svc, "Assets:Bank", "Expenses:Coffee", 450, 1735689600, "Coffee", model.TxTypeExpense, model.StatusCleared)

	payload := map[string]any{"statement_balance": -450, "transaction_ids": []int64{tx.ID}}
	if status, body := postJSON(t, fmt.Sprintf("%s/api/accounts/%d/reconcile/preview", ts.URL, src.ID), payload); status != http.StatusOK {
		t.Fatalf("preview: got %d; body=%s", status, body)
	}

	// Re-fetch unreconciled list — entry must still be present.
	status, body := getJSON(t, fmt.Sprintf("%s/api/accounts/%d/unreconciled", ts.URL, src.ID))
	if status != http.StatusOK {
		t.Fatalf("list: got %d; body=%s", status, body)
	}
	var got struct {
		Entries []model.ReconcileEntry `json:"entries"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if len(got.Entries) != 1 {
		t.Errorf("entries: got %d, want 1 (preview must not write)", len(got.Entries))
	}
}

func TestHandleReconcilePreview_BadPathParam(t *testing.T) {
	ts, _, _ := newServerForWrite(t)
	payload := map[string]any{"statement_balance": 0, "transaction_ids": []int64{1}}
	status, body := postJSON(t, ts.URL+"/api/accounts/not-a-number/reconcile/preview", payload)
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400; body=%s", status, body)
	}
}
