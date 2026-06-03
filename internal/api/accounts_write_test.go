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

	balResp, _ := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID) + "/balance")
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
