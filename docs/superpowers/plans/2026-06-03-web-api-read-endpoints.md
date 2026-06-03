# Web API Read Endpoints Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 12 read-only HTTP endpoints (5 accounts, 2 transactions, 5 reports) on top of the existing `internal/api/` foundation. Each endpoint maps onto an existing service method; the work is purely additive in `internal/api/`.

**Architecture:** Per-resource handler files (`accounts.go`, `transactions.go`, `reports.go`) hung off `*Server`. A shared `params.go` parses query strings into `model.ListOptions`, `model.AccountFilter`, `model.TransactionFilter`, and `service.DateRangeParams`, returning `*service.ValidationError` on bad input so the existing `mapError` routes them to 400. Handler tests use a real `*service.Service` backed by an in-memory SQLite DB so they exercise the full stack.

**Tech Stack:** Go 1.25, `github.com/go-chi/chi/v5` (already in `go.mod`), stdlib `net/http` + `log/slog` + `net/http/httptest`. No new dependencies.

**Spec:** [`docs/superpowers/specs/2026-06-03-web-api-read-endpoints-design.md`](../specs/2026-06-03-web-api-read-endpoints-design.md)

---

## File Layout

| File | Responsibility |
|------|----------------|
| `internal/api/params.go` | Query/path parsers: `parseInt64Query`, `parseIntQuery`, `parseBoolQuery`, `parseStringQuery`, `parseInt64Path`, `parseListOptions`, `parseAccountFilter`, `parseTransactionFilter`, `parseDateRangeParams` |
| `internal/api/params_test.go` | Unit tests for every parser (no HTTP setup needed) |
| `internal/api/accounts.go` | 5 account handlers + 2 file-local helpers (`wrapAsListResult`, `applyHiddenFilter`) |
| `internal/api/accounts_test.go` | End-to-end HTTP tests for account routes |
| `internal/api/transactions.go` | 2 transaction handlers |
| `internal/api/transactions_test.go` | End-to-end HTTP tests for transaction routes |
| `internal/api/reports.go` | 5 report handlers |
| `internal/api/reports_test.go` | End-to-end HTTP tests for report routes |
| `internal/api/testhelper_test.go` | `newServerWithStore(t)` + seed helpers (in-memory SQLite store + real `*service.Service`) |
| `internal/api/router.go` *(modify)* | Register the 12 new routes |

The existing `router.go`-defined `newTestServer(t)` helper (nil-svc) stays untouched — the new helper uses a distinct name to avoid collision.

Build order: parsers → test substrate → accounts → transactions → reports. Each handler is wired into the router *in the same task that introduces it*, so every task ends in a green build with a working endpoint.

---

## Task 1: Core query/path parsers (TDD)

**Files:**
- Create: `internal/api/params.go`
- Create: `internal/api/params_test.go`

- [ ] **Step 1: Write failing tests for core parsers**

Create `internal/api/params_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/hance08/kea/internal/service"
)

func reqWithQuery(t *testing.T, raw string) *http.Request {
	t.Helper()
	r, err := http.NewRequest("GET", "/?"+raw, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return r
}

func TestParseInt64Query(t *testing.T) {
	t.Run("absent returns nil nil", func(t *testing.T) {
		got, err := parseInt64Query(reqWithQuery(t, ""), "id")
		if err != nil || got != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", got, err)
		}
	})
	t.Run("present parses", func(t *testing.T) {
		got, err := parseInt64Query(reqWithQuery(t, "id=42"), "id")
		if err != nil || got == nil || *got != 42 {
			t.Fatalf("got (%v, %v), want (*42, nil)", got, err)
		}
	})
	t.Run("invalid returns *ValidationError with field", func(t *testing.T) {
		_, err := parseInt64Query(reqWithQuery(t, "id=xyz"), "id")
		var verr *service.ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("expected *ValidationError, got %T: %v", err, err)
		}
		if verr.Field != "id" {
			t.Errorf("Field = %q, want %q", verr.Field, "id")
		}
	})
	t.Run("empty string treated as absent", func(t *testing.T) {
		got, err := parseInt64Query(reqWithQuery(t, "id="), "id")
		if err != nil || got != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", got, err)
		}
	})
}

func TestParseIntQuery(t *testing.T) {
	got, err := parseIntQuery(reqWithQuery(t, "n=7"), "n")
	if err != nil || got == nil || *got != 7 {
		t.Fatalf("got (%v, %v), want (*7, nil)", got, err)
	}
	_, err = parseIntQuery(reqWithQuery(t, "n=bad"), "n")
	if err == nil {
		t.Fatalf("expected error for bad int")
	}
}

func TestParseBoolQuery(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
		err  bool
	}{
		{"", false, false},
		{"flag=true", true, false},
		{"flag=false", false, false},
		{"flag=1", true, false},
		{"flag=0", false, false},
		{"flag=yes", false, true},
	}
	for _, c := range cases {
		got, err := parseBoolQuery(reqWithQuery(t, c.raw), "flag")
		if c.err {
			if err == nil {
				t.Errorf("%q: expected error", c.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", c.raw, err)
		}
		if got != c.want {
			t.Errorf("%q: got %v, want %v", c.raw, got, c.want)
		}
	}
}

func TestParseStringQuery(t *testing.T) {
	if p := parseStringQuery(reqWithQuery(t, ""), "q"); p != nil {
		t.Errorf("absent: got %v, want nil", *p)
	}
	if p := parseStringQuery(reqWithQuery(t, "q="), "q"); p != nil {
		t.Errorf("empty: got %v, want nil", *p)
	}
	p := parseStringQuery(reqWithQuery(t, "q=bank"), "q")
	if p == nil || *p != "bank" {
		t.Errorf("got %v, want bank", p)
	}
}

func TestParseInt64Path(t *testing.T) {
	r := httptest.NewRequest("GET", "/x", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "42")
	r = r.WithContext(chi.NewContext(r.Context(), rctx))

	got, err := parseInt64Path(r, "id")
	if err != nil || got != 42 {
		t.Fatalf("got (%d, %v), want (42, nil)", got, err)
	}

	r2 := httptest.NewRequest("GET", "/x", nil)
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("id", "xyz")
	r2 = r2.WithContext(chi.NewContext(r2.Context(), rctx2))
	_, err = parseInt64Path(r2, "id")
	var verr *service.ValidationError
	if !errors.As(err, &verr) || verr.Field != "id" {
		t.Fatalf("expected *ValidationError{Field:\"id\"}, got %v", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/api/ -run "TestParseInt64Query|TestParseIntQuery|TestParseBoolQuery|TestParseStringQuery|TestParseInt64Path" -v`

