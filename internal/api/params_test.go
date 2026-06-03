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
