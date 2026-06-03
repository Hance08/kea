// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import "net/http"

func (s *Server) handleTransactionByID(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	d, err := s.svc.Transaction().GetTransactionByID(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, d)
}
