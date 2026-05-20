// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package config

import "testing"

func TestNewDefault_ServerConfig(t *testing.T) {
	cfg := NewDefault()

	if cfg.Server.Host != "localhost" {
		t.Errorf("expected default host %q, got %q", "localhost", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port %d, got %d", 8080, cfg.Server.Port)
	}
	if len(cfg.Server.CORSOrigins) != 1 || cfg.Server.CORSOrigins[0] != "http://localhost:5173" {
		t.Errorf("expected default CORS origins [http://localhost:5173], got %v", cfg.Server.CORSOrigins)
	}
}
