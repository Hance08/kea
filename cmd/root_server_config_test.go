// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"testing"

	"github.com/spf13/viper"
)

func TestInitConfig_ServerDefaults(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")

	setServerDefaults(v)

	if v.GetString("server.host") != "localhost" {
		t.Errorf("expected server.host %q, got %q", "localhost", v.GetString("server.host"))
	}
	if v.GetInt("server.port") != 8080 {
		t.Errorf("expected server.port %d, got %d", 8080, v.GetInt("server.port"))
	}
	origins := v.GetStringSlice("server.cors_origins")
	if len(origins) != 1 || origins[0] != "http://localhost:5173" {
		t.Errorf("expected server.cors_origins [http://localhost:5173], got %v", origins)
	}
}
