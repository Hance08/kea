// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hance08/kea/internal/ledger"
	"github.com/hance08/kea/internal/service"
)

func TestMapError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{"validation_error", &service.ValidationError{Field: "name", Message: "required"}, 400, "validation_failed", "name"},
		{"not_found", service.ErrNotFound, 404, "not_found", ""},
		{"not_found_wrapped", fmt.Errorf("account: %w", service.ErrNotFound), 404, "not_found", ""},
		{"already_exists", service.ErrAlreadyExists, 409, "already_exists", ""},
		{"reconciled", service.ErrReconciled, 409, "reconciled", ""},
		{"circular_parent", service.ErrCircularParent, 409, "circular_parent", ""},
		{"not_editable", service.ErrNotEditable, 403, "not_editable", ""},
		{"ledger_not_found", ledger.ErrLedgerNotFound, 404, "not_found", ""},
		{"ledger_not_found_wrapped", fmt.Errorf("ledger %q: %w", "x", ledger.ErrLedgerNotFound), 404, "not_found", ""},
		{"ledger_exists", ledger.ErrLedgerExists, 409, "already_exists", ""},
		{"ledger_exists_wrapped", fmt.Errorf("ledger %q: %w", "x", ledger.ErrLedgerExists), 409, "already_exists", ""},
		{"remove_active", ledger.ErrRemoveActive, 409, "cannot_remove_active", ""},
		{"remove_active_wrapped", fmt.Errorf("active: %w", ledger.ErrRemoveActive), 409, "cannot_remove_active", ""},
		{"no_active_ledger", ledger.ErrNoActiveLedger, 404, "no_active_ledger", ""},
		{"no_active_ledger_wrapped", fmt.Errorf("ledger: %w", ledger.ErrNoActiveLedger), 404, "no_active_ledger", ""},
		{"unknown", errors.New("boom"), 500, "internal", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := mapError(tc.err)
			if status != tc.wantStatus {
				t.Errorf("status: got %d, want %d", status, tc.wantStatus)
			}
			if body.Error != tc.wantCode {
				t.Errorf("error code: got %q, want %q", body.Error, tc.wantCode)
			}
			if body.Field != tc.wantField {
				t.Errorf("field: got %q, want %q", body.Field, tc.wantField)
			}
		})
	}
}

func TestMapErrorInternalHidesDetail(t *testing.T) {
	_, body := mapError(errors.New("database password=hunter2 leaked"))
	if body.Message != "internal server error" {
		t.Errorf("internal error must not expose detail in body; got %q", body.Message)
	}
}
