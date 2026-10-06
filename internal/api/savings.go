// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

type savingsTargetListResponse struct {
	Items []model.SavingsTarget `json:"items"`
}

func (s *Server) handleListSavingsTargets(w http.ResponseWriter, r *http.Request) error {
	items, err := s.svc.Savings().ListSavingsTargets(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, savingsTargetListResponse{Items: items})
}

func (s *Server) handleSetSavingsTarget(w http.ResponseWriter, r *http.Request) error {
	var input model.SetSavingsTargetInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	t, err := s.svc.Savings().SetSavingsTarget(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleStopSavingsTarget(w http.ResponseWriter, r *http.Request) error {
	var input model.StopSavingsTargetInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	t, err := s.svc.Savings().StopSavingsTarget(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteSavingsTarget(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.Savings().DeleteSavingsTarget(r.Context(), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

func (s *Server) handleSavingsReport(w http.ResponseWriter, r *http.Request) error {
	report, err := s.svc.Savings().GenerateSavingsReport(r.Context(), r.URL.Query().Get("month"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, report)
}
