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

func TestHandleListAccounts_EmptyStore(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/accounts")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var lr model.ListResult[*model.Account]
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if lr.Items == nil {
		t.Errorf("Items should be a non-nil slice")
	}
}

func TestHandleListAccounts_FiltersHidden(t *testing.T) {
	ts, svc := newServerWithStore(t)
	visible := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	hidden := seedAccount(t, svc, "Assets:OldBank", model.AccountTypeAsset, 0)
	if _, err := svc.Account().UpdateAccountMetadata(
		t.Context(), hidden.ID, hidden.Description, true,
	); err != nil {
		t.Fatalf("hide: %v", err)
	}

	resp, err := http.Get(ts.URL + "/api/accounts?type=A")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var lr model.ListResult[*model.Account]
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, a := range lr.Items {
		if a.ID == hidden.ID {
			t.Errorf("hidden account leaked into default list")
		}
	}

	resp2, err := http.Get(ts.URL + "/api/accounts?type=A&include_hidden=true")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp2.Body.Close()
	var lr2 model.ListResult[*model.Account]
	if err := json.NewDecoder(resp2.Body).Decode(&lr2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sawHidden, sawVisible := false, false
	for _, a := range lr2.Items {
		if a.ID == hidden.ID {
			sawHidden = true
		}
		if a.ID == visible.ID {
			sawVisible = true
		}
	}
	if !sawHidden || !sawVisible {
		t.Errorf("include_hidden=true: sawHidden=%v sawVisible=%v", sawHidden, sawVisible)
	}
}

func TestHandleListAccounts_SearchPaginated(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:BankA", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Assets:BankB", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)

	resp, err := http.Get(ts.URL + "/api/accounts?q=Bank&limit=10&offset=0&include_count=true")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var lr model.ListResult[*model.Account]
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if lr.TotalCount < 2 {
		t.Errorf("expected TotalCount >= 2 for Bank matches, got %d", lr.TotalCount)
	}
	for _, a := range lr.Items {
		if a.Name == "Assets:Cash" {
			t.Errorf("Cash leaked into Bank search: %+v", a)
		}
	}
}

func TestHandleListAccounts_InvalidLimit(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/accounts?limit=abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["field"] != "limit" {
		t.Errorf("field: got %q", body["field"])
	}
}

func TestHandleListAccounts_InvalidType(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/accounts?type=Z")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleListBalances_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 125000)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 3500)

	resp, err := http.Get(ts.URL + "/api/balances")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var got model.ListResult[model.AccountBalance]
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.TotalCount < 2 {
		t.Fatalf("total_count: got %d, want >= 2 (system account may bump it)", got.TotalCount)
	}
	if got.Limit != 0 || got.Offset != 0 {
		t.Errorf("limit/offset: got %d/%d, want 0/0", got.Limit, got.Offset)
	}
	var foundBank, foundCash bool
	for _, row := range got.Items {
		switch row.Name {
		case "Assets:Bank":
			foundBank = true
			if row.Amount != 125000 {
				t.Errorf("Bank amount: got %d, want 125000", row.Amount)
			}
		case "Assets:Cash":
			foundCash = true
			if row.Amount != 3500 {
				t.Errorf("Cash amount: got %d, want 3500", row.Amount)
			}
		}
	}
	if !foundBank || !foundCash {
		t.Errorf("missing seeded rows: bank=%v cash=%v", foundBank, foundCash)
	}
	// Sorted by AccountID ascending.
	for i := 1; i < len(got.Items); i++ {
		if got.Items[i-1].AccountID > got.Items[i].AccountID {
			t.Errorf("rows not sorted by AccountID at index %d", i)
		}
	}
}

func TestHandleListBalances_AsOfDefault(t *testing.T) {
	ts, _ := newServerWithStore(t)

	resp, err := http.Get(ts.URL + "/api/balances")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	// We do not assert the as_of value the service was called with — the spec
	// accepts this gap deliberately (no nowFunc test seam).
}

