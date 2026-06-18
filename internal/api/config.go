// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"fmt"
	"net/http"

	"github.com/hance08/kea/internal/service"
)

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

type configPatchRequest struct {
	Display *configDisplayPatch `json:"display"`
}

type configDisplayPatch struct {
	HideDecimals *bool `json:"hide_decimals"`
}

func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) error {
	var req configPatchRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if req.Display == nil || req.Display.HideDecimals == nil {
		return &service.ValidationError{Field: "display.hide_decimals", Message: "required"}
	}

	cfg := s.svc.Config()
	cfg.Display.HideDecimals = *req.Display.HideDecimals

	if err := s.saveConfig(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return writeJSON(w, http.StatusOK, configResponse{
		Defaults: configDefaults{Currency: cfg.Defaults.Currency},
		Display:  configDisplay{HideDecimals: cfg.Display.HideDecimals},
	})
}
