// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/hance08/kea/internal/model"
)

func TestHandleAccountByID_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp, err := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var got model.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != acc.ID || got.Name != "Assets:Cash" {
		t.Errorf("got %+v", got)
	}
}

func TestHandleAccountByID_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)

	resp, err := http.Get(ts.URL + "/api/accounts/9999")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "not_found" {
		t.Errorf("error code: got %q", body["error"])
	}
}

func TestHandleAccountByID_BadPath(t *testing.T) {
	ts, _ := newServerWithStore(t)

	resp, err := http.Get(ts.URL + "/api/accounts/abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// itoa is a tiny helper used across handler tests.
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestHandleAccountByName_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank:Checking", model.AccountTypeAsset, 0)

	resp, err := http.Get(ts.URL + "/api/accounts/by-name?name=Assets:Bank:Checking")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var got model.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Assets:Bank:Checking" {
		t.Errorf("got %q", got.Name)
	}
}

func TestHandleAccountByName_MissingName(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/accounts/by-name")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["field"] != "name" {
		t.Errorf("field: got %q", body["field"])
	}
}

func TestHandleAccountByName_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/accounts/by-name?name=Missing")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleAccountBalance_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	acc := seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 50000)

	resp, err := http.Get(ts.URL + "/api/accounts/" + itoa(acc.ID) + "/balance")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var got struct {
		AccountID int64  `json:"account_id"`
		Amount    int64  `json:"amount"`
		Currency  string `json:"currency"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.AccountID != acc.ID || got.Amount != 50000 || got.Currency != "USD" {
		t.Errorf("got %+v", got)
	}
}

func TestHandleAccountBalance_NotFound(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/accounts/9999/balance")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleAccountTree_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp, err := http.Get(ts.URL + "/api/accounts/tree")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var roots []*model.AccountNode
	if err := json.NewDecoder(resp.Body).Decode(&roots); err != nil {
		t.Fatalf("decode: %v", err)
	}

	names := map[string]bool{}
	for _, n := range roots {
		if n.Account != nil {
			names[n.Account.Name] = true
		}
	}
	if !names["Assets:Bank"] || !names["Assets:Cash"] {
		t.Errorf("expected both seeded roots in tree; got names %v", names)
	}
}
