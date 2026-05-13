# Skip DB Init for Ledger Commands — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent `kea ledger list|add|switch|remove` from opening, backing up, migrating, or initializing the active ledger database.

**Architecture:** Add a helper `isLedgerCommand(args)` that inspects `os.Args` before the DB-init block runs. When it returns true, skip `app.NewApp`, `ensureCurrency`, and `initSysAcc` — register only the ledger subcommand, then execute and return. The existing "no active ledger" early-return path already proves this pattern works (lines 91-99); we extend it to also cover the "active ledger exists but we don't need it" case.

**Tech Stack:** Go, Cobra, testify

---

### Task 1: Add `isLedgerCommand` helper + tests

**Files:**
- Modify: `cmd/root.go` (add `isLedgerCommand` function)
- Modify: `cmd/root_test.go` (add tests)

- [ ] **Step 1: Write the failing tests**

Add to `cmd/root_test.go`:

```go
func TestIsLedgerCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"ledger list", []string{"ledger", "list"}, true},
		{"ledger ls alias", []string{"ledger", "ls"}, true},
		{"ledger add", []string{"ledger", "add", "work"}, true},
		{"ledger switch", []string{"ledger", "switch", "work"}, true},
		{"ledger remove", []string{"ledger", "remove", "old"}, true},
		{"bare ledger", []string{"ledger"}, true},
		{"account list", []string{"account", "list"}, false},
		{"add transaction", []string{"add"}, false},
		{"no args", []string{}, false},
		{"ledger with global flags", []string{"--no-color", "ledger", "list"}, true},
		{"ledger with config flag", []string{"-c", "/tmp/cfg.yaml", "ledger", "add", "x"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isLedgerCommand(tt.args))
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run TestIsLedgerCommand -v`
Expected: FAIL — `isLedgerCommand` is not defined.

- [ ] **Step 3: Implement `isLedgerCommand`**

Add to `cmd/root.go`:

```go
func isLedgerCommand(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a == "ledger"
	}
	return false
}
```

The logic: skip flag-like tokens (starting with `-`), then check whether the first positional argument is `"ledger"`. This works because Cobra's top-level positional arg is always the subcommand name, and global flags (`--no-color`, `-c <path>`) precede it.

Note: `-c` takes a value argument, but that value (a file path) won't start with `-` in normal usage and won't equal `"ledger"`, so the skip-flags heuristic still works. The only pathological case would be `-c ledger` (a config file literally named "ledger"), which is not realistic.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run TestIsLedgerCommand -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go cmd/root_test.go
git commit -m "feat: add isLedgerCommand helper to detect ledger subcommands"
```

---

### Task 2: Short-circuit DB init when running a ledger command

**Files:**
- Modify: `cmd/root.go:86-134` (restructure the `exitCode` closure)

- [ ] **Step 1: Write the failing test**

Add to `cmd/root_test.go`:

```go
func TestExecuteLedgerCommand_SkipsDBInit(t *testing.T) {
	appDir := t.TempDir()
	registry, err := internalled.Load(appDir)
	require.NoError(t, err)

	dbPath := filepath.Join(appDir, "test.db")
	require.NoError(t, os.WriteFile(dbPath, []byte{}, 0644))
	require.NoError(t, registry.Add("myledger", dbPath))
	require.NoError(t, registry.Switch("myledger"))

	// An active ledger exists, so without the fix, executeLedgerRootCmd
	// would try to open the DB. With the fix, it should skip DB init
	// and successfully run "ledger list".
	code := executeLedgerRootCmd(registry, nil, appDir, []string{"ledger", "list"})
	assert.Equal(t, 0, code)
}
```

This test passes `nil` for migrations, which would panic inside `app.NewApp` / `store.NewStore` if DB init were attempted. A successful exit proves DB init was skipped.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestExecuteLedgerCommand_SkipsDBInit -v`
Expected: FAIL — `executeLedgerRootCmd` is not defined.

- [ ] **Step 3: Extract and modify the init logic**

Refactor the inner closure of `Execute` into a testable function in `cmd/root.go`. Replace the `exitCode` closure (lines 86-134) with a call to a new `executeLedgerRootCmd` function, and add the early-return path:

