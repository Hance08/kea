// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"errors"
	"net/http"

	"github.com/hance08/kea/internal/service"
)

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, body := mapError(err)
	loggerFrom(r.Context()).Error("request failed",
		"error_code", body.Error,
		"status", status,
		"err", err.Error(),
	)
	_ = writeJSON(w, status, body)
}

func mapError(err error) (int, errorBody) {
	var verr *service.ValidationError
	switch {
	case errors.As(err, &verr):
		return http.StatusBadRequest, errorBody{
			Error: "validation_failed", Message: verr.Message, Field: verr.Field,
		}
	case errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound, errorBody{Error: "not_found", Message: err.Error()}
	case errors.Is(err, service.ErrAlreadyExists):
		return http.StatusConflict, errorBody{Error: "already_exists", Message: err.Error()}
	case errors.Is(err, service.ErrReconciled):
		return http.StatusConflict, errorBody{Error: "reconciled", Message: err.Error()}
	case errors.Is(err, service.ErrCircularParent):
		return http.StatusConflict, errorBody{Error: "circular_parent", Message: err.Error()}
	case errors.Is(err, service.ErrNotEditable):
		return http.StatusForbidden, errorBody{Error: "not_editable", Message: err.Error()}
	default:
		return http.StatusInternalServerError, errorBody{Error: "internal", Message: "internal server error"}
	}
}
