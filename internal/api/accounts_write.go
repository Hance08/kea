// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
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

type updateAccountRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsHidden    *bool   `json:"is_hidden,omitempty"`
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	var req updateAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if req.Name == nil && req.Description == nil && req.IsHidden == nil {
		return &service.ValidationError{Message: "no updatable fields provided"}
	}

	ctx := r.Context()
	current, err := s.svc.Account().GetAccountByID(ctx, id)
	if err != nil {
		return err
	}

	updated := current
	if req.Name != nil && *req.Name != current.Name {
		renamed, err := s.svc.Account().RenameAccount(ctx, current.Name, *req.Name)
		if err != nil {
			return err
		}
		updated = renamed
	}
	if req.Description != nil || req.IsHidden != nil {
		desc, hidden := updated.Description, updated.IsHidden
		if req.Description != nil {
			desc = *req.Description
		}
		if req.IsHidden != nil {
			hidden = *req.IsHidden
		}
		meta, err := s.svc.Account().UpdateAccountMetadata(ctx, updated.ID, desc, hidden)
		if err != nil {
			return err
		}
		updated = meta
	}
	return writeJSON(w, http.StatusOK, updated)
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
