// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestHandleCreateTransaction_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"Coffee",
		"timestamp":1700000000,
		"status":"Cleared",
		"type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", resp.StatusCode)
	}
	var got model.TransactionDetail
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID == 0 || got.Description != "Coffee" || got.Type != model.TxTypeExpense {
		t.Errorf("got %+v", got)
	}
	if len(got.Splits) != 2 {
		t.Fatalf("splits: got %d, want 2", len(got.Splits))
	}
	for _, s := range got.Splits {
		if s.AccountID == 0 || s.AccountName == "" || s.AccountType == "" {
			t.Errorf("split missing resolved fields: %+v", s)
		}
	}
}

func TestHandleCreateTransaction_Unbalanced(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":400}
		],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleCreateTransaction_OneSplit(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)

	body := `{
		"splits":[{"account_name":"Assets:Bank","amount":-500}],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "splits" {
		t.Errorf("field: got %q", errBody["field"])
	}
}

func TestHandleCreateTransaction_TypeMismatch(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	// Splits describe an Expense, but type claims Income.
	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Income"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleCreateTransaction_ReconciledOnCreate(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"x","timestamp":1700000000,"status":"Reconciled","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "status" {
		t.Errorf("field: got %q", errBody["field"])
	}
}

func TestHandleCreateTransaction_EmptyDescription(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleCreateTransaction_MemoTooLong(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	longMemo := strings.Repeat("a", model.MemoMaxLength+1)
	body, err := json.Marshal(map[string]any{
		"splits": []map[string]any{
			{"account_name": "Assets:Bank", "amount": -500, "memo": longMemo},
			{"account_name": "Expenses:Coffee", "amount": 500},
		},
		"description": "x",
		"timestamp":   1700000000,
		"status":      "Cleared",
		"type":        "Expense",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp := postJSON(t, ts.URL+"/api/transactions", string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "memo" {
		t.Errorf("field: got %q, want memo", errBody["field"])
	}
}

func TestHandleCreateTransaction_HiddenAccount(t *testing.T) {
	ts, svc := newServerWithStore(t)
	bank := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	if _, err := svc.Account().UpdateAccountMetadata(t.Context(), bank.ID, "", true); err != nil {
		t.Fatalf("hide: %v", err)
	}

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:Coffee","amount":500}
		],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// A split referencing a nonexistent account currently returns 500 because
// CreateTransaction surfaces repository.ErrNotFound (not service.ErrNotFound),
// and mapError only matches the service-level sentinel. The spec records this
// as a known rough edge to be fixed outside this plan.
func TestHandleCreateTransaction_NonexistentAccount_Currently500(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)

	body := `{
		"splits":[
			{"account_name":"Assets:Bank","amount":-500},
			{"account_name":"Expenses:DoesNotExist","amount":500}
		],
		"description":"x","timestamp":1700000000,"status":"Cleared","type":"Expense"
	}`
	resp := postJSON(t, ts.URL+"/api/transactions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500 (known rough edge — fix is out of scope)", resp.StatusCode)
	}
}
