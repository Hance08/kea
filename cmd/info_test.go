// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"testing"

	"github.com/hance08/kea/internal/app"
	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/ui/views"
)

type fakeInfoProvider struct {
	cfg     *config.Config
	runtime app.RuntimeState
}

func (f *fakeInfoProvider) Config() *config.Config        { return f.cfg }
func (f *fakeInfoProvider) RuntimeState() app.RuntimeState { return f.runtime }

type capturingView struct {
	got views.SystemInfo
}

func (c *capturingView) Render(info views.SystemInfo) error {
	c.got = info
	return nil
}

// TestInfoRunner_UsesRuntimeStateNotConfig asserts that the info command
// displays the path the store is actually open against (RuntimeState), not
// the value originally loaded from yaml (Config.Database.Path).
func TestInfoRunner_UsesRuntimeStateNotConfig(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Database.Path = "/yaml/loaded/path.db"
	cfg.Defaults.Currency = "USD"

	provider := &fakeInfoProvider{
		cfg: cfg,
		runtime: app.RuntimeState{
			ActiveLedger: "alpha",
			DatabasePath: "/runtime/active/path.db",
		},
	}
	view := &capturingView{}

	runner := &infoRunner{svc: provider, view: view, json: false}
	if err := runner.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if view.got.ActiveLedger != "alpha" {
		t.Errorf("ActiveLedger: got %q, want %q", view.got.ActiveLedger, "alpha")
	}
	// expandPath leaves absolute paths untouched.
	if view.got.DBPath != "/runtime/active/path.db" {
		t.Errorf("DBPath: got %q, want %q (should come from RuntimeState, not Config)",
			view.got.DBPath, "/runtime/active/path.db")
	}
}

// TestInfoRunner_FallsBackToDefaultWhenRuntimePathEmpty pins the existing
// fallback: if RuntimeState.DatabasePath is empty, the runner falls back to
// <appDataDir>/kea.db. This branch is reachable in unit tests where a fake
// provider returns an empty runtime; production NewApp always seeds a
// non-empty path because registry.Active() guarantees one.
func TestInfoRunner_FallsBackToDefaultWhenRuntimePathEmpty(t *testing.T) {
	cfg := config.NewDefault()
	cfg.Defaults.Currency = "USD"

	provider := &fakeInfoProvider{
		cfg:     cfg,
		runtime: app.RuntimeState{},
	}
	view := &capturingView{}

	runner := &infoRunner{svc: provider, view: view, json: false}
	if err := runner.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if view.got.DBPath == "" {
		t.Errorf("DBPath fell through to empty; expected <appDataDir>/kea.db fallback")
	}
}
