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

func postJSON(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func TestHandleCreateAccount_OK(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"Assets:Cash","type":"A","currency":"USD","description":"","balance":0}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", resp.StatusCode)
	}
	var got model.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Assets:Cash" || got.Type != model.AccountTypeAsset || got.Currency != "USD" {
		t.Errorf("got %+v", got)
	}
	if got.ID == 0 {
		t.Errorf("expected non-zero id")
	}
}

func TestHandleCreateAccount_WithBalance(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"Assets:Bank","type":"A","currency":"USD","description":"","balance":100000}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", resp.StatusCode)
	}
	var acc model.Account
	if err := json.NewDecoder(resp.Body).Decode(&acc); err != nil {
		t.Fatalf("decode: %v", err)
	}

	balResp, err := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID) + "/balance")
	if err != nil {
		t.Fatalf("GET balance: %v", err)
	}
	defer balResp.Body.Close()
	var bal balanceResponse
	if err := json.NewDecoder(balResp.Body).Decode(&bal); err != nil {
		t.Fatalf("decode balance: %v", err)
	}
	if bal.Amount != 100000 {
		t.Errorf("balance: got %d, want 100000", bal.Amount)
	}

	sysResp, err := http.Get(ts.URL + "/api/accounts/by-name?name=Equity:OpeningBalances_USD")
	if err != nil {
		t.Fatalf("GET sys account: %v", err)
	}
	defer sysResp.Body.Close()
	if sysResp.StatusCode != http.StatusOK {
		t.Errorf("system equity account not found: %d", sysResp.StatusCode)
	}
}

func TestHandleCreateAccount_LiabilityBalanceSignReversed(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"Liabilities:CreditCard","type":"L","currency":"USD","description":"","balance":50000}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d", resp.StatusCode)
	}
	var acc model.Account
	_ = json.NewDecoder(resp.Body).Decode(&acc)

	balResp, err := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID) + "/balance")
	if err != nil {
		t.Fatalf("GET balance: %v", err)
	}
	defer balResp.Body.Close()
	var bal balanceResponse
	_ = json.NewDecoder(balResp.Body).Decode(&bal)
	// Liability opening: liability split = -amount, so the stored balance is -50000.
	if bal.Amount != -50000 {
		t.Errorf("liability balance: got %d, want -50000", bal.Amount)
	}
}

