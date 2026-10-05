// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

type budgetListResponse struct {
	Items []model.Budget `json:"items"`
}

func (s *Server) handleListBudgets(w http.ResponseWriter, r *http.Request) error {
	items, err := s.svc.Budget().ListBudgets(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, budgetListResponse{Items: items})
}

func (s *Server) handleSetBudget(w http.ResponseWriter, r *http.Request) error {
	var input model.SetBudgetInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	b, err := s.svc.Budget().SetBudget(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleStopBudget(w http.ResponseWriter, r *http.Request) error {
	var input model.StopBudgetInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	b, err := s.svc.Budget().StopBudget(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleDeleteBudget(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.Budget().DeleteBudget(r.Context(), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

func (s *Server) handleBudgetReport(w http.ResponseWriter, r *http.Request) error {
	report, err := s.svc.Budget().GenerateBudgetReport(r.Context(), r.URL.Query().Get("month"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, report)
}
