// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

type unreconciledResponse struct {
	Entries               []*model.ReconcileEntry `json:"entries"`
	LastReconciledBalance int64                   `json:"last_reconciled_balance"`
}

func (s *Server) handleListUnreconciled(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	if _, err := s.svc.Account().GetAccountByID(ctx, id); err != nil {
		return err
	}
	entries, lastBalance, err := s.svc.Transaction().GetUnreconciledByAccount(ctx, id)
	if err != nil {
		return err
	}
	if entries == nil {
		entries = []*model.ReconcileEntry{}
	}
	return writeJSON(w, http.StatusOK, unreconciledResponse{
		Entries:               entries,
		LastReconciledBalance: lastBalance,
	})
}