func TestHandleListBalances_HistoricalAsOf(t *testing.T) {
	ts, _ := newServerWithStore(t)

	resp, err := http.Get(ts.URL + "/api/balances?as_of=1700000000")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestHandleListBalances_InvalidAsOf(t *testing.T) {
	ts, _ := newServerWithStore(t)

	resp, err := http.Get(ts.URL + "/api/balances?as_of=banana")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
		Field string `json:"field"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error != "validation_failed" {
		t.Errorf("error code: got %q, want %q", body.Error, "validation_failed")
	}
	if body.Field != "as_of" {
		t.Errorf("field: got %q, want %q", body.Field, "as_of")
	}
}

func TestHandleListBalances_InvalidIncludeHidden(t *testing.T) {
	ts, _ := newServerWithStore(t)

	resp, err := http.Get(ts.URL + "/api/balances?include_hidden=maybe")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
		Field string `json:"field"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error != "validation_failed" {
		t.Errorf("error code: got %q, want %q", body.Error, "validation_failed")
	}
	if body.Field != "include_hidden" {
		t.Errorf("field: got %q, want %q", body.Field, "include_hidden")
	}
}

func TestHandleListBalances_IncludeHidden(t *testing.T) {
	ts, svc := newServerWithStore(t)
	visible := seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	hidden := seedAccount(t, svc, "Assets:Old", model.AccountTypeAsset, 0)

	// Mark "Assets:Old" hidden via the service.
	if _, err := svc.Account().UpdateAccountMetadata(
		t.Context(), hidden.ID, hidden.Description, true,
	); err != nil {
		t.Fatalf("hide account: %v", err)
	}
	_ = visible // referenced via the response

	// Default: hidden excluded.
	respDefault, err := http.Get(ts.URL + "/api/balances")
	if err != nil {
		t.Fatalf("GET default: %v", err)
	}
	defer respDefault.Body.Close()
	var gotDefault model.ListResult[model.AccountBalance]
	if err := json.NewDecoder(respDefault.Body).Decode(&gotDefault); err != nil {
		t.Fatalf("decode default: %v", err)
	}
	for _, row := range gotDefault.Items {
		if row.Name == "Assets:Old" {
			t.Errorf("hidden account should be excluded by default, got row %+v", row)
		}
	}

	// Toggled: hidden included.
	respAll, err := http.Get(ts.URL + "/api/balances?include_hidden=true")
	if err != nil {
		t.Fatalf("GET include_hidden: %v", err)
	}
	defer respAll.Body.Close()
	var gotAll model.ListResult[model.AccountBalance]
	if err := json.NewDecoder(respAll.Body).Decode(&gotAll); err != nil {
		t.Fatalf("decode include_hidden: %v", err)
	}
	var sawHidden bool
	for _, row := range gotAll.Items {
		if row.Name == "Assets:Old" {
			sawHidden = true
			if !row.IsHidden {
				t.Errorf("Assets:Old should have IsHidden=true; got %+v", row)
			}
		}
	}
	if !sawHidden {
		t.Error("Assets:Old should appear when include_hidden=true")
	}
}

func TestHandleListAccounts_SearchFiltersHidden(t *testing.T) {
	ts, svc := newServerWithStore(t)
	visible := seedAccount(t, svc, "Assets:BankA", model.AccountTypeAsset, 0)
	hidden := seedAccount(t, svc, "Assets:BankB", model.AccountTypeAsset, 0)
	if _, err := svc.Account().UpdateAccountMetadata(
		t.Context(), hidden.ID, hidden.Description, true,
	); err != nil {
		t.Fatalf("hide: %v", err)
	}

	// ?q= triggers the SearchAccounts path; the store filters hidden when IncludeHidden is false.
	resp, err := http.Get(ts.URL + "/api/accounts?q=Bank&include_count=true")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var lr model.ListResult[*model.Account]
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sawVisible, sawHidden := false, false
	for _, a := range lr.Items {
		if a.ID == visible.ID {
			sawVisible = true
		}
		if a.ID == hidden.ID {
			sawHidden = true
		}
	}
	if !sawVisible {
		t.Errorf("visible account missing from search")
	}
	if sawHidden {
		t.Errorf("hidden account leaked into search (store did not filter hidden)")
	}
	// TotalCount should reflect only the visible items returned (store filters hidden).
	if lr.TotalCount != len(lr.Items) {
		t.Errorf("TotalCount %d != len(Items) %d", lr.TotalCount, len(lr.Items))
	}

	// Verify the same query with include_hidden=true returns the hidden one too.
	resp2, err := http.Get(ts.URL + "/api/accounts?q=Bank&include_hidden=true")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp2.Body.Close()
	var lr2 model.ListResult[*model.Account]
	if err := json.NewDecoder(resp2.Body).Decode(&lr2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gotHidden := false
	for _, a := range lr2.Items {
		if a.ID == hidden.ID {
			gotHidden = true
		}
	}
	if !gotHidden {
		t.Errorf("include_hidden=true: hidden account missing from search")
	}
}
