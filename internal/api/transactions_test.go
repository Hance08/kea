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

func TestHandleListTransactions_Empty(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/transactions")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var lr model.ListResult[*model.TransactionDetail]
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if lr.Items == nil {
		t.Errorf("Items must be non-nil slice")
	}
}

func TestHandleListTransactions_OrderingAndSplits(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:A", model.AccountTypeExpense, 0)
	seedAccount(t, svc, "Expenses:B", model.AccountTypeExpense, 0)

	older := seedTransaction(t, svc, "Assets:Cash", "Expenses:A", 100, 1000, "old", model.TxTypeExpense, model.StatusCleared)
	newer := seedTransaction(t, svc, "Assets:Cash", "Expenses:B", 200, 2000, "new", model.TxTypeExpense, model.StatusCleared)

	resp, err := http.Get(ts.URL + "/api/transactions")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var lr model.ListResult[*model.TransactionDetail]
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var newerIdx, olderIdx = -1, -1
	for i, tx := range lr.Items {
		switch tx.ID {
		case newer.ID:
			newerIdx = i
		case older.ID:
			olderIdx = i
		}
		if (tx.ID == newer.ID || tx.ID == older.ID) && len(tx.Splits) == 0 {
			t.Errorf("tx %d has no splits", tx.ID)
		}
	}
	if newerIdx == -1 || olderIdx == -1 {
		t.Fatalf("seeded txs missing: newer=%d older=%d", newerIdx, olderIdx)
	}
	if newerIdx >= olderIdx {
		t.Errorf("expected newer before older (DESC), got newer=%d older=%d", newerIdx, olderIdx)
	}
}

func TestHandleListTransactions_FilterByAccountAndType(t *testing.T) {
	ts, svc := newServerWithStore(t)
	cash := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:A", model.AccountTypeExpense, 0)
	seedAccount(t, svc, "Revenue:Salary", model.AccountTypeRevenue, 0)

	exp := seedTransaction(t, svc, "Assets:Cash", "Expenses:A", 100, 1000, "exp", model.TxTypeExpense, model.StatusCleared)
	inc := seedTransaction(t, svc, "Revenue:Salary", "Assets:Cash", 200, 2000, "inc", model.TxTypeIncome, model.StatusCleared)

	resp, err := http.Get(ts.URL + "/api/transactions?account_id=" + itoa(cash.ID) + "&type=Expense")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var lr model.ListResult[*model.TransactionDetail]
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sawExp, sawInc := false, false
	for _, tx := range lr.Items {
		if tx.ID == exp.ID {
			sawExp = true
		}
		if tx.ID == inc.ID {
			sawInc = true
		}
	}
	if !sawExp {
		t.Errorf("expected expense tx in filtered list")
	}
	if sawInc {
		t.Errorf("income tx leaked into Expense filter")
	}
}

func TestHandleListTransactions_BadStatus(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/transactions?status=Wat")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["field"] != "status" {
		t.Errorf("field: got %q", body["field"])
	}
}

func TestHandleListTransactions_CrossFieldDate(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/transactions?start_time=200&end_time=100")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}
