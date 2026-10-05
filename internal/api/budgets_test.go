// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
)

func putJSON(t *testing.T, url string, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, b
}

func decodeErrBody(t *testing.T, b []byte) errorBody {
	t.Helper()
	var e errorBody
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("decode error body %s: %v", b, err)
	}
	return e
}

func TestHandleSetBudget_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Expenses:Food", model.AccountTypeExpense, 0)

	status, body := putJSON(t, ts.URL+"/api/budgets",
		`{"account_name":"Expenses:Food","effective_month":"2026-01","amount":800000}`)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var got model.Budget
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.AccountName != "Expenses:Food" || got.Amount != 800000 || got.EffectiveMonth != "2026-01" {
		t.Errorf("unexpected budget: %+v", got)
	}
}

func TestHandleSetBudget_ValidationAndNotFound(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Expenses:Food", model.AccountTypeExpense, 0)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

	cases := []struct {
		name   string
		body   string
		status int
		field  string
	}{
		{"negative amount", `{"account_name":"Expenses:Food","effective_month":"2026-01","amount":-1}`, 400, "amount"},
		{"bad month", `{"account_name":"Expenses:Food","effective_month":"2026-13","amount":1}`, 400, "effective_month"},
		{"missing month", `{"account_name":"Expenses:Food","amount":1}`, 400, "effective_month"},
		{"non-expense", `{"account_name":"Assets:Bank","effective_month":"2026-01","amount":1}`, 400, "account_name"},
		{"unknown field", `{"account_name":"Expenses:Food","effective_month":"2026-01","amount":1,"x":1}`, 400, "body"},
		{"unknown account", `{"account_name":"Expenses:Nope","effective_month":"2026-01","amount":1}`, 404, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := putJSON(t, ts.URL+"/api/budgets", tc.body)
			if status != tc.status {
				t.Fatalf("status %d, want %d: %s", status, tc.status, body)
			}
			if tc.field != "" && decodeErrBody(t, body).Field != tc.field {
				t.Errorf("field = %q, want %q", decodeErrBody(t, body).Field, tc.field)
			}
		})
	}
}

func TestHandleListStopDeleteBudgets(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Expenses:Food", model.AccountTypeExpense, 0)

	status, body := getJSON(t, ts.URL+"/api/budgets")
	if status != http.StatusOK || string(bytes.TrimSpace(body)) != `{"items":[]}` {
		t.Fatalf("empty list: %d %s", status, body)
	}

	if status, body := putJSON(t, ts.URL+"/api/budgets",
		`{"account_name":"Expenses:Food","effective_month":"2026-01","amount":100}`); status != 200 {
		t.Fatalf("set: %d %s", status, body)
	}

	status, body = postJSON(t, ts.URL+"/api/budgets/stop",
		map[string]string{"account_name": "Expenses:Food", "effective_month": "2026-03"})
	if status != http.StatusOK {
		t.Fatalf("stop: %d %s", status, body)
	}
	var stopped model.Budget
	_ = json.Unmarshal(body, &stopped)
	if !stopped.Stopped {
		t.Errorf("expected stopped budget, got %+v", stopped)
	}

	status, body = postJSON(t, ts.URL+"/api/budgets/stop",
		map[string]string{"account_name": "Expenses:Food", "effective_month": "2026-05"})
	if status != http.StatusBadRequest || decodeErrBody(t, body).Field != "effective_month" {
		t.Fatalf("stop twice: %d %s", status, body)
	}

	status, body = getJSON(t, ts.URL+"/api/budgets")
	var list struct{ Items []model.Budget }
	_ = json.Unmarshal(body, &list)
	if status != 200 || len(list.Items) != 2 {
		t.Fatalf("list: %d %s", status, body)
	}

	id := list.Items[0].ID
	status, body = deleteURL(t, ts.URL+"/api/budgets/"+itoa(id))
	if status != http.StatusOK {
		t.Fatalf("delete: %d %s", status, body)
	}
	status, _ = deleteURL(t, ts.URL+"/api/budgets/"+itoa(id))
	if status != http.StatusNotFound {
		t.Errorf("delete again: %d, want 404", status)
	}
	status, _ = deleteURL(t, ts.URL+"/api/budgets/abc")
	if status != http.StatusBadRequest {
		t.Errorf("delete bad id: %d, want 400", status)
	}
}

func TestHandleBudgetReport(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Food", model.AccountTypeExpense, 0)
	if _, err := svc.Budget().SetBudget(t.Context(), model.SetBudgetInput{
		AccountName: "Expenses:Food", EffectiveMonth: "2026-05", Amount: 5000,
	}); err != nil {
		t.Fatal(err)
	}
	ts15 := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local).Unix()
	seedTransaction(t, svc, "Assets:Cash", "Expenses:Food", 1200, ts15, "lunch", model.TxTypeExpense, model.StatusCleared)

	status, body := getJSON(t, ts.URL+"/api/reports/budget?month=2026-05")
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var got model.BudgetReport
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || got.Rows[0].Actual != 1200 || got.Rows[0].Remaining != 3800 {
		t.Fatalf("unexpected report: %s", body)
	}
	if got.Rows[0].ExcludedAccounts == nil {
		t.Errorf("excluded_accounts must serialize as []")
	}

	status, body = getJSON(t, ts.URL+"/api/reports/budget")
	if status != http.StatusOK {
		t.Fatalf("default month: %d %s", status, body)
	}
	_ = json.Unmarshal(body, &got)
	if got.Month != time.Now().Format("2006-01") {
		t.Errorf("default month = %q", got.Month)
	}

	status, body = getJSON(t, ts.URL+"/api/reports/budget?month=2026-13")
	if status != http.StatusBadRequest || decodeErrBody(t, body).Field != "month" {
		t.Errorf("bad month: %d %s", status, body)
	}
}
