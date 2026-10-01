# Reconcile Non-Interactive Flag Validation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Treat any of `--balance`, `--ids`, or `--force` as a request for non-interactive mode and reject incomplete flag combinations with a clear error, instead of silently falling back to interactive mode.

**Architecture:** Extract a `resolveMode` helper that checks which flags were explicitly set and returns either interactive mode or non-interactive mode (or an error for incomplete combinations). The `Run` method delegates to this helper. Tests exercise `resolveMode` directly — no cobra command wiring needed.

**Tech Stack:** Go, cobra, testify

---

### Task 1: Add `resolveMode` helper with tests for incomplete flag combinations

**Files:**
- Modify: `cmd/reconcile_actions.go:22-37` (replace inline detection with `resolveMode` call)
- Create: `cmd/reconcile_actions_test.go`

The key insight: `cmd.Flags().Changed()` tells us if the user explicitly set a flag. We collect those booleans and pass them to a pure function that decides the mode. This makes the logic testable without cobra wiring.

- [ ] **Step 1: Write the failing tests**

Create `cmd/reconcile_actions_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveMode(t *testing.T) {
	t.Run("no flags means interactive", func(t *testing.T) {
		mode, err := resolveMode(false, false, false)
		require.NoError(t, err)
		assert.False(t, mode)
	})

	t.Run("balance and ids means non-interactive", func(t *testing.T) {
		mode, err := resolveMode(true, true, false)
		require.NoError(t, err)
		assert.True(t, mode)
	})

	t.Run("balance and ids and force means non-interactive", func(t *testing.T) {
		mode, err := resolveMode(true, true, true)
		require.NoError(t, err)
		assert.True(t, mode)
	})

	t.Run("balance only is an error", func(t *testing.T) {
		_, err := resolveMode(true, false, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--ids")
	})

	t.Run("ids only is an error", func(t *testing.T) {
		_, err := resolveMode(false, true, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--balance")
	})

	t.Run("force only is an error", func(t *testing.T) {
		_, err := resolveMode(false, false, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--balance")
		assert.Contains(t, err.Error(), "--ids")
	})

	t.Run("force with balance only is an error", func(t *testing.T) {
		_, err := resolveMode(true, false, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--ids")
	})

	t.Run("force with ids only is an error", func(t *testing.T) {
		_, err := resolveMode(false, true, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--balance")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run TestResolveMode -v`
Expected: compilation error — `resolveMode` is not defined.

- [ ] **Step 3: Implement `resolveMode` and wire it into `Run`**

Add this function to `cmd/reconcile_actions.go` (above or below the helpers section):

```go
// resolveMode decides whether to run in non-interactive mode.
// Returns (true, nil) for non-interactive, (false, nil) for interactive,
// or an error if flags are partially set.
func resolveMode(balanceSet, idsSet, forceSet bool) (bool, error) {
	if !balanceSet && !idsSet && !forceSet {
		return false, nil
	}

	var missing []string
	if !balanceSet {
		missing = append(missing, "--balance")
	}
	if !idsSet {
		missing = append(missing, "--ids")
	}
	if len(missing) > 0 {
		return false, fmt.Errorf(
			"non-interactive mode requires both --balance and --ids; missing: %s",
			strings.Join(missing, ", "),
		)
	}
	return true, nil
}
```

Then replace lines 32-37 in the `Run` method:

```go
func (r *reconcileRunner) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	accountName := args[0]

	// Resolve account.
	acc, err := r.accSvc.GetAccountByName(ctx, accountName)
	if err != nil {
		return fmt.Errorf("account %q not found: %w", accountName, err)
	}

	nonInteractive, err := resolveMode(
		cmd.Flags().Changed("balance"),
		cmd.Flags().Changed("ids"),
		cmd.Flags().Changed("force"),
	)
	if err != nil {
		return err
	}

	if nonInteractive {
		return r.runNonInteractive(ctx, acc)
	}
	return r.runInteractive(ctx, acc)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run TestResolveMode -v`
Expected: all 8 tests PASS.

- [ ] **Step 5: Run full test suite to check for regressions**

Run: `go test ./...`
Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/reconcile_actions.go cmd/reconcile_actions_test.go
git commit -m "fix: reject partial reconcile flags instead of falling back to interactive mode (closes #45)"
```
