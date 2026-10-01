# GetAccountByID Public Method Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose `GetAccountByID` as a public method on `AccountService` so the web layer can look up accounts by numeric ID without bypassing the service layer.

**Architecture:** Add a single public method following the existing `GetAccountByName` pattern (wrap `repository.ErrNotFound` into `service.ErrNotFound`). Test with the existing mock infrastructure.

**Tech Stack:** Go, testify (assert/require)

---

### Task 1: Add GetAccountByID method with TDD

**Files:**
- Create: `internal/service/account_service_test.go`
- Modify: `internal/service/account_service.go:41` (insert after `GetAccountByName`)

- [ ] **Step 1: Write the failing tests**

Create `internal/service/account_service_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAccountByID(t *testing.T) {
	t.Run("returns account when found", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.GetAccountByID(context.Background(), 1)

		require.NoError(t, err)
		assert.Equal(t, int64(1), acc.ID)
		assert.Equal(t, "Assets:Cash", acc.Name)
	})

	t.Run("wraps ErrNotFound from repo", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.GetAccountByID(context.Background(), 999)

		assert.Nil(t, acc)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.Contains(t, err.Error(), "999")
	})

	t.Run("passes through other errors", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		dbErr := fmt.Errorf("connection refused")
		accRepo.getByIDErr[42] = dbErr
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.GetAccountByID(context.Background(), 42)

		assert.Nil(t, acc)
		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrNotFound))
		assert.Equal(t, dbErr, err)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestGetAccountByID -v`
Expected: compilation error — `svc.GetAccountByID` undefined

- [ ] **Step 3: Implement GetAccountByID**

Add to `internal/service/account_service.go` after `GetAccountByName` (after line 41):

```go
func (as *AccountService) GetAccountByID(ctx context.Context, id int64) (*model.Account, error) {
	acc, err := as.repo.GetAccountByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("account #%d: %w", id, ErrNotFound)
		}
		return nil, err
	}
	return acc, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestGetAccountByID -v`
Expected: all 3 subtests PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: all tests PASS, no regressions

- [ ] **Step 6: Commit**

```bash
git add internal/service/account_service.go internal/service/account_service_test.go
git commit -m "feat: add public GetAccountByID to AccountService (#104)"
```
