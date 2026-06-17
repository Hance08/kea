// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"encoding/json"
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
