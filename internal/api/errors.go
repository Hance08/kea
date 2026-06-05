// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/service"
)

type errorBody struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	Field      string `json:"field,omitempty"`
	Difference *int64 `json:"difference,omitempty"`
}

// balanceMismatchError is returned by the reconcile commit handler when the
// caller did not set allow_mismatch=true and the computed diff is non-zero.
// Mirrors --force semantics from the CLI; lives in the API layer because the
// service contract always persists regardless of diff.
type balanceMismatchError struct {
	Difference int64
}

func (e *balanceMismatchError) Error() string {
	return fmt.Sprintf("statement balance off by %d", e.Difference)
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
	if errors.As(err, &verr) {
		return http.StatusBadRequest, errorBody{
			Error: "validation_failed", Message: verr.Message, Field: verr.Field,
		}
	}
	var bme *balanceMismatchError
	if errors.As(err, &bme) {
		diff := bme.Difference
		return http.StatusConflict, errorBody{
			Error:      "balance_mismatch",
			Message:    bme.Error(),
			Difference: &diff,
		}
	}
	switch {
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
	case errors.Is(err, ledger.ErrLedgerNotFound):
		return http.StatusNotFound, errorBody{Error: "not_found", Message: err.Error()}
	case errors.Is(err, ledger.ErrLedgerExists):
		return http.StatusConflict, errorBody{Error: "already_exists", Message: err.Error()}
	case errors.Is(err, ledger.ErrRemoveActive):
		return http.StatusConflict, errorBody{Error: "cannot_remove_active", Message: err.Error()}
	case errors.Is(err, ledger.ErrNoActiveLedger):
		return http.StatusNotFound, errorBody{Error: "no_active_ledger", Message: err.Error()}
	default:
		return http.StatusInternalServerError, errorBody{Error: "internal", Message: "internal server error"}
	}
}