```go
func executeLedgerRootCmd(registry *ledger.Registry, migrations fs.FS, appDir string, args []string) int {
	rootCmd := &cobra.Command{
		Use:           "kea",
		Short:         "kea is a CLI/TUI based personal accounting tool",
		Long:          `kea is a CLI/TUI based personal accounting tool`,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(ledgercmd.NewLedgerCmd(registry, migrations, appDir))

	if isLedgerCommand(args) {
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			pterm.Error.Println(capitalize(err.Error()))
			return 1
		}
		return 0
	}

	activePath, err := registry.Active()
	if err != nil {
		pterm.Warning.Println("No ledger configured. Run: kea ledger add <name>")
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			pterm.Error.Println(capitalize(err.Error()))
			return 1
		}
		return 0
	}

	cfg.Database.Path = activePath
	cfg.ActiveLedger = registry.ActiveName()

	application, cleanup, err := app.NewApp(cfg, migrations)
	if err != nil {
		pterm.Error.Println(err)
		return 1
	}
	defer cleanup()

	if err := ensureCurrency(cfg); err != nil {
		pterm.Error.Println(err)
		return 1
	}

	if err := initSysAcc(application.Service, cfg); err != nil {
		pterm.Error.Println(err)
		return 1
	}

	rootCmd.AddCommand(account.NewAccountCmd(application.Service))
	rootCmd.AddCommand(transaction.NewTransactionCmd(application.Service))
	rootCmd.AddCommand(NewAddCmd(application.Service))
	rootCmd.AddCommand(NewInfoCmd(application.Service))
	rootCmd.AddCommand(NewReportCmd(application.Service))
	rootCmd.AddCommand(NewReconcileCmd(application.Service))

	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		pterm.Error.Println(capitalize(err.Error()))
		return 1
	}
	return 0
}
```

The `Execute` function then becomes responsible only for config/registry init and calling this function. The key change: `cfg` must be passed as a parameter to `executeLedgerRootCmd`. The full refactored `Execute` function:

```go
func Execute(migrations fs.FS) {
	pterm.Error.Prefix = pterm.Prefix{
		Text:  " ERROR ",
		Style: pterm.NewStyle(pterm.BgLightRed, pterm.FgBlack),
	}

	args := os.Args[1:]

	// Parse global flags early for --no-color and --config.
	flagSet := pflag.NewFlagSet("kea-global", pflag.ContinueOnError)
	flagSet.ParseErrorsWhitelist.UnknownFlags = true
	cfgFlag := flagSet.StringP("config", "c", "", "set the config file path")
	noColor := flagSet.Bool("no-color", false, "disable colored output")
	_ = flagSet.Parse(args)

	configureOutput(*noColor)

	cfg, err := initConfig(*cfgFlag)
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	appDir, err := app.GetAppDataDir()
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	registry, err := ledger.Load(appDir)
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	if registry.MigratedLegacy {
		pterm.Info.Println(`Migrated existing database as ledger "default".`)
	}

	os.Exit(executeLedgerRootCmd(cfg, registry, migrations, appDir, args))
}
```

Wait — the current `Execute` uses a temporary `rootCmd` just to parse `--no-color` and `--config`. After the refactor, `executeLedgerRootCmd` creates its own `rootCmd`. We need to handle global flag parsing without the Cobra rootCmd. Use `pflag` directly (already a transitive dependency via Cobra) or parse `os.Args` manually.

Actually, the simpler approach: keep most of `Execute` as-is but add the short-circuit check right after the registry is loaded, before `registry.Active()` is called. This is a minimal change:

In `cmd/root.go`, inside the `exitCode` closure, insert the ledger-command short-circuit right after `rootCmd.AddCommand(ledgercmd.NewLedgerCmd(...))` and before `activePath, err := registry.Active()`:

```go
exitCode := func() int {
    rootCmd.AddCommand(ledgercmd.NewLedgerCmd(registry, migrations, appDir))

    // Ledger management commands do not need the active database.
    if isLedgerCommand(os.Args[1:]) {
        if err := rootCmd.Execute(); err != nil {
            pterm.Error.Println(capitalize(err.Error()))
            return 1
        }
        return 0
    }

    activePath, err := registry.Active()
    // ... rest unchanged ...
```

