// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"fmt"
	"net/http"

	"github.com/hance08/kea/internal/ledger"
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

func (s *Server) handleActiveLedger(w http.ResponseWriter, r *http.Request) error {
	name := s.registry.ActiveName()
	if name == "" {
		return ledger.ErrNoActiveLedger
	}
	e, ok := s.registry.EntryFor(name)
	if !ok {
		return fmt.Errorf("%w: %q", ledger.ErrLedgerNotFound, name)
	}
	return writeJSON(w, http.StatusOK, ledgerInfo{Name: name, Path: e.Path, Active: true})
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
