// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/hance08/kea/internal/service"
)

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
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	got, err := parseInt64Path(r, "id")
	if err != nil || got != 42 {
		t.Fatalf("got (%d, %v), want (42, nil)", got, err)
	}

	r2 := httptest.NewRequest("GET", "/x", nil)
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("id", "xyz")
	r2 = r2.WithContext(context.WithValue(r2.Context(), chi.RouteCtxKey, rctx2))
	_, err = parseInt64Path(r2, "id")
	var verr *service.ValidationError
	if !errors.As(err, &verr) || verr.Field != "id" {
		t.Fatalf("expected *ValidationError{Field:\"id\"}, got %v", err)
	}
}
