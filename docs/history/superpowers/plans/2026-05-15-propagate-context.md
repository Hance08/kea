# Propagate Context Through Startup Functions (Issue #68)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace all `context.Background()` calls in `cmd/root.go` startup functions with a propagated `context.Context` from the cobra command.

**Architecture:** Add a `context.Context` parameter to `initSysAcc`, `migrateLegacySysAcc`, `migrateLegacySysAccWith`, and the `accountMigrator` interface. The context originates from `cmd.Context()` in `Execute()` (cobra wires this to OS signals). Existing tests pass a `context.Background()` explicitly.

**Tech Stack:** Go, Cobra CLI framework

---

### Task 1: Update `migrateLegacySysAccWith` and `accountMigrator` to accept context

**Files:**
- Modify: `cmd/root.go:176-225` (interface + function)
- Modify: `cmd/root_test.go:28-78` (mock methods)
- Modify: `cmd/root_test.go:88-145` (test calls)

- [ ] **Step 1: Write a failing test that passes context through `migrateLegacySysAccWith`**

Update the first test case to pass `context.Background()` as the first argument:

```go
func TestMigrateLegacySysAccWith(t *testing.T) {
	t.Run("renames legacy account to currency-suffixed leaf segment", func(t *testing.T) {
		mock := newMockAccountMigrator()
		mock.add(model.LegacyOpeningBalancesName)

		err := migrateLegacySysAccWith(context.Background(), mock, usdConfig())
		require.NoError(t, err)

		require.Len(t, mock.renameCalls, 1)
		assert.Equal(t, model.LegacyOpeningBalancesName, mock.renameCalls[0].old)
		assert.Equal(t, "OpeningBalances_USD", mock.renameCalls[0].new)
		assert.Empty(t, mock.deleteCalls)
	})
```

This will fail because `migrateLegacySysAccWith` doesn't accept a context parameter yet.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestMigrateLegacySysAccWith -v`
Expected: compilation error — too many arguments to `migrateLegacySysAccWith`

- [ ] **Step 3: Update the `accountMigrator` interface to not need context (it's already ignored in mock)**

The `accountMigrator` interface methods already accept `context.Context`. No change needed there. Update `migrateLegacySysAccWith` signature and body to accept and thread context:

```go
func migrateLegacySysAccWith(ctx context.Context, acc accountMigrator, cfg *config.Config) error {
	fullTargetName := model.OpeningBalancesAccountName(cfg.Defaults.Currency)
	idx := strings.LastIndex(fullTargetName, ":")
	leafSegment := fullTargetName[idx+1:]

	_, err := acc.GetAccountByName(ctx, model.LegacyOpeningBalancesName)
	if errors.Is(err, service.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check legacy system account: %w", err)
	}

	_, err = acc.GetAccountByName(ctx, fullTargetName)
	if err == nil {
		if err := acc.DeleteAccountByName(ctx, model.LegacyOpeningBalancesName); err != nil {
			return fmt.Errorf(
				"legacy system account %q and target %q both exist; "+
					"remove or reconcile the legacy account manually: %w",
				model.LegacyOpeningBalancesName, fullTargetName, err,
			)
		}
		return nil
	}
	if !errors.Is(err, service.ErrNotFound) {
		return fmt.Errorf("failed to check target system account: %w", err)
	}

	if err := acc.RenameAccount(ctx, model.LegacyOpeningBalancesName, leafSegment); err != nil {
		return fmt.Errorf("failed to migrate legacy system account: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Update all remaining test calls to pass `context.Background()`**

Update each of the remaining 4 test cases in `TestMigrateLegacySysAccWith`:

```go
err := migrateLegacySysAccWith(context.Background(), mock, usdConfig())
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cmd/ -run TestMigrateLegacySysAccWith -v`
Expected: all 5 sub-tests PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go cmd/root_test.go
git commit -m "refactor: thread context through migrateLegacySysAccWith

Closes #68 (partial)"
```

### Task 2: Update `migrateLegacySysAcc` and `initSysAcc` to accept and propagate context

**Files:**
- Modify: `cmd/root.go:146-187` (`initSysAcc` and `migrateLegacySysAcc`)

- [ ] **Step 1: Update `migrateLegacySysAcc` to accept and forward context**

```go
func migrateLegacySysAcc(ctx context.Context, svc *service.Service, cfg *config.Config) error {
	return migrateLegacySysAccWith(ctx, svc.Account(), cfg)
}
```

- [ ] **Step 2: Update `initSysAcc` to accept and use context**

```go
func initSysAcc(ctx context.Context, svc *service.Service, cfg *config.Config) error {
	if err := migrateLegacySysAcc(ctx, svc, cfg); err != nil {
		return err
	}

	targetName := model.OpeningBalancesAccountName(cfg.Defaults.Currency)
	_, err := svc.Account().GetAccountByName(ctx, targetName)
	if err == nil {
		return nil
	}
	if !errors.Is(err, service.ErrNotFound) {
		return fmt.Errorf("failed to check system account: %w", err)
	}

	_, err = svc.Account().CreateAccount(
		ctx,
		model.CreateAccountInput{
			Name:        targetName,
			Type:        model.AccountTypeEquity,
			Currency:    cfg.Defaults.Currency,
			Description: "Opening Balances (System Account)",
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create system account: %w", err)
	}

	return nil
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./cmd/...`
Expected: compilation error — `initSysAcc` call site in `Execute()` passes wrong number of args

- [ ] **Step 4: Commit (will be combined with Task 3)**

Do not commit yet — the call site is broken. Proceed to Task 3.

### Task 3: Wire context from `Execute()` call site

**Files:**
- Modify: `cmd/root.go:125` (call site in `Execute`)

- [ ] **Step 1: Create a context with signal cancellation and pass it to `initSysAcc`**

In the `Execute` function, create a signal-aware context and pass it through. Update line 125:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Wire OS signal cancellation via cobra's built-in mechanism.
rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
	ctx = cmd.Context()
}
```

Wait — cobra doesn't provide a signal context by default. The simplest correct approach: create a signal-aware context at the top of `Execute()` and pass it to `initSysAcc`. Since `initSysAcc` runs before `rootCmd.Execute()`, we can't use `cmd.Context()` there. Instead, use `signal.NotifyContext`:

Replace the call at line 125 from:
```go
if err := initSysAcc(application.Service, cfg); err != nil {
```
to:
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()

if err := initSysAcc(ctx, application.Service, cfg); err != nil {
```

Add `"os/signal"` to the imports.

- [ ] **Step 2: Verify the full build compiles**

Run: `go build ./cmd/...`
Expected: SUCCESS (no errors)

- [ ] **Step 3: Run all tests**

Run: `go test ./cmd/ -v`
Expected: all tests PASS

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: all tests PASS

- [ ] **Step 5: Remove unused `"context"` import if present**

Check if `context.Background()` is still used anywhere in `cmd/root.go`. After all replacements, the `"context"` import is still needed for the `accountMigrator` interface method signatures. Keep it.

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go
git commit -m "refactor: propagate signal-aware context through initSysAcc

Replace context.Background() in initSysAcc and migrateLegacySysAcc with
a signal-aware context created via signal.NotifyContext in Execute().

Fixes #68"
```
