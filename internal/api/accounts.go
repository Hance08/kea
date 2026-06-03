// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

// wrapAsListResult wraps a flat slice into a ListResult with limit=0/offset=0
// and total_count=len. Used by /api/accounts when the request has no filter or
// pagination intent so the response shape is consistent.
func wrapAsListResult(accounts []*model.Account) *model.ListResult[*model.Account] {
	return &model.ListResult[*model.Account]{
		Items:      accounts,
		TotalCount: len(accounts),
		Limit:      0,
		Offset:     0,
	}
}

func (s *Server) handleAccountByID(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	acc, err := s.svc.Account().GetAccountByID(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, acc)
}
