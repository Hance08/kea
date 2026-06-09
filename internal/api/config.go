// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import "net/http"

type configResponse struct {
	Defaults configDefaults `json:"defaults"`
}

type configDefaults struct {
	Currency string `json:"currency"`
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, http.StatusOK, configResponse{
		Defaults: configDefaults{
			Currency: s.svc.Config().Defaults.Currency,
		},
	})
}
