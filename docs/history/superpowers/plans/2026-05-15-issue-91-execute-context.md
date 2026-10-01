# Issue #91: Use ExecuteContext to Propagate Signal Context Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `rootCmd.Execute()` with `rootCmd.ExecuteContext(ctx)` so subcommands receive the signal-aware context instead of `context.Background()`.

**Architecture:** Three `rootCmd.Execute()` call sites in `cmd/root.go` need updating. The main call at line 141 already has the signal-aware `ctx` in scope. The two early-exit calls (lines 92, 103) run before `signal.NotifyContext` — they should use `context.Background()` via `ExecuteContext` for consistency. A test verifies context propagation to subcommands.

**Tech Stack:** Go, Cobra, `os/signal`

---

### Task 1: Replace `rootCmd.Execute()` with `rootCmd.ExecuteContext(ctx)` at line 141

**Files:**
- Modify: `cmd/root.go:141`

- [ ] **Step 1: Change the main Execute call to ExecuteContext**

At line 141 of `cmd/root.go`, replace:

```go
if err := rootCmd.Execute(); err != nil {
```

with:

```go
if err := rootCmd.ExecuteContext(ctx); err != nil {
```

This is the critical fix. The variable `ctx` (created at line 126 via `signal.NotifyContext`) is already in scope. After this change, all subcommands calling `cmd.Context()` will receive the signal-aware context.

- [ ] **Step 2: Verify build**

Run: `make build`
Expected: Build succeeds with no errors.

- [ ] **Step 3: Run tests**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "fix: use ExecuteContext(ctx) for signal-aware context propagation (issue #91)

Replace rootCmd.Execute() with rootCmd.ExecuteContext(ctx) at the main
command execution site so subcommands receive the signal-aware context
created via signal.NotifyContext instead of context.Background().

Closes #91"
```

### Task 2: Update early-exit Execute calls for consistency

**Files:**
- Modify: `cmd/root.go:92,103`

- [ ] **Step 1: Update the ledger-command early-exit call at line 92**

At line 92 of `cmd/root.go`, replace:

```go
if err := rootCmd.Execute(); err != nil {
```

with:

```go
if err := rootCmd.ExecuteContext(context.Background()); err != nil {
```

These early-exit paths run before `signal.NotifyContext` is created (no DB connection, no app). Using `ExecuteContext(context.Background())` makes the codebase consistent — every call site uses `ExecuteContext` — while being functionally equivalent to the old `Execute()` call.

- [ ] **Step 2: Update the no-active-ledger early-exit call at line 103**

At line 103 of `cmd/root.go`, replace:

```go
if err := rootCmd.Execute(); err != nil {
```

with:

```go
if err := rootCmd.ExecuteContext(context.Background()); err != nil {
```

- [ ] **Step 3: Verify build**

Run: `make build`
Expected: Build succeeds with no errors.

- [ ] **Step 4: Run tests**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go
git commit -m "refactor: use ExecuteContext(context.Background()) for early-exit paths

Update the two early-exit rootCmd.Execute() calls (ledger-command and
no-active-ledger paths) to use ExecuteContext for consistency. These
paths run before signal.NotifyContext is created, so context.Background()
is the correct context to pass."
```

### Task 3: Add test verifying context propagation

**Files:**
- Create: `cmd/root_context_test.go`

- [ ] **Step 1: Write the test**

Create `cmd/root_context_test.go`:

```go
package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
)

func TestExecuteContextPropagatesContext(t *testing.T) {
	type ctxKey string
	key := ctxKey("test-signal")

	ctx := context.WithValue(context.Background(), key, "present")

	var receivedCtx context.Context
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{
		Use: "child",
		RunE: func(cmd *cobra.Command, args []string) error {
			receivedCtx = cmd.Context()
			return nil
		},
	}
	root.AddCommand(child)
	root.SetArgs([]string{"child"})

	if err := root.ExecuteContext(ctx); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	if receivedCtx == nil {
		t.Fatal("child command did not receive a context")
	}
	val, ok := receivedCtx.Value(key).(string)
	if !ok || val != "present" {
		t.Errorf("child context missing test value: got %q, want %q", val, "present")
	}
}
```

This test validates the Cobra mechanism we rely on: that `ExecuteContext` makes the provided context available to subcommands via `cmd.Context()`. It uses a context value as a marker to prove the exact context instance is propagated.

- [ ] **Step 2: Run the test to verify it passes**

Run: `go test ./cmd/ -run TestExecuteContextPropagatesContext -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/root_context_test.go
git commit -m "test: verify ExecuteContext propagates context to subcommands"
```
