// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/hance08/kea/internal/app"
	"github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/service"
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

type deleteLedgerResponse struct {
	Deleted bool   `json:"deleted"`
	Name    string `json:"name"`
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

func validateLedgerName(name string) error {
	if name == "" {
		return &service.ValidationError{Field: "name", Message: "name is required"}
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return &service.ValidationError{Field: "name", Message: "name must not contain path separators"}
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return &service.ValidationError{Field: "name", Message: "name must not contain control characters"}
		}
	}
	return nil
}

func (s *Server) handleCreateLedger(w http.ResponseWriter, r *http.Request) error {
	var req createLedgerRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if err := validateLedgerName(req.Name); err != nil {
		return err
	}
	if _, exists := s.registry.EntryFor(req.Name); exists {
		return fmt.Errorf("%w: %q", ledger.ErrLedgerExists, req.Name)
	}
	dbPath := filepath.Join(s.appDir, "ledgers", req.Name+".db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("create ledger directory: %w", err)
	}
	if err := app.InitLedgerDB(dbPath, s.migrations); err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	if err := s.registry.Add(req.Name, dbPath); err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, ledgerInfo{Name: req.Name, Path: dbPath, Active: false})
}

func (s *Server) handleSwitchLedger(w http.ResponseWriter, r *http.Request) error {
	var req switchLedgerRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if err := validateLedgerName(req.Name); err != nil {
		return err
	}
	if err := s.switchLedger(req.Name); err != nil {
		return err
	}
	e, _ := s.registry.EntryFor(req.Name)
	return writeJSON(w, http.StatusOK, ledgerInfo{Name: req.Name, Path: e.Path, Active: true})
}

func (s *Server) handleDeleteLedger(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")
	if err := validateLedgerName(name); err != nil {
		return err
	}
	if err := s.registry.Remove(name, false); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, deleteLedgerResponse{Deleted: true, Name: name})
}
