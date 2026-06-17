// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import "net/http"

type configResponse struct {
	Defaults configDefaults `json:"defaults"`
	Display  configDisplay  `json:"display"`
}

type configDefaults struct {
	Currency string `json:"currency"`
}

type configDisplay struct {
	HideDecimals bool `json:"hide_decimals"`
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) error {
	cfg := s.svc.Config()
	return writeJSON(w, http.StatusOK, configResponse{
		Defaults: configDefaults{Currency: cfg.Defaults.Currency},
		Display:  configDisplay{HideDecimals: cfg.Display.HideDecimals},
	})
}