This is the minimal fix. For testability, extract just the guard into a testable function. But actually, the test in Step 1 can simply verify that passing `nil` migrations with a ledger command doesn't panic — we need a thin wrapper. Let's keep it simple:

Extract a helper `runRoot` that takes `rootCmd`, `registry`, `cfg`, `migrations`, `appDir` and performs the logic from line 86 onward, making the guard testable. But that's a big refactor. The simpler path: the `isLedgerCommand` unit tests (Task 1) validate detection. For integration coverage, add a test that constructs a Cobra command tree manually and asserts the short-circuit path.

Let me simplify. The actual code change is a 5-line insertion. The test needs a way to prove DB init didn't happen. The cleanest approach: test `isLedgerCommand` thoroughly (Task 1, done), then add the guard in production code and verify with a manual test or a narrowly-scoped integration test.

Here's the refined approach for Task 2:

- [ ] **Step 3 (revised): Add the short-circuit guard**

In `cmd/root.go`, in the `exitCode` closure, insert after the `rootCmd.AddCommand(ledgercmd.NewLedgerCmd(...))` line and before `activePath, err := registry.Active()`:

```go
if isLedgerCommand(os.Args[1:]) {
    if err := rootCmd.Execute(); err != nil {
        pterm.Error.Println(capitalize(err.Error()))
        return 1
    }
    return 0
}
```

- [ ] **Step 4: Run all existing tests**

Run: `go test ./cmd/... -v`
Expected: all PASS — no behavioral change for existing tests.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go
git commit -m "fix: skip DB init for ledger management commands (issue #48)"
```

---

### Task 3: Add integration-style test proving DB init is skipped

**Files:**
- Modify: `cmd/root_test.go`

The best way to prove the DB path is never opened is to set the active ledger to a nonexistent file path. Without the fix, `app.NewApp` would fail because the file doesn't exist. With the fix, ledger commands succeed.

- [ ] **Step 1: Write the test**

We can't easily test `Execute` (it calls `os.Exit`), but we can test the behavior indirectly by constructing the same Cobra tree that `Execute` builds and checking the short-circuit path. Add to `cmd/root_test.go`:

```go
func TestLedgerCommand_SkipsDBInit(t *testing.T) {
	appDir := t.TempDir()
	registry, err := internalled.Load(appDir)
	require.NoError(t, err)

	// Register a ledger pointing to a nonexistent DB file.
	// If DB init runs, it would fail or create the file.
	bogusPath := filepath.Join(appDir, "does-not-exist.db")
	require.NoError(t, registry.Add("phantom", bogusPath))
	require.NoError(t, registry.Switch("phantom"))

	// Build the same command tree that Execute builds.
	rootCmd := &cobra.Command{
		Use:           "kea",
		SilenceErrors: true,
	}
	rootCmd.AddCommand(ledgercmd.NewLedgerCmd(registry, nil, appDir))

	// Simulate: the guard detects "ledger list" and short-circuits.
	args := []string{"ledger", "list"}
	require.True(t, isLedgerCommand(args), "should detect ledger command")

	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	require.NoError(t, err)

	// The bogus DB file must not have been created or opened.
	_, statErr := os.Stat(bogusPath)
	assert.True(t, os.IsNotExist(statErr), "DB file should not exist — DB init must not have run")
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./cmd/ -run TestLedgerCommand_SkipsDBInit -v`
Expected: PASS

- [ ] **Step 3: Add import for `internalled` and other needed packages**

Ensure `cmd/root_test.go` imports:

```go
import (
    // existing imports...
    "os"
    "path/filepath"

    ledgercmd "github.com/hance08/kea/cmd/ledger"
    internalled "github.com/hance08/kea/internal/ledger"
    "github.com/spf13/cobra"
)
```

- [ ] **Step 4: Run all tests**

Run: `go test ./cmd/... -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/root_test.go
git commit -m "test: prove ledger commands skip DB initialization (issue #48)"
```

---

### Task 4: Final verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: all PASS

- [ ] **Step 2: Build and smoke-test**

Run: `make build`
Expected: builds without errors.

- [ ] **Step 3: Commit any remaining changes (if any)**
