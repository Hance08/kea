// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"testing"
)

func TestNewServeCmdShape(t *testing.T) {
	cmd := NewServeCmd(nil, nil, "")
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
