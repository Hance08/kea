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

func TestHandleCreateTransaction_ParentAccountInSplit(t *testing.T) {
	ts, svc := newServerWithStore(t)
	// Create a parent account and a child of it. The parent then has a child,
	// making it non-leaf and not selectable for transactions.
	parent := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	_, err := svc.Account().CreateAccount(t.Context(), model.CreateAccountInput{
		Name:     "Assets:Bank:Checking",
		Type:     model.AccountTypeAsset,
		Currency: "USD",
		ParentID: &parent.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	// Reference the non-leaf parent in a split.
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
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "account" {
		t.Errorf("field: got %q, want account", errBody["field"])
	}
}

func TestHandleDeleteTransaction_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	resp := deleteReq(t, ts.URL+"/api/transactions/"+itoa(d.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["deleted"] != true {
		t.Errorf("deleted: got %v", body["deleted"])
	}

	getResp, err := http.Get(ts.URL + "/api/transactions/" + itoa(d.ID))
	if err != nil {
		t.Fatalf("GET transaction: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("after delete: %d, want 404", getResp.StatusCode)
	}
}

func TestHandleDeleteTransaction_Pending(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusPending)

	resp := deleteReq(t, ts.URL+"/api/transactions/"+itoa(d.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestHandleDeleteTransaction_Reconciled(t *testing.T) {
	ts, svc, st := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)
	seedReconciledTransaction(t, st, src.ID, d.ID)

	resp := deleteReq(t, ts.URL+"/api/transactions/"+itoa(d.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "reconciled" {
		t.Errorf("error: got %q, want reconciled", errBody["error"])
	}
}

func TestHandleDeleteTransaction_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := deleteReq(t, ts.URL+"/api/transactions/9999")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleDeleteTransaction_BadPath(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := deleteReq(t, ts.URL+"/api/transactions/abc")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleUpdateTransactionStatus_PendingToCleared(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusPending)

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID)+"/status", `{"status":"Cleared"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.TransactionDetail
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.Status != model.StatusCleared {
		t.Errorf("status: got %v, want Cleared", got.Status)
	}
}

func TestHandleUpdateTransactionStatus_ClearedToPending(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID)+"/status", `{"status":"Pending"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestHandleUpdateTransactionStatus_RejectsReconciledTarget(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID)+"/status", `{"status":"Reconciled"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "status" {
		t.Errorf("field: got %q", errBody["field"])
	}
}

func TestHandleUpdateTransactionStatus_OnReconciled(t *testing.T) {
	ts, svc, st := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)
	seedReconciledTransaction(t, st, src.ID, d.ID)

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID)+"/status", `{"status":"Cleared"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status: got %d, want 409", resp.StatusCode)
	}
}

func TestHandleUpdateTransactionStatus_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := patchJSON(t, ts.URL+"/api/transactions/9999/status", `{"status":"Cleared"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleUpdateTransaction_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)
	if len(d.Splits) != 2 {
		t.Fatalf("seed splits: %d", len(d.Splits))
	}

	bodyMap := map[string]any{
		"description": "Updated coffee",
		"timestamp":   1700000001,
		"status":      "Cleared",
		"type":        "Expense",
		"splits": []map[string]any{
			{"id": d.Splits[0].ID, "account_id": d.Splits[0].AccountID, "amount": -750, "currency": "USD"},
			{"id": d.Splits[1].ID, "account_id": d.Splits[1].AccountID, "amount": 750, "currency": "USD"},
		},
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID), string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.TransactionDetail
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Description != "Updated coffee" || got.Timestamp != 1700000001 {
		t.Errorf("got %+v", got)
	}
	if len(got.Splits) != 2 {
		t.Fatalf("splits: %d", len(got.Splits))
	}
	if got.Splits[0].Amount != 750 && got.Splits[0].Amount != -750 {
		t.Errorf("amount: %d", got.Splits[0].Amount)
	}
}

func TestHandleUpdateTransaction_Reconciled(t *testing.T) {
	ts, svc, st := newServerForWrite(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)
	seedReconciledTransaction(t, st, src.ID, d.ID)

	body, _ := json.Marshal(map[string]any{
		"description": "x",
		"timestamp":   1700000001,
		"status":      "Cleared",
		"type":        "Expense",
		"splits": []map[string]any{
			{"id": d.Splits[0].ID, "account_id": d.Splits[0].AccountID, "amount": -500},
			{"id": d.Splits[1].ID, "account_id": d.Splits[1].AccountID, "amount": 500},
		},
	})

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID), string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "reconciled" {
		t.Errorf("error: got %q", errBody["error"])
	}
}

func TestHandleUpdateTransaction_BodyIDRejected(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	// Body carries "id":99 — DisallowUnknownFields should reject because
	// UpdateTransactionInput.ID is json:"-" (invisible to the decoder).
	body, _ := json.Marshal(map[string]any{
		"id":          99,
		"description": "x",
		"timestamp":   1700000001,
		"status":      "Cleared",
		"type":        "Expense",
		"splits": []map[string]any{
			{"id": d.Splits[0].ID, "account_id": d.Splits[0].AccountID, "amount": -500},
			{"id": d.Splits[1].ID, "account_id": d.Splits[1].AccountID, "amount": 500},
		},
	})

	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID), string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleUpdateTransaction_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	body, _ := json.Marshal(map[string]any{
		"description": "x", "timestamp": 1, "status": "Cleared", "type": "Expense",
		"splits": []map[string]any{
			{"account_id": 1, "amount": -1},
			{"account_id": 2, "amount": 1},
		},
	})
	resp := patchJSON(t, ts.URL+"/api/transactions/9999", string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleUpdateTransaction_Unbalanced(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	body, _ := json.Marshal(map[string]any{
		"description": "x", "timestamp": 1700000001, "status": "Cleared", "type": "Expense",
		"splits": []map[string]any{
			{"id": d.Splits[0].ID, "account_id": d.Splits[0].AccountID, "amount": -500},
			{"id": d.Splits[1].ID, "account_id": d.Splits[1].AccountID, "amount": 400},
		},
	})
	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d.ID), string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleUpdateTransaction_ForeignSplitID(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 100000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	d1 := seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "T1", model.TxTypeExpense, model.StatusCleared)
	d2 := seedTransaction(t, svc, src.Name, dst.Name, 700, 1700000001, "T2", model.TxTypeExpense, model.StatusCleared)

	// Try to update d1 with a split ID belonging to d2.
	body, _ := json.Marshal(map[string]any{
		"description": "x", "timestamp": 1700000002, "status": "Cleared", "type": "Expense",
		"splits": []map[string]any{
			{"id": d2.Splits[0].ID, "account_id": d2.Splits[0].AccountID, "amount": -500},
			{"id": d1.Splits[1].ID, "account_id": d1.Splits[1].AccountID, "amount": 500},
		},
	})
	resp := patchJSON(t, ts.URL+"/api/transactions/"+itoa(d1.ID), string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "splits" {
		t.Errorf("field: got %q", errBody["field"])
	}
}
