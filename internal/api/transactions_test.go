// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestHandleTransactionByID_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	tx := seedTransaction(t, svc, "Assets:Cash", "Expenses:Coffee",
		500, 1733184000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	resp, err := http.Get(ts.URL + "/api/transactions/" + itoa(tx.ID))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var got model.TransactionDetail
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != tx.ID || got.Description != "Coffee" {
		t.Errorf("got %+v", got)
	}
	if len(got.Splits) != 2 {
		t.Errorf("expected 2 splits, got %d", len(got.Splits))
	}
}

func TestHandleTransactionByID_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/transactions/9999")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleTransactionByID_BadPath(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/transactions/abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}
