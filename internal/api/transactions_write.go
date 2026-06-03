// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

func (s *Server) handleCreateTransaction(w http.ResponseWriter, r *http.Request) error {
	var input model.CreateTransactionFromSplitsInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	ctx := r.Context()
	detail, err := s.svc.Transaction().CreateTransactionFromSplits(ctx, input)
	if err != nil {
		return err
	}
	full, err := s.svc.Transaction().GetTransactionByID(ctx, detail.ID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, full)
}

func (s *Server) handleDeleteTransaction(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.Transaction().DeleteTransaction(r.Context(), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}
