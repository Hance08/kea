# Fix Duplicate Reconcile IDs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject duplicate transaction IDs in reconciliation flows so that `clearedBalance` is never inflated by repeated IDs, preventing `last_reconciled_balance` corruption.

**Architecture:** Defense-in-depth at two layers. The service layer (`reconcile_ops.go`) validates uniqueness before computing `clearedBalance` — this is the authoritative guard since callers may bypass the CLI. The CLI helper `parseIDs` (`cmd/reconcile_actions.go`) deduplicates as a user-facing convenience, returning an error if duplicates are found.

**Tech Stack:** Go, standard library only (no new dependencies)

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/service/reconcile_ops.go:39-72` | Add seen-ID check in `PreviewReconcile` validation loop |
| Modify | `internal/service/reconcile_ops.go:90-150` | Add seen-ID check in `ReconcileTransactions` validation loop |
| Modify | `cmd/reconcile_actions.go:179-194` | Add duplicate rejection in `parseIDs` |
| Test   | `internal/service/reconcile_ops_test.go` | New tests for duplicate ID rejection |
| Test   | `cmd/reconcile_actions_test.go` | New test file for `parseIDs` duplicate rejection |

---

### Task 1: Service layer — reject duplicate IDs in PreviewReconcile

**Files:**
- Test: `internal/service/reconcile_ops_test.go`
- Modify: `internal/service/reconcile_ops.go:63-70`

- [ ] **Step 1: Write the failing test for PreviewReconcile with duplicate IDs**

Add to `internal/service/reconcile_ops_test.go`:

```go
func TestPreviewReconcile_DuplicateIDs_ReturnsError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	_, err := svc.PreviewReconcile(context.Background(), 1, 200000, []int64{10, 10})

	if err == nil {
		t.Fatal("expected error for duplicate transaction IDs, got nil")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "transactions" {
		t.Errorf("expected field 'transactions', got %q", ve.Field)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestPreviewReconcile_DuplicateIDs -v`
Expected: FAIL — duplicate ID 10 silently passes validation and returns diff instead of error.

- [ ] **Step 3: Add duplicate detection in PreviewReconcile**

In `internal/service/reconcile_ops.go`, replace the `clearedBalance` accumulation loop in `PreviewReconcile` (lines 63–70):

```go
	var clearedBalance int64
	seen := make(map[int64]bool, len(txIDs))
	for _, id := range txIDs {
		if seen[id] {
			return 0, validationErrorf("transactions", "duplicate transaction ID %d", id)
		}
		seen[id] = true
		amount, ok := validAmounts[id]
		if !ok {
			return 0, validationErrorf("transactions", "transaction ID %d is not in the unreconciled set for this account", id)
		}
		clearedBalance += amount
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/ -run TestPreviewReconcile -v`
Expected: ALL PreviewReconcile tests PASS (new + existing).

- [ ] **Step 5: Commit**

```bash
git add internal/service/reconcile_ops.go internal/service/reconcile_ops_test.go
git commit -m "fix: reject duplicate IDs in PreviewReconcile (#154)"
```

---

### Task 2: Service layer — reject duplicate IDs in ReconcileTransactions

**Files:**
- Test: `internal/service/reconcile_ops_test.go`
- Modify: `internal/service/reconcile_ops.go:118-125`

- [ ] **Step 1: Write the failing test for ReconcileTransactions with duplicate IDs**

Add to `internal/service/reconcile_ops_test.go`:

```go
func TestReconcileTransactions_DuplicateIDs_ReturnsError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	_, err := svc.ReconcileTransactions(context.Background(), 1, 200000, []int64{10, 10})

	if err == nil {
		t.Fatal("expected error for duplicate transaction IDs, got nil")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "transactions" {
		t.Errorf("expected field 'transactions', got %q", ve.Field)
	}
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Error("MarkSplitsReconciledByAccount must not be called when validation fails")
	}
	if len(txRepo.setLastReconciledBalCalls) != 0 {
		t.Error("SetLastReconciledBalance must not be called when validation fails")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestReconcileTransactions_DuplicateIDs -v`
Expected: FAIL — duplicate ID passes validation and inflates `clearedBalance`.

- [ ] **Step 3: Add duplicate detection in ReconcileTransactions**

In `internal/service/reconcile_ops.go`, replace the `clearedBalance` accumulation loop in `ReconcileTransactions` (lines 118–125):

```go
	var clearedBalance int64
	seen := make(map[int64]bool, len(txIDs))
	for _, id := range txIDs {
		if seen[id] {
			return 0, validationErrorf("transactions", "duplicate transaction ID %d", id)
		}
		seen[id] = true
		amount, ok := validAmounts[id]
		if !ok {
			return 0, validationErrorf("transactions", "transaction ID %d is not in the unreconciled set for this account", id)
		}
		clearedBalance += amount
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/ -run TestReconcileTransactions -v`
Expected: ALL ReconcileTransactions tests PASS (new + existing).

- [ ] **Step 5: Commit**

```bash
git add internal/service/reconcile_ops.go internal/service/reconcile_ops_test.go
git commit -m "fix: reject duplicate IDs in ReconcileTransactions (#154)"
```

---

### Task 3: CLI layer — reject duplicate IDs in parseIDs

**Files:**
- Create: `cmd/reconcile_actions_test.go`
- Modify: `cmd/reconcile_actions.go:179-194`

- [ ] **Step 1: Write the failing test for parseIDs with duplicate IDs**

Create `cmd/reconcile_actions_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"testing"
)

func TestParseIDs_ValidInput(t *testing.T) {
	ids, err := parseIDs("1, 2, 3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	if ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("expected [1 2 3], got %v", ids)
	}
}

func TestParseIDs_DuplicateIDs_ReturnsError(t *testing.T) {
	_, err := parseIDs("10,10")
	if err == nil {
		t.Fatal("expected error for duplicate IDs, got nil")
	}
}

func TestParseIDs_DuplicateIDs_NonAdjacent_ReturnsError(t *testing.T) {
	_, err := parseIDs("10,20,10")
	if err == nil {
		t.Fatal("expected error for duplicate IDs, got nil")
	}
}

func TestParseIDs_EmptyInput_ReturnsError(t *testing.T) {
	_, err := parseIDs("")
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestParseIDs_InvalidNumber_ReturnsError(t *testing.T) {
	_, err := parseIDs("10,abc,20")
	if err == nil {
		t.Fatal("expected error for invalid number, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify duplicate tests fail**

Run: `go test ./cmd/ -run TestParseIDs_Duplicate -v`
Expected: FAIL — `parseIDs` returns `[10 10]` without error.

- [ ] **Step 3: Add duplicate detection in parseIDs**

In `cmd/reconcile_actions.go`, replace the `parseIDs` function (lines 179–194):

```go
func parseIDs(s string) ([]int64, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("empty ID list")
	}
	parts := strings.Split(s, ",")
	seen := make(map[int64]bool, len(parts))
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ID %q: %w", p, err)
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate transaction ID %d", id)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}
```

- [ ] **Step 4: Run all parseIDs tests to verify they pass**

Run: `go test ./cmd/ -run TestParseIDs -v`
Expected: ALL TestParseIDs tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/reconcile_actions.go cmd/reconcile_actions_test.go
git commit -m "fix: reject duplicate IDs in parseIDs CLI helper (#154)"
```

---

### Task 4: Full regression check

**Files:** (none modified — verification only)

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: ALL tests PASS, zero failures.

- [ ] **Step 2: Run vet and build**

Run: `go vet ./... && make build`
Expected: No issues, clean build.
