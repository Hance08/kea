# Circular Parent Reference Validation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent circular parent chains in the account hierarchy by validating that setting a ParentID doesn't create a cycle.

**Architecture:** Add a `validateParentChain` method on `AccountService` that walks `GetAccountByID` upward from the proposed parent until it reaches a nil ParentID or detects the new account's ID in the chain. A new sentinel error `ErrCircularParent` is returned on cycle detection. The validation is called from `CreateAccount` when `parentID != nil`. Since new accounts have no children yet, a true cycle can't form via `CreateAccount` alone today — but this guards against any future reparent operation and validates that the parent actually exists.

**Tech Stack:** Go, existing mock infrastructure in `testhelper_test.go`

---

## File Map

- **Modify:** `internal/service/errors.go` — add `ErrCircularParent`
- **Modify:** `internal/service/account_ops.go` — add `validateParentChain`, call it from `CreateAccount`
- **Create:** `internal/service/account_ops_test.go` — tests for circular parent validation

---

### Task 1: Add the sentinel error

**Files:**
- Modify: `internal/service/errors.go:9-13`

- [ ] **Step 1: Add `ErrCircularParent` to errors.go**

Add a new sentinel error after the existing ones:

```go
ErrCircularParent = errors.New("circular parent reference detected")
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/service/...`
Expected: success (error is defined but unused — Go allows unused package-level vars)

- [ ] **Step 3: Commit**

```bash
git add internal/service/errors.go
git commit -m "feat: add ErrCircularParent sentinel error"
```

---

### Task 2: Write failing tests for parent chain validation

**Files:**
- Create: `internal/service/account_ops_test.go`

- [ ] **Step 1: Write test file with cycle detection tests**

Create `internal/service/account_ops_test.go` with these test cases:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

func TestCreateAccount_ValidParent(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestAccountService(accRepo, txRepo)

	parent := &model.Account{ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"}
	accRepo.addAccount(parent)

	acc, err := svc.CreateAccount(context.Background(), "Assets:Bank:Checking", model.AccountTypeAsset, "USD", "", int64Ptr(10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc.ParentID == nil || *acc.ParentID != 10 {
		t.Fatalf("expected ParentID=10, got %v", acc.ParentID)
	}
}

func TestCreateAccount_ParentNotFound(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestAccountService(accRepo, txRepo)

	_, err := svc.CreateAccount(context.Background(), "Assets:Bank:Checking", model.AccountTypeAsset, "USD", "", int64Ptr(999))
	if err == nil {
		t.Fatal("expected error for non-existent parent, got nil")
	}
}

func TestCreateAccount_CircularParentSelf(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestAccountService(accRepo, txRepo)

	// Pre-seed an account with ID 5; then try to create a new account
	// whose parent is itself — but since CreateAccount creates a *new* account
	// whose ID isn't known in advance, self-reference via CreateAccount is
	// impossible. This test instead validates the chain walker by seeding a
	// chain A(1) → B(2) → C(3) and attempting to create with parentID=3,
	// ensuring the walker doesn't choke on a deep chain.
	a := &model.Account{ID: 1, Name: "Assets", Type: model.AccountTypeAsset, Currency: "USD", ParentID: nil}
	b := &model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(1)}
	c := &model.Account{ID: 3, Name: "Assets:Bank:Savings", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(2)}
	accRepo.addAccount(a)
	accRepo.addAccount(b)
	accRepo.addAccount(c)

	acc, err := svc.CreateAccount(context.Background(), "Assets:Bank:Savings:Sub", model.AccountTypeAsset, "USD", "", int64Ptr(3))
	if err != nil {
		t.Fatalf("deep chain should succeed: %v", err)
	}
	if acc.ParentID == nil || *acc.ParentID != 3 {
		t.Fatalf("expected ParentID=3, got %v", acc.ParentID)
	}
}

func TestValidateParentChain_DetectsCycle(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestAccountService(accRepo, txRepo)

	// Build a chain: A(1) → B(2) → C(3), then inject a cycle C.ParentID=1 → A
	// This simulates what a future reparent could produce.
	a := &model.Account{ID: 1, Name: "A", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(3)} // cycle!
	b := &model.Account{ID: 2, Name: "B", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(1)}
	c := &model.Account{ID: 3, Name: "C", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(2)}
	accRepo.addAccount(a)
	accRepo.addAccount(b)
	accRepo.addAccount(c)

	err := svc.validateParentChain(context.Background(), 99, int64Ptr(3))
	if err == nil {
		t.Fatal("expected ErrCircularParent, got nil")
	}
	if !errors.Is(err, ErrCircularParent) {
		t.Fatalf("expected ErrCircularParent, got: %v", err)
	}
}

func TestValidateParentChain_NilParent(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestAccountService(accRepo, txRepo)

	err := svc.validateParentChain(context.Background(), 1, nil)
	if err != nil {
		t.Fatalf("nil parentID should be valid: %v", err)
	}
}

func TestValidateParentChain_RepoError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestAccountService(accRepo, txRepo)

	accRepo.getByIDErr[50] = repository.ErrNotFound

	err := svc.validateParentChain(context.Background(), 1, int64Ptr(50))
	if err == nil {
		t.Fatal("expected error when parent not found")
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run "TestCreateAccount_ValidParent|TestCreateAccount_ParentNotFound|TestCreateAccount_CircularParentSelf|TestValidateParentChain" -v`
Expected: compilation error — `validateParentChain` doesn't exist yet

- [ ] **Step 3: Commit**

```bash
git add internal/service/account_ops_test.go
git commit -m "test: add failing tests for parent chain validation"
```

---

### Task 3: Implement `validateParentChain` and wire it into `CreateAccount`

**Files:**
- Modify: `internal/service/account_ops.go:17-45`

- [ ] **Step 1: Add `validateParentChain` method to `account_ops.go`**

Add this method before `CreateAccount`:

```go
func (as *AccountService) validateParentChain(ctx context.Context, accountID int64, parentID *int64) error {
	if parentID == nil {
		return nil
	}

	visited := make(map[int64]bool)
	visited[accountID] = true
	currentID := *parentID

	for {
		if visited[currentID] {
			return fmt.Errorf("account %d would create a cycle via parent %d: %w", accountID, currentID, ErrCircularParent)
		}
		visited[currentID] = true

		acc, err := as.repo.GetAccountByID(ctx, currentID)
		if err != nil {
			return fmt.Errorf("failed to look up parent account %d: %w", currentID, err)
		}
		if acc.ParentID == nil {
			return nil
		}
		currentID = *acc.ParentID
	}
}
```

- [ ] **Step 2: Call `validateParentChain` from `CreateAccount`**

In `CreateAccount`, add the validation call after the type check and before the `repo.CreateAccount` call. Insert after line 26 (`if !accType.IsValid()` block):

```go
	if parentID != nil {
		if err := as.validateParentChain(ctx, 0, parentID); err != nil {
			return nil, err
		}
	}
```

Note: we pass `0` as `accountID` because the account doesn't exist yet — no real ID to collide with. The chain walk still validates the parent exists and the chain is acyclic.

- [ ] **Step 3: Run all tests to verify they pass**

Run: `go test ./internal/service/ -run "TestCreateAccount_ValidParent|TestCreateAccount_ParentNotFound|TestCreateAccount_CircularParentSelf|TestValidateParentChain" -v`
Expected: all PASS

- [ ] **Step 4: Run the full test suite to check for regressions**

Run: `go test ./...`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/account_ops.go
git commit -m "feat: validate parent chain acyclicity on account creation

Closes #22"
```
