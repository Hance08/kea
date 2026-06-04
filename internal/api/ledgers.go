// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"
)

type ledgerInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Active bool   `json:"active"`
}

type ledgerListResponse struct {
	Active string       `json:"active"`
	Items  []ledgerInfo `json:"items"`
}

type createLedgerRequest struct {
	Name string `json:"name"`
}

type switchLedgerRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleListLedgers(w http.ResponseWriter, r *http.Request) error {
	active := s.registry.ActiveName()
	names := s.registry.Names()
	items := make([]ledgerInfo, 0, len(names))
	for _, n := range names {
		e, _ := s.registry.EntryFor(n)
		items = append(items, ledgerInfo{Name: n, Path: e.Path, Active: n == active})
	}
	return writeJSON(w, http.StatusOK, ledgerListResponse{Active: active, Items: items})
}
