# Fix Inconsistent Error Wrapping in GetTransactionByID — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wrap the bare error on line 55 of `transaction_service.go` so both error paths in `GetTransactionByID` use consistent `fmt.Errorf("...: %w", err)` wrapping.

**Architecture:** Single-line fix in the service layer, plus a new test verifying the error is wrapped with context. Follows the existing `"failed to <verb> <noun>: %w"` convention used throughout `transaction_service.go`.

**Tech Stack:** Go, `fmt.Errorf`, `errors.Is`, white-box service tests with hand-written mocks.

---

### Task 1: Add test for error wrapping on GetTransactionByID repo failure

**Files:**
- Test: `internal/service/transaction_service_test.go` (create or append)

The mock `mockTransactionRepo` already supports error injection via `getByIDErr` map (see `testhelper_test.go:304`). We need a test that calls `GetTransactionByID` when the repo returns an error, and verifies the returned error both wraps the original (via `errors.Is`) and contains a context message (via `strings.Contains`).

- [ ] **Step 1: Write the failing test**

```go
func TestGetTransactionByID_RepoError_WrapsWithContext(t *testing.T) {
	svc := newTestTransactionService()
	repoErr := fmt.Errorf("db connection lost")
	svc.txRepo.(*mockTransactionRepo).getByIDErr[99] = repoErr

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
```

Ensure the test file has these imports: `"context"`, `"errors"`, `"fmt"`, `"strings"`, `"testing"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestGetTransactionByID_RepoError_WrapsWithContext -v`

Expected: FAIL — the bare `return nil, err` on line 55 means `errors.Is` passes but `strings.Contains` fails (no context message yet).

### Task 2: Fix the bare error return

**Files:**
- Modify: `internal/service/transaction_service.go:55`

- [ ] **Step 1: Wrap the error**

Change line 55 from:

```go
		return nil, err
```

to:

```go
		return nil, fmt.Errorf("failed to retrieve transaction %d: %w", txID, err)
```

This matches the `"failed to <verb> <noun>: %w"` pattern used on lines 60, 77, 91, 101, and 107 of the same file.

- [ ] **Step 2: Run the new test to verify it passes**

Run: `go test ./internal/service/ -run TestGetTransactionByID_RepoError_WrapsWithContext -v`

Expected: PASS

- [ ] **Step 3: Run all tests to check for regressions**

Run: `go test ./...`

Expected: All tests pass. No caller uses bare equality (`err == someSpecificErr`) on this path — all use `errors.Is`, which unwraps.

- [ ] **Step 4: Commit**

```bash
git add internal/service/transaction_service.go internal/service/transaction_service_test.go
git commit -m "fix: wrap error consistently in GetTransactionByID (issue #25)"
```
