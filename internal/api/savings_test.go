// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
)

func TestHandleSetSavingsTarget_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Savings", model.AccountTypeAsset, 0)

	status, body := putJSON(t, ts.URL+"/api/savings-targets",
		`{"account_name":"Assets:Savings","effective_month":"2026-01","amount":1500000}`)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var got model.SavingsTarget
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.AccountName != "Assets:Savings" || got.Amount != 1500000 || got.EffectiveMonth != "2026-01" {
		t.Errorf("unexpected target: %+v", got)
	}
}

func TestHandleSetSavingsTarget_ValidationAndNotFound(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Savings", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Expenses:Food", model.AccountTypeExpense, 0)

	cases := []struct {
		name   string
		body   string
		status int
		field  string
	}{
		{"negative amount", `{"account_name":"Assets:Savings","effective_month":"2026-01","amount":-1}`, 400, "amount"},
		{"bad month", `{"account_name":"Assets:Savings","effective_month":"2026-13","amount":1}`, 400, "effective_month"},
		{"missing month", `{"account_name":"Assets:Savings","amount":1}`, 400, "effective_month"},
		{"non-asset", `{"account_name":"Expenses:Food","effective_month":"2026-01","amount":1}`, 400, "account_name"},
		{"unknown field", `{"account_name":"Assets:Savings","effective_month":"2026-01","amount":1,"x":1}`, 400, "body"},
		{"unknown account", `{"account_name":"Assets:Nope","effective_month":"2026-01","amount":1}`, 404, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := putJSON(t, ts.URL+"/api/savings-targets", tc.body)
			if status != tc.status {
				t.Fatalf("status %d, want %d: %s", status, tc.status, body)
			}
			if tc.field != "" && decodeErrBody(t, body).Field != tc.field {
				t.Errorf("field = %q, want %q", decodeErrBody(t, body).Field, tc.field)
			}
		})
	}
}

func TestHandleListStopDeleteSavingsTargets(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Savings", model.AccountTypeAsset, 0)

	status, body := getJSON(t, ts.URL+"/api/savings-targets")
	if status != http.StatusOK || string(bytes.TrimSpace(body)) != `{"items":[]}` {
		t.Fatalf("empty list: %d %s", status, body)
	}

	if status, body := putJSON(t, ts.URL+"/api/savings-targets",
		`{"account_name":"Assets:Savings","effective_month":"2026-01","amount":100}`); status != 200 {
		t.Fatalf("set: %d %s", status, body)
	}

	status, body = postJSON(t, ts.URL+"/api/savings-targets/stop",
		map[string]string{"account_name": "Assets:Savings", "effective_month": "2026-03"})
	if status != http.StatusOK {
		t.Fatalf("stop: %d %s", status, body)
	}
	var stopped model.SavingsTarget
	_ = json.Unmarshal(body, &stopped)
	if !stopped.Stopped {
		t.Errorf("expected stopped target, got %+v", stopped)
	}

	status, body = postJSON(t, ts.URL+"/api/savings-targets/stop",
		map[string]string{"account_name": "Assets:Savings", "effective_month": "2026-05"})
	if status != http.StatusBadRequest || decodeErrBody(t, body).Field != "effective_month" {
		t.Fatalf("stop twice: %d %s", status, body)
	}

	status, body = getJSON(t, ts.URL+"/api/savings-targets")
	var list struct{ Items []model.SavingsTarget }
	_ = json.Unmarshal(body, &list)
	if status != 200 || len(list.Items) != 2 {
		t.Fatalf("list: %d %s", status, body)
	}

	id := list.Items[0].ID
	status, body = deleteURL(t, ts.URL+"/api/savings-targets/"+itoa(id))
	if status != http.StatusOK {
		t.Fatalf("delete: %d %s", status, body)
	}
	status, _ = deleteURL(t, ts.URL+"/api/savings-targets/"+itoa(id))
	if status != http.StatusNotFound {
		t.Errorf("delete again: %d, want 404", status)
	}
	status, _ = deleteURL(t, ts.URL+"/api/savings-targets/abc")
	if status != http.StatusBadRequest {
		t.Errorf("delete bad id: %d, want 400", status)
	}
}

func TestHandleSavingsReport(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	// Created with an opening balance: the Opening transaction must not count as saved.
	seedAccount(t, svc, "Assets:Savings", model.AccountTypeAsset, 500000)
	if _, err := svc.Savings().SetSavingsTarget(t.Context(), model.SetSavingsTargetInput{
		AccountName: "Assets:Savings", EffectiveMonth: "2026-05", Amount: 5000,
	}); err != nil {
		t.Fatal(err)
	}
	ts15 := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local).Unix()
	seedTransaction(t, svc, "Assets:Cash", "Assets:Savings", 1200, ts15, "save", model.TxTypeTransfer, model.StatusCleared)

	status, body := getJSON(t, ts.URL+"/api/reports/savings?month=2026-05")
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var got model.SavingsReport
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || got.Rows[0].Saved != 1200 || got.Rows[0].Remaining != 3800 {
		t.Fatalf("unexpected report: %s", body)
	}
	if got.Rows[0].ExcludedAccounts == nil || got.Rows[0].Months == nil {
		t.Errorf("excluded_accounts and months must serialize as []")
	}

	status, body = getJSON(t, ts.URL+"/api/reports/savings")
	if status != http.StatusOK {
		t.Fatalf("default month: %d %s", status, body)
	}
	_ = json.Unmarshal(body, &got)
	if got.Month != time.Now().Format("2006-01") {
		t.Errorf("default month = %q", got.Month)
	}
	// The opening balance of Assets:Savings is dated now, so it falls in the
	// current month; it must not count as saved.
	if len(got.Rows) != 1 || got.Rows[0].Saved != 0 {
		t.Errorf("opening balance counted as saved: %s", body)
	}

	status, body = getJSON(t, ts.URL+"/api/reports/savings?month=2026-13")
	if status != http.StatusBadRequest || decodeErrBody(t, body).Field != "month" {
		t.Errorf("bad month: %d %s", status, body)
	}
}