Expected: build error — `parseInt64Query`, `parseIntQuery`, `parseBoolQuery`, `parseStringQuery`, `parseInt64Path` undefined.

- [ ] **Step 3: Implement the core parsers**

Create `internal/api/params.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hance08/kea/internal/service"
)

func parseInt64Query(r *http.Request, key string) (*int64, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, &service.ValidationError{Field: key, Message: key + " must be an integer"}
	}
	return &n, nil
}

func parseIntQuery(r *http.Request, key string) (*int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil, &service.ValidationError{Field: key, Message: key + " must be an integer"}
	}
	return &n, nil
}

func parseBoolQuery(r *http.Request, key string) (bool, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return false, nil
	}
	switch strings.ToLower(raw) {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, &service.ValidationError{Field: key, Message: key + " must be true/false/1/0"}
	}
}

func parseStringQuery(r *http.Request, key string) *string {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil
	}
	return &raw
}

func parseInt64Path(r *http.Request, key string) (int64, error) {
	raw := chi.URLParam(r, key)
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, &service.ValidationError{Field: key, Message: key + " must be an integer"}
	}
	return n, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestParseInt64Query|TestParseIntQuery|TestParseBoolQuery|TestParseStringQuery|TestParseInt64Path" -v`

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/params.go internal/api/params_test.go
git commit -m "feat(api): add query/path parameter parsers for read endpoints"
```

---

## Task 2: Composite parsers — ListOptions, AccountFilter, TransactionFilter, DateRangeParams (TDD)

**Files:**
- Modify: `internal/api/params.go` (append composite parsers)
- Modify: `internal/api/params_test.go` (append composite tests)

- [ ] **Step 1: Write failing tests for composite parsers**

Append to `internal/api/params_test.go`:

```go
func TestParseListOptions(t *testing.T) {
	t.Run("absent yields zero opts", func(t *testing.T) {
		opts, err := parseListOptions(reqWithQuery(t, ""))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if opts.Limit != 0 || opts.Offset != 0 || opts.IncludeCount != false {
			t.Errorf("got %+v, want zero", opts)
		}
	})
	t.Run("present", func(t *testing.T) {
		opts, err := parseListOptions(reqWithQuery(t, "limit=20&offset=40&include_count=true"))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if opts.Limit != 20 || opts.Offset != 40 || !opts.IncludeCount {
			t.Errorf("got %+v", opts)
		}
	})
	t.Run("negative limit rejected", func(t *testing.T) {
		_, err := parseListOptions(reqWithQuery(t, "limit=-1"))
		var verr *service.ValidationError
		if !errors.As(err, &verr) || verr.Field != "limit" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("negative offset rejected", func(t *testing.T) {
		_, err := parseListOptions(reqWithQuery(t, "offset=-1"))
		var verr *service.ValidationError
		if !errors.As(err, &verr) || verr.Field != "offset" {
			t.Fatalf("got %v", err)
		}
	})
}

func TestParseAccountFilter(t *testing.T) {
	t.Run("absent yields zero filter", func(t *testing.T) {
		f, err := parseAccountFilter(reqWithQuery(t, ""))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if f.Query != nil || f.Type != nil || f.Currency != nil {
			t.Errorf("got %+v, want zero", f)
		}
	})
	t.Run("all fields", func(t *testing.T) {
		f, err := parseAccountFilter(reqWithQuery(t, "q=bank&type=A&currency=USD"))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if f.Query == nil || *f.Query != "bank" {
			t.Errorf("Query: %v", f.Query)
		}
		if f.Type == nil || string(*f.Type) != "A" {
			t.Errorf("Type: %v", f.Type)
		}
		if f.Currency == nil || *f.Currency != "USD" {
			t.Errorf("Currency: %v", f.Currency)
		}
	})
	t.Run("invalid type rejected", func(t *testing.T) {
		_, err := parseAccountFilter(reqWithQuery(t, "type=Z"))
		var verr *service.ValidationError
		if !errors.As(err, &verr) || verr.Field != "type" {
			t.Fatalf("got %v", err)
		}
	})
}

func TestParseTransactionFilter(t *testing.T) {
	t.Run("absent yields zero filter", func(t *testing.T) {
		f, err := parseTransactionFilter(reqWithQuery(t, ""))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if f.AccountID != nil || f.Type != nil || f.Status != nil ||
			f.StartTime != nil || f.EndTime != nil || f.Description != nil {
			t.Errorf("got %+v, want zero", f)
		}
	})
	t.Run("all fields", func(t *testing.T) {
		f, err := parseTransactionFilter(reqWithQuery(t,
			"account_id=12&type=Expense&status=Cleared&start_time=100&end_time=200&description=coffee"))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if f.AccountID == nil || *f.AccountID != 12 {
			t.Errorf("AccountID: %v", f.AccountID)
		}
		if f.Type == nil || string(*f.Type) != "Expense" {
			t.Errorf("Type: %v", f.Type)
		}
		if f.Status == nil || int(*f.Status) != 1 {
			t.Errorf("Status: %v", f.Status)
		}
		if f.StartTime == nil || *f.StartTime != 100 {
			t.Errorf("StartTime: %v", f.StartTime)
		}
		if f.EndTime == nil || *f.EndTime != 200 {
			t.Errorf("EndTime: %v", f.EndTime)
		}
		if f.Description == nil || *f.Description != "coffee" {
			t.Errorf("Description: %v", f.Description)
		}
	})
	t.Run("invalid type rejected", func(t *testing.T) {
		_, err := parseTransactionFilter(reqWithQuery(t, "type=Foo"))
		var verr *service.ValidationError
		if !errors.As(err, &verr) || verr.Field != "type" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("invalid status rejected", func(t *testing.T) {
		_, err := parseTransactionFilter(reqWithQuery(t, "status=Wat"))
		var verr *service.ValidationError
		if !errors.As(err, &verr) || verr.Field != "status" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("start_time greater than end_time rejected", func(t *testing.T) {
		_, err := parseTransactionFilter(reqWithQuery(t, "start_time=200&end_time=100"))
		var verr *service.ValidationError
		if !errors.As(err, &verr) || verr.Field != "end_time" {
			t.Fatalf("got %v", err)
		}
	})
}

func TestParseDateRangeParams(t *testing.T) {
	p := parseDateRangeParams(reqWithQuery(t, "month=2026-05"))
	if p.Month != "2026-05" || p.From != "" || p.To != "" {
		t.Errorf("got %+v", p)
	}
	p = parseDateRangeParams(reqWithQuery(t, "from=2026-05-01&to=2026-05-31"))
	if p.From != "2026-05-01" || p.To != "2026-05-31" || p.Month != "" {
		t.Errorf("got %+v", p)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/api/ -run "TestParseListOptions|TestParseAccountFilter|TestParseTransactionFilter|TestParseDateRangeParams" -v`

Expected: build error — composite functions undefined.

- [ ] **Step 3: Implement the composite parsers**

Append to `internal/api/params.go`:

```go
import (
	// existing imports above, plus:
	"github.com/hance08/kea/internal/model"
)

// parseListOptions reads ?limit=&offset=&include_count=.
// limit and offset must be >= 0 if present.
func parseListOptions(r *http.Request) (model.ListOptions, error) {
	var opts model.ListOptions
	limit, err := parseIntQuery(r, "limit")
	if err != nil {
		return opts, err
	}
	if limit != nil {
		if *limit < 0 {
			return opts, &service.ValidationError{Field: "limit", Message: "limit must be >= 0"}
		}
		opts.Limit = *limit
	}
	offset, err := parseIntQuery(r, "offset")
	if err != nil {
		return opts, err
	}
	if offset != nil {
		if *offset < 0 {
			return opts, &service.ValidationError{Field: "offset", Message: "offset must be >= 0"}
		}
		opts.Offset = *offset
	}
	inc, err := parseBoolQuery(r, "include_count")
	if err != nil {
		return opts, err
	}
	opts.IncludeCount = inc
	return opts, nil
}

func parseAccountFilter(r *http.Request) (model.AccountFilter, error) {
	var f model.AccountFilter
	f.Query = parseStringQuery(r, "q")
	f.Currency = parseStringQuery(r, "currency")
	if rawType := r.URL.Query().Get("type"); rawType != "" {
		at := model.AccountType(rawType)
		if !at.IsValid() {
			return f, &service.ValidationError{
				Field:   "type",
				Message: "type must be one of A, L, C, R, E",
			}
		}
		f.Type = &at
	}
	return f, nil
}

func parseTransactionFilter(r *http.Request) (model.TransactionFilter, error) {
	var f model.TransactionFilter

	accID, err := parseInt64Query(r, "account_id")
	if err != nil {
		return f, err
	}
	f.AccountID = accID

	if rawType := r.URL.Query().Get("type"); rawType != "" {
		tt := model.TransactionType(rawType)
		if !tt.IsValid() {
			return f, &service.ValidationError{
				Field:   "type",
				Message: "type must be one of Expense, Income, Transfer, Opening, Deposit, Withdrawal, Other",
			}
		}
		f.Type = &tt
	}

	if rawStatus := r.URL.Query().Get("status"); rawStatus != "" {
		st, perr := model.ParseTransactionStatus(rawStatus)
		if perr != nil {
			return f, &service.ValidationError{
				Field:   "status",
				Message: "status must be one of Pending, Cleared, Reconciled",
			}
		}
		f.Status = &st
	}

	start, err := parseInt64Query(r, "start_time")
	if err != nil {
		return f, err
	}
	f.StartTime = start

	end, err := parseInt64Query(r, "end_time")
	if err != nil {
		return f, err
	}
	f.EndTime = end

	if f.StartTime != nil && f.EndTime != nil && *f.StartTime > *f.EndTime {
		return f, &service.ValidationError{
			Field:   "end_time",
			Message: "end_time must be greater than or equal to start_time",
		}
	}

	f.Description = parseStringQuery(r, "description")
	return f, nil
}

func parseDateRangeParams(r *http.Request) service.DateRangeParams {
	q := r.URL.Query()
	return service.DateRangeParams{
		Month: q.Get("month"),
		From:  q.Get("from"),
		To:    q.Get("to"),
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/api/ -run "TestParseListOptions|TestParseAccountFilter|TestParseTransactionFilter|TestParseDateRangeParams" -v`

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/params.go internal/api/params_test.go
git commit -m "feat(api): add composite filter and pagination parsers"
```

---

## Task 3: Test substrate — in-memory store with seed helpers

**Files:**
- Create: `internal/api/testhelper_test.go`

- [ ] **Step 1: Write the substrate (no separate test target — it's a test helper used by Tasks 4+)**

Create `internal/api/testhelper_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
)

// newServerWithStore builds a *Server backed by an in-memory SQLite store and
// returns an httptest.Server fronting its routes plus the *service.Service for
// seeding.
func newServerWithStore(t *testing.T) (*httptest.Server, *service.Service) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(dbPath, migrations.FS)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := config.NewDefault()
	cfg.Defaults.Currency = "USD"

	svc := service.NewService(st, st, st, cfg)

	srv := NewServer(cfg, svc, discardLogger())
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)

	return ts, svc
}

// seedAccount creates a leaf account and returns it.
func seedAccount(t *testing.T, svc *service.Service, name string, accType model.AccountType, balance int64) *model.Account {
	t.Helper()
	acc, err := svc.Account().CreateAccountWithBalance(context.Background(), model.CreateAccountInput{
		Name:     name,
		Type:     accType,
		Currency: "USD",
		Balance:  balance,
	})
	if err != nil {
		t.Fatalf("seedAccount %q: %v", name, err)
	}
	return acc
}

// seedTransaction creates a simple two-split transaction and returns its detail.
func seedTransaction(t *testing.T, svc *service.Service, from, to string, amount int64, timestamp int64, description string, txType model.TransactionType, status model.TransactionStatus) model.TransactionDetail {
	t.Helper()
	d, err := svc.Transaction().CreateSimpleTransaction(context.Background(), model.CreateSimpleTransactionInput{
		FromAccount: from,
		ToAccount:   to,
		Amount:      amount,
		Timestamp:   timestamp,
		Description: description,
		Type:        txType,
		Status:      status,
	})
	if err != nil {
		t.Fatalf("seedTransaction %q->%q: %v", from, to, err)
	}
	return d
}
```

- [ ] **Step 2: Verify the file compiles by running the existing tests**

Run: `go test ./internal/api/ -count=1`

Expected: all existing tests still pass; the new helper file builds. The helpers themselves are uncalled (Go does not warn about unused exported-package helpers in test files, but `go vet` may complain about unused imports — verify clean).

- [ ] **Step 3: Commit**

```bash
git add internal/api/testhelper_test.go
git commit -m "test(api): add in-memory store test substrate for handler tests"
```

---

## Task 4: handleAccountByID — register route + handler + tests

**Files:**
- Create: `internal/api/accounts.go`
- Create: `internal/api/accounts_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/api/accounts_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail at compile time**

Run: `go test ./internal/api/ -run "TestHandleAccountByID" -v`

Expected: build error — `handleAccountByID` undefined, route not registered.

- [ ] **Step 3: Create accounts.go with the handler**

Create `internal/api/accounts.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

// wrapAsListResult wraps a flat slice into a ListResult with limit=0/offset=0
// and total_count=len. Used by /api/accounts when the request has no filter or
// pagination intent so the response shape is consistent.
func wrapAsListResult(accounts []*model.Account) *model.ListResult[*model.Account] {
	return &model.ListResult[*model.Account]{
		Items:      accounts,
		TotalCount: len(accounts),
		Limit:      0,
		Offset:     0,
	}
}

func (s *Server) handleAccountByID(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	acc, err := s.svc.Account().GetAccountByID(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, acc)
}
```

- [ ] **Step 4: Register the route**

Modify `internal/api/router.go`. Inside the `r.Route("/api", ...)` block, add (after the existing health/version lines):

```go
		r.Method(http.MethodGet, "/accounts/{id}", apiHandler(s.handleAccountByID))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandleAccountByID" -v`

Expected: all three subtests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/accounts.go internal/api/accounts_test.go internal/api/router.go internal/api/testhelper_test.go
git commit -m "feat(api): GET /api/accounts/{id}"
```

---

## Task 5: handleAccountByName

**Files:**
- Modify: `internal/api/accounts.go`
- Modify: `internal/api/accounts_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Append failing tests**

Append to `internal/api/accounts_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/api/ -run "TestHandleAccountByName" -v`

Expected: build error / undefined route.

- [ ] **Step 3: Add the handler**

Append to `internal/api/accounts.go`:

```go
import (
	// add to existing imports:
	"github.com/hance08/kea/internal/service"
)

func (s *Server) handleAccountByName(w http.ResponseWriter, r *http.Request) error {
	name := r.URL.Query().Get("name")
	if name == "" {
		return &service.ValidationError{Field: "name", Message: "name query parameter is required"}
	}
	acc, err := s.svc.Account().GetAccountByName(r.Context(), name)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, acc)
}
```

- [ ] **Step 4: Register the route**

Modify `internal/api/router.go`. Add **before** `"/accounts/{id}"` so chi matches the static segment first (chi actually matches static before dynamic regardless of order, but listing static routes first makes the intent obvious):

```go
		r.Method(http.MethodGet, "/accounts/by-name", apiHandler(s.handleAccountByName))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandleAccountByName" -v`

Expected: all three subtests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/accounts.go internal/api/accounts_test.go internal/api/router.go
git commit -m "feat(api): GET /api/accounts/by-name"
```

---

## Task 6: handleAccountBalance

**Files:**
- Modify: `internal/api/accounts.go`
- Modify: `internal/api/accounts_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Append failing tests**

Append to `internal/api/accounts_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/api/ -run "TestHandleAccountBalance" -v`

Expected: build error — handler undefined.

- [ ] **Step 3: Add the handler**

Append to `internal/api/accounts.go`:

```go
type balanceResponse struct {
	AccountID int64  `json:"account_id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}

func (s *Server) handleAccountBalance(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	acc, err := s.svc.Account().GetAccountByID(r.Context(), id)
	if err != nil {
		return err
	}
	amount, err := s.svc.Account().GetAccountBalance(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, balanceResponse{
		AccountID: id,
		Amount:    amount,
		Currency:  acc.Currency,
	})
}
```

- [ ] **Step 4: Register the route**

Modify `internal/api/router.go`. Add after `"/accounts/{id}"`:

```go
		r.Method(http.MethodGet, "/accounts/{id}/balance", apiHandler(s.handleAccountBalance))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandleAccountBalance" -v`

Expected: both subtests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/accounts.go internal/api/accounts_test.go internal/api/router.go
git commit -m "feat(api): GET /api/accounts/{id}/balance"
```

---

## Task 7: handleAccountTree

**Files:**
- Modify: `internal/api/accounts.go`
- Modify: `internal/api/accounts_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Append failing tests**

Append to `internal/api/accounts_test.go`:

```go
func TestHandleAccountTree_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Assets:Bank:Checking", model.AccountTypeAsset, 0)

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
	// Root accounts (Assets, Equity, etc.) are auto-created by seeding the leaf;
	// verify the response is a non-empty array and Assets has at least one child.
	if len(roots) == 0 {
		t.Fatalf("expected at least one root node")
	}
	var assets *model.AccountNode
	for _, n := range roots {
		if n.Account != nil && n.Account.Name == "Assets" {
			assets = n
			break
		}
	}
	if assets == nil {
		t.Fatalf("Assets root missing; roots: %+v", roots)
	}
	if len(assets.Children) == 0 {
		t.Errorf("Assets has no children")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/api/ -run "TestHandleAccountTree" -v`

Expected: build error.

- [ ] **Step 3: Add the handler**

Append to `internal/api/accounts.go`:

```go
import (
	// add to existing imports:
	"github.com/hance08/kea/internal/service"
)

func (s *Server) handleAccountTree(w http.ResponseWriter, r *http.Request) error {
	includeHidden, err := parseBoolQuery(r, "include_hidden")
	if err != nil {
		return err
	}
	roots, err := s.svc.Account().GetAccountTree(r.Context(), service.AccountTreeOptions{
		ShowHidden: includeHidden,
	})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, roots)
}
```

(The `service` import may already exist from Task 5; if so, no change.)

- [ ] **Step 4: Register the route**

Modify `internal/api/router.go`. Add **before** `"/accounts/{id}"`:

```go
		r.Method(http.MethodGet, "/accounts/tree", apiHandler(s.handleAccountTree))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandleAccountTree" -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/accounts.go internal/api/accounts_test.go internal/api/router.go
git commit -m "feat(api): GET /api/accounts/tree"
```

---

## Task 8: handleListAccounts (the big one)

**Files:**
- Modify: `internal/api/accounts.go`
- Modify: `internal/api/accounts_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Append failing tests**

Append to `internal/api/accounts_test.go`:

```go
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
	// Note: seeding creates root accounts; an "empty store" here means
	// no user-created accounts but root nodes may exist from migrations.
	// What we want to assert is the response shape, not exact counts.
	if lr.Items == nil {
		t.Errorf("Items should be non-nil slice")
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
	// And include_hidden=true exposes it:
	resp2, _ := http.Get(ts.URL + "/api/accounts?type=A&include_hidden=true")
	defer resp2.Body.Close()
	var lr2 model.ListResult[*model.Account]
	_ = json.NewDecoder(resp2.Body).Decode(&lr2)
	sawHidden := false
	sawVisible := false
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
		t.Errorf("expected TotalCount >= 2 for matches, got %d", lr.TotalCount)
	}
	for _, a := range lr.Items {
		// every returned item should match the query
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
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/api/ -run "TestHandleListAccounts" -v`

Expected: build error.

- [ ] **Step 3: Add the handler + `applyHiddenFilter` helper**

Append to `internal/api/accounts.go`:

```go
// applyHiddenFilter removes hidden accounts from the result and updates
// TotalCount to reflect what was returned. Used after SearchAccounts, which
// does not itself filter hidden.
func applyHiddenFilter(res *model.ListResult[*model.Account], includeHidden bool) *model.ListResult[*model.Account] {
	if includeHidden {
		return res
	}
	out := make([]*model.Account, 0, len(res.Items))
	for _, a := range res.Items {
		if !a.IsHidden {
			out = append(out, a)
		}
	}
	res.Items = out
	res.TotalCount = len(out)
	return res
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	opts, err := parseListOptions(r)
	if err != nil {
		return err
	}
	filter, err := parseAccountFilter(r)
	if err != nil {
		return err
	}
	includeHidden, err := parseBoolQuery(r, "include_hidden")
	if err != nil {
		return err
	}

	hasSearchIntent := filter.Query != nil || filter.Currency != nil ||
		opts.Limit > 0 || opts.Offset > 0 || opts.IncludeCount

	if hasSearchIntent {
		res, err := s.svc.Account().SearchAccounts(ctx, filter, opts)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, applyHiddenFilter(res, includeHidden))
	}

	accounts, err := s.svc.Account().ListAccounts(ctx, service.ListAccountsOptions{
		Type:       filter.Type,
		ShowHidden: includeHidden,
	})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, wrapAsListResult(accounts))
}
```

- [ ] **Step 4: Register the route**

Modify `internal/api/router.go`. Add a route line for the bare `/accounts` path. The full account routes block should now look like:

```go
		r.Method(http.MethodGet, "/accounts",                apiHandler(s.handleListAccounts))
		r.Method(http.MethodGet, "/accounts/tree",           apiHandler(s.handleAccountTree))
		r.Method(http.MethodGet, "/accounts/by-name",        apiHandler(s.handleAccountByName))
		r.Method(http.MethodGet, "/accounts/{id}",           apiHandler(s.handleAccountByID))
		r.Method(http.MethodGet, "/accounts/{id}/balance",   apiHandler(s.handleAccountBalance))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandleListAccounts" -v`

Expected: all five subtests PASS.

- [ ] **Step 6: Run the full api package tests**

Run: `go test ./internal/api/ -v`

Expected: all account, foundation, and parser tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/accounts.go internal/api/accounts_test.go internal/api/router.go
git commit -m "feat(api): GET /api/accounts list/search/filter"
```

---

## Task 9: handleTransactionByID

**Files:**
- Create: `internal/api/transactions.go`
- Create: `internal/api/transactions_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write failing tests**

Create `internal/api/transactions_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/api/ -run "TestHandleTransactionByID" -v`

Expected: build error.

- [ ] **Step 3: Create transactions.go**

Create `internal/api/transactions.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import "net/http"

func (s *Server) handleTransactionByID(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	d, err := s.svc.Transaction().GetTransactionByID(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, d)
}
```

- [ ] **Step 4: Register the route**

Modify `internal/api/router.go`. Add after the account routes:

```go
		r.Method(http.MethodGet, "/transactions/{id}", apiHandler(s.handleTransactionByID))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandleTransactionByID" -v`

Expected: all three PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/transactions.go internal/api/transactions_test.go internal/api/router.go
git commit -m "feat(api): GET /api/transactions/{id}"
```

---

## Task 10: handleListTransactions

**Files:**
- Modify: `internal/api/transactions.go`
- Modify: `internal/api/transactions_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Append failing tests**

Append to `internal/api/transactions_test.go`:

```go
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

	// Locate our two transactions in the response. Opening-balance transactions
	// from seedAccount() also appear; we only check that our two are present
	// and that newer comes before older.
	var newerIdx, olderIdx = -1, -1
	for i, tx := range lr.Items {
		switch tx.ID {
		case newer.ID:
			newerIdx = i
		case older.ID:
			olderIdx = i
		}
		if len(tx.Splits) == 0 && (tx.ID == newer.ID || tx.ID == older.ID) {
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
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/api/ -run "TestHandleListTransactions" -v`

Expected: build error.

- [ ] **Step 3: Add the handler**

Append to `internal/api/transactions.go`:

```go
import (
	// add to existing imports:
	"github.com/hance08/kea/internal/model"
)

func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	opts, err := parseListOptions(r)
	if err != nil {
		return err
	}
	filter, err := parseTransactionFilter(r)
	if err != nil {
		return err
	}

	res, err := s.svc.Transaction().FilterTransactions(ctx, filter, opts)
	if err != nil {
		return err
	}

	details, err := s.svc.Transaction().GetTransactionDetailsByIDs(ctx, res.Items)
	if err != nil {
		return err
	}

	out := make([]*model.TransactionDetail, 0, len(res.Items))
	for _, tx := range res.Items {
		if d, ok := details[tx.ID]; ok {
			out = append(out, d)
		}
	}

	return writeJSON(w, http.StatusOK, &model.ListResult[*model.TransactionDetail]{
		Items:      out,
		TotalCount: res.TotalCount,
		Limit:      res.Limit,
		Offset:     res.Offset,
	})
}
```

- [ ] **Step 4: Register the route**

Modify `internal/api/router.go`. Add **before** `"/transactions/{id}"`:

```go
		r.Method(http.MethodGet, "/transactions", apiHandler(s.handleListTransactions))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandleListTransactions" -v`

Expected: all five subtests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/transactions.go internal/api/transactions_test.go internal/api/router.go
git commit -m "feat(api): GET /api/transactions list with filtering"
```

---

## Task 11: Report handlers — income-statement, income-breakdown, expense-breakdown

**Files:**
- Create: `internal/api/reports.go`
- Create: `internal/api/reports_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write failing tests**

Create `internal/api/reports_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/api/ -run "TestHandleIncomeStatement|TestHandleIncomeBreakdown|TestHandleExpenseBreakdown" -v`

Expected: build error — handlers undefined.

- [ ] **Step 3: Create reports.go with three handlers**

Create `internal/api/reports.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import "net/http"

func (s *Server) handleIncomeStatement(w http.ResponseWriter, r *http.Request) error {
	params := parseDateRangeParams(r)
	result, err := s.svc.Transaction().GenerateFullIncomeStatement(r.Context(), params)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleIncomeBreakdown(w http.ResponseWriter, r *http.Request) error {
	params := parseDateRangeParams(r)
	result, err := s.svc.Transaction().GenerateFullIncomeBreakdown(r.Context(), params)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleExpenseBreakdown(w http.ResponseWriter, r *http.Request) error {
	params := parseDateRangeParams(r)
	result, err := s.svc.Transaction().GenerateFullExpenseBreakdown(r.Context(), params)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 4: Register the three routes**

Modify `internal/api/router.go`. Add after the transactions block:

```go
		r.Method(http.MethodGet, "/reports/income-statement",  apiHandler(s.handleIncomeStatement))
		r.Method(http.MethodGet, "/reports/income-breakdown",  apiHandler(s.handleIncomeBreakdown))
		r.Method(http.MethodGet, "/reports/expense-breakdown", apiHandler(s.handleExpenseBreakdown))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandleIncomeStatement|TestHandleIncomeBreakdown|TestHandleExpenseBreakdown" -v`

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/reports.go internal/api/reports_test.go internal/api/router.go
git commit -m "feat(api): GET /api/reports income-statement, income-breakdown, expense-breakdown"
```

---

## Task 12: handleBalanceSheet

**Files:**
- Modify: `internal/api/reports.go`
- Modify: `internal/api/reports_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Append failing tests**

Append to `internal/api/reports_test.go`:

```go
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
	// AsOf should be within 30s of "now" since the handler defaults to time.Now().Unix().
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
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/api/ -run "TestHandleBalanceSheet" -v`

Expected: build error.

- [ ] **Step 3: Add the handler**

Append to `internal/api/reports.go`:

```go
import (
	// add to existing imports:
	"time"
)

func (s *Server) handleBalanceSheet(w http.ResponseWriter, r *http.Request) error {
	asOfPtr, err := parseInt64Query(r, "as_of")
	if err != nil {
		return err
	}
	asOf := time.Now().Unix()
	if asOfPtr != nil {
		asOf = *asOfPtr
	}
	result, err := s.svc.Transaction().GenerateBalanceSheet(r.Context(), asOf)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 4: Register the route**

Modify `internal/api/router.go`. Append to the reports block:

```go
		r.Method(http.MethodGet, "/reports/balance-sheet", apiHandler(s.handleBalanceSheet))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandleBalanceSheet" -v`

Expected: both subtests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/reports.go internal/api/reports_test.go internal/api/router.go
git commit -m "feat(api): GET /api/reports/balance-sheet"
```

---

## Task 13: handleNetWorth

**Files:**
- Modify: `internal/api/reports.go`
- Modify: `internal/api/reports_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Append failing tests**

Append to `internal/api/reports_test.go`:

```go
func TestHandleNetWorth_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Liabilities:Card", model.AccountTypeLiability, 25000)

	resp, err := http.Get(ts.URL + "/api/reports/net-worth")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var got struct {
		At       int64            `json:"at"`
		NetWorth map[string]int64 `json:"net_worth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	now := time.Now().Unix()
	if got.At < now-30 || got.At > now+30 {
		t.Errorf("At out of window: %d (now=%d)", got.At, now)
	}
	if got.NetWorth == nil {
		t.Errorf("NetWorth must be a JSON object, got nil")
	}
}

func TestHandleNetWorth_BadAt(t *testing.T) {
	ts, _ := newServerWithStore(t)
	resp, err := http.Get(ts.URL + "/api/reports/net-worth?at=xyz")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/api/ -run "TestHandleNetWorth" -v`

Expected: build error.

- [ ] **Step 3: Add the handler**

Append to `internal/api/reports.go`:

```go
type netWorthResponse struct {
	At       int64            `json:"at"`
	NetWorth map[string]int64 `json:"net_worth"`
}

func (s *Server) handleNetWorth(w http.ResponseWriter, r *http.Request) error {
	atPtr, err := parseInt64Query(r, "at")
	if err != nil {
		return err
	}
	at := time.Now().Unix()
	if atPtr != nil {
		at = *atPtr
	}
	nw, err := s.svc.Transaction().GetNetWorthAt(r.Context(), at)
	if err != nil {
		return err
	}
	if nw == nil {
		nw = map[string]int64{}
	}
	return writeJSON(w, http.StatusOK, netWorthResponse{At: at, NetWorth: nw})
}
```

- [ ] **Step 4: Register the route**

Modify `internal/api/router.go`. Append to the reports block:

```go
		r.Method(http.MethodGet, "/reports/net-worth", apiHandler(s.handleNetWorth))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -run "TestHandleNetWorth" -v`

Expected: both subtests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/reports.go internal/api/reports_test.go internal/api/router.go
git commit -m "feat(api): GET /api/reports/net-worth"
```

---

## Task 14: Final verification

**Files:** none modified — verification only.

- [ ] **Step 1: Run the full api package test suite**

Run: `go test ./internal/api/ -v -count=1`

Expected: every foundation + parser + handler test PASSes. Roughly 30+ tests across the four new files plus the foundation suite.

- [ ] **Step 2: Run the full repo test suite to catch any cross-package regressions**

Run: `go test ./... -count=1`

Expected: every package green.

- [ ] **Step 3: Build the binary**

Run: `go build ./cmd/kea`

Expected: `kea` binary produced; no output, no error.

- [ ] **Step 4: Smoke-test the live server end-to-end**

Run in one terminal:

```bash
./kea serve &
SERVER_PID=$!
sleep 1
```

Then in the same shell:

```bash
curl -s http://localhost:8080/api/accounts | head -c 400; echo
curl -s http://localhost:8080/api/accounts/tree | head -c 400; echo
curl -s "http://localhost:8080/api/reports/balance-sheet" | head -c 400; echo
curl -s "http://localhost:8080/api/reports/net-worth" | head -c 400; echo
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/accounts/9999
curl -s -o /dev/null -w "%{http_code}\n" "http://localhost:8080/api/transactions?status=Wat"
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null
```

Expected:
- `/api/accounts` returns `{"items":[...],"total_count":...,"limit":0,"offset":0}` with the seed accounts.
- `/api/accounts/tree` returns a JSON array starting with `[{"account":...`.
- `/api/reports/balance-sheet` returns `{"assets":[...],"liabilities":[...],"equity":[...],...}`.
- `/api/reports/net-worth` returns `{"at":<unix>,"net_worth":{...}}`.
- `/api/accounts/9999` → `404`.
- `/api/transactions?status=Wat` → `400`.

- [ ] **Step 5: No commit needed**

This task only verifies; nothing to commit. If any step fails, return to the failing task and fix.

---

## Spec Coverage Check

| Spec section | Task(s) |
|--------------|---------|
| `GET /api/accounts` | 8 |
| `GET /api/accounts/tree` | 7 |
| `GET /api/accounts/by-name` | 5 |
| `GET /api/accounts/{id}` | 4 |
| `GET /api/accounts/{id}/balance` | 6 |
| `GET /api/transactions` | 10 |
| `GET /api/transactions/{id}` | 9 |
| `GET /api/reports/income-statement` | 11 |
| `GET /api/reports/income-breakdown` | 11 |
| `GET /api/reports/expense-breakdown` | 11 |
| `GET /api/reports/balance-sheet` | 12 |
| `GET /api/reports/net-worth` | 13 |
| Core query/path parsers | 1 |
| Composite parsers (ListOptions, AccountFilter, TransactionFilter, DateRangeParams) | 2 |
| In-memory test substrate + seeders | 3 |
| `applyHiddenFilter`, `wrapAsListResult` | 4, 8 |
| Error mapping (no new sentinels needed) | Foundation already covers — Tasks 4/5/8/9/10/11/12/13 exercise the existing mapper |
| Final smoke verification | 14 |
