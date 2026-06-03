// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
	var input model.CreateAccountInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	acc, err := s.svc.Account().CreateAccountWithBalance(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, acc)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	acc, err := s.svc.Account().GetAccountByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.svc.Account().DeleteAccountByName(ctx, acc.Name); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}
