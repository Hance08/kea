// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import "net/http"

// Version is the server version string. Override at build time via:
//
//	-ldflags "-X github.com/hance08/kea/internal/api.Version=v0.1.0"
var Version = "dev"

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, http.StatusOK, map[string]string{"version": Version})
}
