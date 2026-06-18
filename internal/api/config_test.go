// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetConfig(t *testing.T) {
	tests := []struct {
		name         string
		currency     string
		hideDecimals bool
		wantBody     string
	}{
		{"populated", "USD", false, `{"defaults":{"currency":"USD"},"display":{"hide_decimals":false}}`},
		{"empty", "", false, `{"defaults":{"currency":""},"display":{"hide_decimals":false}}`},
		{"non_default", "TWD", false, `{"defaults":{"currency":"TWD"},"display":{"hide_decimals":false}}`},
		{"hide_decimals", "USD", true, `{"defaults":{"currency":"USD"},"display":{"hide_decimals":true}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _, _ := newServerForWriteWithDisplay(t, tt.currency, tt.hideDecimals)

			resp, err := http.Get(ts.URL + "/api/config")
			if err != nil {
				t.Fatalf("GET /api/config: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("status: got %d, want 200", resp.StatusCode)
			}
			if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type: got %q, want application/json", ct)
			}

			raw, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			got := strings.TrimSpace(string(raw))
			if got != tt.wantBody {
				t.Errorf("body: got %q, want %q", got, tt.wantBody)
			}

			var parsed struct {
				Defaults struct {
					Currency string `json:"currency"`
				} `json:"defaults"`
				Display struct {
					HideDecimals bool `json:"hide_decimals"`
				} `json:"display"`
			}
			if err := json.Unmarshal(raw, &parsed); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if parsed.Defaults.Currency != tt.currency {
				t.Errorf("parsed currency: got %q, want %q", parsed.Defaults.Currency, tt.currency)
			}
			if parsed.Display.HideDecimals != tt.hideDecimals {
				t.Errorf("parsed hide_decimals: got %v, want %v", parsed.Display.HideDecimals, tt.hideDecimals)
			}
		})
	}
}

func TestPatchConfig(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		wantStatus    int
		wantHide      bool
		wantSaveCalls int
	}{
		{"set_true", `{"display":{"hide_decimals":true}}`, http.StatusOK, true, 1},
		{"set_false", `{"display":{"hide_decimals":false}}`, http.StatusOK, false, 1},
		{"malformed_json", `not json`, http.StatusBadRequest, false, 0},
		{"unknown_top_field", `{"defaults":{"currency":"EUR"}}`, http.StatusBadRequest, false, 0},
		{"empty_body", `{}`, http.StatusBadRequest, false, 0},
		{"empty_display", `{"display":{}}`, http.StatusBadRequest, false, 0},
		{"wrong_type", `{"display":{"hide_decimals":"yes"}}`, http.StatusBadRequest, false, 0},
		{"unknown_display_field", `{"display":{"foo":true}}`, http.StatusBadRequest, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, cfg, saveCalls, _ := newServerForPatchConfig(t, "USD", false)

			req, err := http.NewRequest(http.MethodPatch, ts.URL+"/api/config", strings.NewReader(tt.body))
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("PATCH: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status: got %d, want %d. body=%s", resp.StatusCode, tt.wantStatus, body)
			}
			if cfg.Display.HideDecimals != tt.wantHide {
				t.Errorf("cfg.Display.HideDecimals: got %v, want %v", cfg.Display.HideDecimals, tt.wantHide)
			}
			if *saveCalls != tt.wantSaveCalls {
				t.Errorf("saveCalls: got %d, want %d", *saveCalls, tt.wantSaveCalls)
			}
		})
	}
}

func TestPatchConfig_DoesNotTouchOtherFields(t *testing.T) {
	ts, cfg, _, _ := newServerForPatchConfig(t, "TWD", false)
	originalCurrency := cfg.Defaults.Currency
	originalHost := cfg.Server.Host
	originalPort := cfg.Server.Port

	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/config",
		strings.NewReader(`{"display":{"hide_decimals":true}}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer resp.Body.Close()

	if cfg.Defaults.Currency != originalCurrency {
		t.Errorf("Defaults.Currency changed: got %q, want %q", cfg.Defaults.Currency, originalCurrency)
	}
	if cfg.Server.Host != originalHost {
		t.Errorf("Server.Host changed: got %q, want %q", cfg.Server.Host, originalHost)
	}
	if cfg.Server.Port != originalPort {
		t.Errorf("Server.Port changed: got %d, want %d", cfg.Server.Port, originalPort)
	}
}

func TestPatchConfig_RoundTrip(t *testing.T) {
	ts, _, _, _ := newServerForPatchConfig(t, "USD", false)

	patch := func(value bool) {
		body := fmt.Sprintf(`{"display":{"hide_decimals":%t}}`, value)
		req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PATCH: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PATCH status: %d", resp.StatusCode)
		}
	}

	getHide := func() bool {
		resp, err := http.Get(ts.URL + "/api/config")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		var parsed struct {
			Display struct {
				HideDecimals bool `json:"hide_decimals"`
			} `json:"display"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return parsed.Display.HideDecimals
	}

	patch(true)
	if got := getHide(); got != true {
		t.Errorf("after PATCH true: GET returned %v", got)
	}
	patch(false)
	if got := getHide(); got != false {
		t.Errorf("after PATCH false: GET returned %v", got)
	}
}

func TestPatchConfig_SaveErrorReturns500(t *testing.T) {
	ts, _, _, saveErr := newServerForPatchConfig(t, "USD", false)
	*saveErr = errors.New("disk full")

	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/config",
		strings.NewReader(`{"display":{"hide_decimals":true}}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}
