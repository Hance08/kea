// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/repository"
)

func TestGetTransactionByID_RepoError_WrapsWithContext(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	repoErr := fmt.Errorf("db connection lost")
	txRepo.getByIDErr[99] = repoErr

	_, err := svc.GetTransactionByID(context.Background(), 99)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("expected error to wrap %v, got %v", repoErr, err)
	}
	if !strings.Contains(err.Error(), "failed to retrieve transaction") {
		t.Errorf("expected context message in error, got: %s", err.Error())
	}
}

func TestGetTransactionByID_NotFound_WrapsServiceErrNotFound(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	// Simulate repository.ErrNotFound
	txRepo.getByIDErr[999] = repository.ErrNotFound

	_, err := svc.GetTransactionByID(context.Background(), 999)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected error to wrap service.ErrNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "transaction") {
		t.Errorf("expected 'transaction' in error message, got: %s", err.Error())
	}
}
