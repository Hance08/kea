// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/hance08/kea/internal/config"
)

// TestViperWriteLeaks_NeedsFallback documents why saveDisplayHideDecimals
// cannot just call viper.WriteConfig: viper merges in-memory defaults
// registered via SetDefault (here, the server.* keys registered by
// setServerDefaults) into the on-disk YAML, silently growing the user's
// config file with keys they never typed. This test asserts the leak
// behavior so a future viper upgrade that fixes it will surface here and
// signal that the in-place YAML rewrite in saveDisplayHideDecimals could be
// simplified back to viper.WriteConfig.
func TestViperWriteLeaks_NeedsFallback(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	initial := "defaults:\n  currency: TWD\ndisplay:\n  hide_decimals: true\n"
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write initial yaml: %v", err)
	}

	viper.SetConfigFile(cfgPath)
	setServerDefaults(viper.GetViper())
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("ReadInConfig: %v", err)
	}

	viper.Set("display.hide_decimals", false)
	if err := viper.WriteConfig(); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read written yaml: %v", err)
	}
	written := string(raw)

	leakedKeys := []string{"server", "host:", "port:", "cors_origins"}
	leaks := 0
	for _, leaked := range leakedKeys {
		if strings.Contains(written, leaked) {
			leaks++
		}
	}
	if leaks == 0 {
		t.Errorf("viper.WriteConfig no longer leaks server defaults; the "+
			"fallback in saveDisplayHideDecimals may now be unnecessary.\n"+
			"Full contents:\n%s", written)
	}
}

// TestSaveDisplayHideDecimals_NoLeak_DirectMarshal exercises the production
// saveDisplayHideDecimals closure and proves it does NOT leak
// viper-registered server defaults into the user's config file, while still
// updating display.hide_decimals and preserving existing keys.
func TestSaveDisplayHideDecimals_NoLeak_DirectMarshal(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	initial := "defaults:\n  currency: TWD\ndisplay:\n  hide_decimals: true\n"
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write initial yaml: %v", err)
	}

	// Simulate the runtime: viper is configured with the same defaults the
	// real `kea serve` command sets, but saveDisplayHideDecimals must not
	// touch viper — it writes directly to disk based on cfg.
	viper.SetConfigFile(cfgPath)
	setServerDefaults(viper.GetViper())
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("ReadInConfig: %v", err)
	}

	cfg := config.NewDefault()
	cfg.Defaults.Currency = "TWD"
	cfg.Display.HideDecimals = false
	cfg.ConfigPath = cfgPath

	if err := saveDisplayHideDecimals(cfg); err != nil {
		t.Fatalf("saveDisplayHideDecimals: %v", err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read written yaml: %v", err)
	}
	written := string(raw)

	for _, leaked := range []string{"server", "host:", "port:", "cors_origins", "database"} {
		if strings.Contains(written, leaked) {
			t.Errorf("written yaml contains leaked key %q.\nFull contents:\n%s", leaked, written)
		}
	}

	if !strings.Contains(written, "hide_decimals: false") {
		t.Errorf("written yaml missing updated hide_decimals=false.\nFull contents:\n%s", written)
	}
	if !strings.Contains(written, "currency: TWD") {
		t.Errorf("written yaml dropped existing currency.\nFull contents:\n%s", written)
	}
}

// TestSaveDisplayHideDecimals_PreservesUnrelatedKeys ensures the in-place
// rewrite preserves any keys the user has actually written to their config
// (e.g. customized server.port), since the default-config template ships
// with a server block. Only the targeted key may change.
func TestSaveDisplayHideDecimals_PreservesUnrelatedKeys(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	initial := "defaults:\n  currency: USD\ndisplay:\n  hide_decimals: false\n" +
		"server:\n  host: 0.0.0.0\n  port: 9999\n  cors_origins:\n    - http://example.test\n" +
		"database:\n  path: /custom/path.db\n"
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write initial yaml: %v", err)
	}

	cfg := config.NewDefault()
	cfg.Defaults.Currency = "USD"
	cfg.Display.HideDecimals = true
	cfg.ConfigPath = cfgPath

	if err := saveDisplayHideDecimals(cfg); err != nil {
		t.Fatalf("saveDisplayHideDecimals: %v", err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read written yaml: %v", err)
	}
	written := string(raw)

	for _, want := range []string{
		"hide_decimals: true",
		"host: 0.0.0.0",
		"port: 9999",
		"http://example.test",
		"/custom/path.db",
		"currency: USD",
	} {
		if !strings.Contains(written, want) {
			t.Errorf("written yaml missing %q.\nFull contents:\n%s", want, written)
		}
	}
}

// TestSaveDisplayHideDecimals_CreatesKeyWhenMissing covers the case where
// the user's existing YAML has no display block at all — the closure must
// add display.hide_decimals rather than failing.
func TestSaveDisplayHideDecimals_CreatesKeyWhenMissing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	initial := "defaults:\n  currency: JPY\n"
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write initial yaml: %v", err)
	}

	cfg := config.NewDefault()
	cfg.Defaults.Currency = "JPY"
	cfg.Display.HideDecimals = true
	cfg.ConfigPath = cfgPath

	if err := saveDisplayHideDecimals(cfg); err != nil {
		t.Fatalf("saveDisplayHideDecimals: %v", err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read written yaml: %v", err)
	}
	written := string(raw)

	if !strings.Contains(written, "hide_decimals: true") {
		t.Errorf("written yaml missing hide_decimals: true.\nFull contents:\n%s", written)
	}
	if !strings.Contains(written, "currency: JPY") {
		t.Errorf("written yaml dropped existing currency.\nFull contents:\n%s", written)
	}
	if strings.Contains(written, "server") {
		t.Errorf("written yaml leaked server block.\nFull contents:\n%s", written)
	}
}