func TestHandleCreateAccount_DuplicateName(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	body := `{"name":"Assets:Cash","type":"A","currency":"USD","description":"","balance":0}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "already_exists" {
		t.Errorf("error code: got %q, want already_exists", errBody["error"])
	}
}

func TestHandleCreateAccount_CircularParent(t *testing.T) {
	ts, svc, st := newServerForWrite(t)
	// Create a valid asset parent, then poke its parent_id to point at itself.
	parent := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	injectParentSelfCycle(t, st, parent.ID)

	// Now attempting to create a child of "Assets:Bank" walks the cycle.
	body, _ := json.Marshal(map[string]any{
		"name":      "Assets:Bank:Checking",
		"type":      "A",
		"currency":  "USD",
		"parent_id": parent.ID,
		"balance":   0,
	})
	resp := postJSON(t, ts.URL+"/api/accounts", string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "circular_parent" {
		t.Errorf("error: got %q, want circular_parent", errBody["error"])
	}
}

func TestHandleCreateAccount_InvalidType(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"Assets:Cash","type":"Z","currency":"USD","description":"","balance":0}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "type" {
		t.Errorf("field: got %q, want type", errBody["field"])
	}
}

func TestHandleCreateAccount_EmptyName(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"","type":"A","currency":"USD","description":"","balance":0}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "name" {
		t.Errorf("field: got %q, want name", errBody["field"])
	}
}

func TestHandleCreateAccount_DescriptionTooLong(t *testing.T) {
	ts, _ := newServerWithStore(t)

	longDesc := strings.Repeat("a", model.DescriptionMaxLength+1)
	body, err := json.Marshal(map[string]any{
		"name":        "Assets:Cash",
		"type":        "A",
		"currency":    "USD",
		"description": longDesc,
		"balance":     0,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp := postJSON(t, ts.URL+"/api/accounts", string(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "description" {
		t.Errorf("field: got %q, want description", errBody["field"])
	}
}

func TestHandleCreateAccount_UnknownField(t *testing.T) {
	ts, _ := newServerWithStore(t)

	body := `{"name":"Assets:Cash","type":"A","currency":"USD","balance":0,"unknown_field":1}`
	resp := postJSON(t, ts.URL+"/api/accounts", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
}

func deleteReq(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	return resp
}

func TestHandleDeleteAccount_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := deleteReq(t, ts.URL+"/api/accounts/"+itoa(acc.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["deleted"] != true {
		t.Errorf("deleted: got %v, want true", body["deleted"])
	}
	if int64(body["id"].(float64)) != acc.ID {
		t.Errorf("id: got %v, want %d", body["id"], acc.ID)
	}

	getResp, err := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID))
	if err != nil {
		t.Fatalf("GET account: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("after delete, GET status: got %d, want 404", getResp.StatusCode)
	}
}

func TestHandleDeleteAccount_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := deleteReq(t, ts.URL+"/api/accounts/9999")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleDeleteAccount_BadPath(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := deleteReq(t, ts.URL+"/api/accounts/abc")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleDeleteAccount_HasChildren(t *testing.T) {
	ts, svc := newServerWithStore(t)
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

	resp := deleteReq(t, ts.URL+"/api/accounts/"+itoa(parent.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "validation_failed" {
		t.Errorf("error: got %q, want validation_failed", errBody["error"])
	}
}

func TestHandleDeleteAccount_HasTransactions(t *testing.T) {
	ts, svc := newServerWithStore(t)
	src := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 10000)
	dst := seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)
	_ = seedTransaction(t, svc, src.Name, dst.Name, 500, 1700000000, "Coffee", model.TxTypeExpense, model.StatusCleared)

	resp := deleteReq(t, ts.URL+"/api/accounts/"+itoa(src.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleDeleteAccount_SystemAccount(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 10000)

	sys, err := svc.Account().GetAccountByName(t.Context(), "Equity:OpeningBalances_USD")
	if err != nil {
		t.Fatalf("lookup sys account: %v", err)
	}

	resp := deleteReq(t, ts.URL+"/api/accounts/"+itoa(sys.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "not_editable" {
		t.Errorf("error: got %q, want not_editable", errBody["error"])
	}
}

func patchJSON(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", url, err)
	}
	return resp
}

func TestHandleUpdateAccount_NoFields(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "validation_failed" {
		t.Errorf("error: got %q", errBody["error"])
	}
}

func TestHandleUpdateAccount_RenameOnly(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), `{"name":"Assets:CashRenamed"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Assets:CashRenamed" {
		t.Errorf("name: got %q", got.Name)
	}

	oldResp, err := http.Get(ts.URL + "/api/accounts/by-name?name=Assets:Cash")
	if err != nil {
		t.Fatalf("GET old name: %v", err)
	}
	defer oldResp.Body.Close()
	if oldResp.StatusCode != http.StatusNotFound {
		t.Errorf("old name still resolvable: %d", oldResp.StatusCode)
	}
}

func TestHandleUpdateAccount_MetadataOnly(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), `{"description":"new desc","is_hidden":true}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Description != "new desc" || !got.IsHidden {
		t.Errorf("got %+v", got)
	}
}

func TestHandleUpdateAccount_RenameAndMetadata(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	body := `{"name":"Assets:Cash2","description":"d2","is_hidden":true}`
	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Assets:Cash2" || got.Description != "d2" || !got.IsHidden {
		t.Errorf("got %+v", got)
	}
}

func TestHandleUpdateAccount_NameEqualToCurrent_NoOp(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	body := `{"name":"Assets:Cash","description":"updated"}`
	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.Account
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.Name != "Assets:Cash" || got.Description != "updated" {
		t.Errorf("got %+v", got)
	}
}

func TestHandleUpdateAccount_RenameAcrossParentPath(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), `{"name":"Liabilities:CC"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["field"] != "name" {
		t.Errorf("field: got %q", errBody["field"])
	}
}

func TestHandleUpdateAccount_RenameSystemAccount(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 10000) // creates Equity:OpeningBalances_USD
	sys, err := svc.Account().GetAccountByName(t.Context(), "Equity:OpeningBalances_USD")
	if err != nil {
		t.Fatalf("lookup sys: %v", err)
	}

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(sys.ID), `{"name":"Equity:OpeningBalances_USD2"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", resp.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "not_editable" {
		t.Errorf("error: got %q", errBody["error"])
	}
}

func TestHandleUpdateAccount_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp := patchJSON(t, ts.URL+"/api/accounts/9999", `{"description":"x"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleUpdateAccount_UnknownField(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp := patchJSON(t, ts.URL+"/api/accounts/"+itoa(acc.ID), `{"description":"x","unknown":1}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}
