// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
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
