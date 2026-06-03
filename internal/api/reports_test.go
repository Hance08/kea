// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
)

func TestHandleIncomeStatement_Month(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Revenue:Salary", model.AccountTypeRevenue, 0)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	may1 := time.Date(2026, 5, 5, 12, 0, 0, 0, time.Local).Unix()
	seedTransaction(t, svc, "Revenue:Salary", "Assets:Cash", 500000, may1, "pay", model.TxTypeIncome, model.StatusCleared)
	seedTransaction(t, svc, "Assets:Cash", "Expenses:Coffee", 1000, may1, "coffee", model.TxTypeExpense, model.StatusCleared)

	resp, err := http.Get(ts.URL + "/api/reports/income-statement?month=2026-05")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var got model.ReportResult
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Period == "" {
		t.Errorf("Period must be populated")
	}
	if got.TotalIncome["USD"] != 500000 {
		t.Errorf("TotalIncome USD: got %d, want 500000", got.TotalIncome["USD"])
	}
	if got.TotalExpense["USD"] != 1000 {
		t.Errorf("TotalExpense USD: got %d, want 1000", got.TotalExpense["USD"])
	}
}

func TestHandleIncomeStatement_BadMonth(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/reports/income-statement?month=2026-13")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["field"] != "month" {
		t.Errorf("field: got %q", body["field"])
	}
}

func TestHandleIncomeStatement_BadDateRange(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/reports/income-statement?from=2026-02-01&to=2026-01-01")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleIncomeBreakdown_Month(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Revenue:Salary", model.AccountTypeRevenue, 0)

	may1 := time.Date(2026, 5, 5, 12, 0, 0, 0, time.Local).Unix()
	seedTransaction(t, svc, "Revenue:Salary", "Assets:Cash", 500000, may1, "pay", model.TxTypeIncome, model.StatusCleared)

	resp, err := http.Get(ts.URL + "/api/reports/income-breakdown?month=2026-05")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var got model.ReportResult
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if len(got.IncomeRows) == 0 {
		t.Errorf("expected income rows")
	}
	if got.TotalIncome["USD"] != 500000 {
		t.Errorf("TotalIncome USD: got %d", got.TotalIncome["USD"])
	}
}

func TestHandleExpenseBreakdown_Month(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Coffee", model.AccountTypeExpense, 0)

	may1 := time.Date(2026, 5, 5, 12, 0, 0, 0, time.Local).Unix()
	seedTransaction(t, svc, "Assets:Cash", "Expenses:Coffee", 1000, may1, "coffee", model.TxTypeExpense, model.StatusCleared)

	resp, err := http.Get(ts.URL + "/api/reports/expense-breakdown?month=2026-05")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var got model.ReportResult
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if len(got.ExpenseRows) == 0 {
		t.Errorf("expected expense rows")
	}
	if got.TotalExpense["USD"] != 1000 {
		t.Errorf("TotalExpense USD: got %d", got.TotalExpense["USD"])
	}
}

func TestHandleBalanceSheet_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Liabilities:Card", model.AccountTypeLiability, 25000)

	resp, err := http.Get(ts.URL + "/api/reports/balance-sheet")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var got model.BalanceSheetResult
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.AsOf == 0 {
		t.Errorf("AsOf should default to now")
	}
	now := time.Now().Unix()
	if got.AsOf < now-30 || got.AsOf > now+30 {
		t.Errorf("AsOf out of window: got %d, now=%d", got.AsOf, now)
	}
	if got.TotalAssets["USD"] != 100000 {
		t.Errorf("TotalAssets USD: got %d, want 100000", got.TotalAssets["USD"])
	}
}

func TestHandleBalanceSheet_BadAsOf(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/reports/balance-sheet?as_of=abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}
