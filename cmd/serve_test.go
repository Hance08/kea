// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"testing"

	"github.com/hance08/kea/internal/config"
)

func TestNewServeCmdShape(t *testing.T) {
	cmd := NewServeCmd(nil, &config.Config{})
	if cmd.Use != "serve" {
		t.Errorf("Use: got %q, want %q", cmd.Use, "serve")
	}
	if cmd.Short == "" {
		t.Error("Short description must not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE must be set")
	}
}
