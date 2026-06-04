// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"
)

func getJSON(t *testing.T, url string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, body
}

func postJSON(t *testing.T, url string, payload any) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			t.Fatalf("encode payload: %v", err)
		}
	}
	resp, err := http.Post(url, "application/json", &buf)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, body
}

func deleteURL(t *testing.T, url string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, body
}

func TestHandleListLedgers_Empty(t *testing.T) {
	ts, _, _, _ := newTestServerWithLedger(t)

	status, body := getJSON(t, ts.URL+"/api/ledgers")
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", status, body)
	}

	var got struct {
		Active string `json:"active"`
		Items  []struct {
			Name   string `json:"name"`
			Path   string `json:"path"`
			Active bool   `json:"active"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.Active != "" {
		t.Errorf("active: got %q, want \"\"", got.Active)
	}
	if len(got.Items) != 0 {
		t.Errorf("items: got %v, want []", got.Items)
	}
}

func TestHandleListLedgers_TwoRegisteredOneActive(t *testing.T) {
	ts, reg, appDir, _ := newTestServerWithLedger(t)

	pathA := filepath.Join(appDir, "ledgers", "alpha.db")
	pathB := filepath.Join(appDir, "ledgers", "beta.db")
	if err := reg.Add("alpha", pathA); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if err := reg.Add("beta", pathB); err != nil {
		t.Fatalf("add beta: %v", err)
	}
	if err := reg.Switch("alpha"); err != nil {
		t.Fatalf("switch alpha: %v", err)
	}

	status, body := getJSON(t, ts.URL+"/api/ledgers")
	if status != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", status, body)
	}

	var got struct {
		Active string `json:"active"`
		Items  []struct {
			Name   string `json:"name"`
			Path   string `json:"path"`
			Active bool   `json:"active"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.Active != "alpha" {
		t.Errorf("active: got %q, want %q", got.Active, "alpha")
	}
	if len(got.Items) != 2 {
		t.Fatalf("items: got %d, want 2", len(got.Items))
	}
	// Items must be sorted by name: alpha, beta.
	if got.Items[0].Name != "alpha" || got.Items[1].Name != "beta" {
		t.Errorf("sort order: got [%q, %q], want [alpha, beta]",
			got.Items[0].Name, got.Items[1].Name)
	}
	if !got.Items[0].Active || got.Items[1].Active {
		t.Errorf("active flags: got [%v, %v], want [true, false]",
			got.Items[0].Active, got.Items[1].Active)
	}
	if got.Items[0].Path != pathA || got.Items[1].Path != pathB {
		t.Errorf("paths: got [%q, %q], want [%q, %q]",
			got.Items[0].Path, got.Items[1].Path, pathA, pathB)
	}
}
