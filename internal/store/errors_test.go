// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"errors"
	"testing"

	"github.com/hance08/kea/internal/repository"
)

func TestErrorSentinelsWrapRepositoryErrors(t *testing.T) {
	tests := []struct {
		name      string
		storeErr  error
		repoErr   error
	}{
		{"ErrRecordNotFound wraps repository.ErrNotFound", ErrRecordNotFound, repository.ErrNotFound},
		{"ErrAccountExists wraps repository.ErrAlreadyExists", ErrAccountExists, repository.ErrAlreadyExists},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.storeErr, tt.repoErr) {
				t.Errorf("%v should wrap %v", tt.storeErr, tt.repoErr)
			}
		})
	}
}
