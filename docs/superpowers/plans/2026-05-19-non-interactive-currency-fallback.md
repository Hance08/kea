# Non-Interactive Currency Fallback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent `ensureCurrency` from launching an interactive TUI prompt when stdin is not a terminal — fall back to USD instead.

**Architecture:** Add an `isInteractive()` helper using Go 1.24+ `os.IsTerminal()`. Extract the config-writing logic from `initWizard` into a shared `setCurrency` helper. Modify `ensureCurrency` to branch: interactive → TUI wizard, non-interactive → USD fallback with warning.

**Tech Stack:** Go stdlib (`os.IsTerminal`), viper, pterm, testify

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `cmd/root.go` | Modify | Add `isInteractive()`, `setCurrency()`, update `ensureCurrency` |
| `cmd/root_test.go` | Modify | Add tests for `ensureCurrency` branching logic |

---

### Task 1: Add `isInteractive` helper and `setCurrency` extractor

**Files:**
- Modify: `cmd/root.go:231-237` (ensureCurrency), `cmd/root.go:287-303` (initWizard)

- [ ] **Step 1: Write failing tests for the non-interactive fallback path**

Add to `cmd/root_test.go`:

```go
func TestEnsureCurrency_AlreadySet(t *testing.T) {
	cfg := &config.Config{Defaults: config.DefaultsConfig{Currency: "TWD"}}
	err := ensureCurrency(cfg)
	require.NoError(t, err)
	assert.Equal(t, "TWD", cfg.Defaults.Currency)
}

func TestSetCurrency(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("defaults:\n  currency: \"\"\n"), 0o644))

	viper.Reset()
	viper.SetConfigFile(cfgPath)
	require.NoError(t, viper.ReadInConfig())

	cfg := &config.Config{}
	err := setCurrency(cfg, "EUR")
	require.NoError(t, err)

	assert.Equal(t, "EUR", cfg.Defaults.Currency)
	assert.Equal(t, "EUR", viper.GetString("defaults.currency"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run "TestEnsureCurrency_AlreadySet|TestSetCurrency" -v`
Expected: FAIL — `setCurrency` undefined

- [ ] **Step 3: Add `isInteractive` and `setCurrency` to `cmd/root.go`**

Add the `isInteractive` function after `ensureCurrency` (around line 237):

```go
func isInteractive() bool {
	return os.IsTerminal(os.Stdin.Fd())
}
```

Extract config-writing logic from `initWizard` into `setCurrency`:

```go
func setCurrency(cfg *config.Config, currency string) error {
	cfg.Defaults.Currency = currency
	viper.Set("defaults.currency", currency)

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save config to file: %w", err)
	}

	return nil
}
```

Update `initWizard` to use `setCurrency`:

```go
func initWizard(cfg *config.Config) error {
	currency, err := prompts.PromptInitCurrency("USD")
	if err != nil {
		return err
	}

	if err := setCurrency(cfg, currency); err != nil {
		return err
	}

	pterm.Success.Printf("Configuration saved. Default currency set to: %s\n", currency)

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run "TestEnsureCurrency_AlreadySet|TestSetCurrency" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go cmd/root_test.go
git commit -m "refactor: extract setCurrency helper and add isInteractive detection"
```

---

### Task 2: Update `ensureCurrency` to branch on TTY

**Files:**
- Modify: `cmd/root.go:231-237` (ensureCurrency)
- Modify: `cmd/root_test.go`

- [ ] **Step 1: Write failing test for non-interactive fallback**

Add to `cmd/root_test.go`:

```go
func TestEnsureCurrency_NonInteractiveFallback(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("defaults:\n  currency: \"\"\n"), 0o644))

	viper.Reset()
	viper.SetConfigFile(cfgPath)
	require.NoError(t, viper.ReadInConfig())

	cfg := &config.Config{}
	err := ensureCurrencyWith(cfg, false)
	require.NoError(t, err)
	assert.Equal(t, "USD", cfg.Defaults.Currency)
	assert.Equal(t, "USD", viper.GetString("defaults.currency"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestEnsureCurrency_NonInteractiveFallback -v`
Expected: FAIL — `ensureCurrencyWith` undefined

- [ ] **Step 3: Implement `ensureCurrencyWith` and update `ensureCurrency`**

Replace the existing `ensureCurrency` in `cmd/root.go`:

```go
func ensureCurrency(cfg *config.Config) error {
	return ensureCurrencyWith(cfg, isInteractive())
}

func ensureCurrencyWith(cfg *config.Config, interactive bool) error {
	if cfg.Defaults.Currency != "" {
		return nil
	}

	if interactive {
		return initWizard(cfg)
	}

	if err := setCurrency(cfg, "USD"); err != nil {
		return err
	}

	pterm.Warning.Println("No default currency configured; defaulting to USD.")

	return nil
}
```

- [ ] **Step 4: Run all tests to verify they pass**

Run: `go test ./cmd/ -run "TestEnsureCurrency|TestSetCurrency" -v`
Expected: All PASS

- [ ] **Step 5: Run full test suite to check for regressions**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go cmd/root_test.go
git commit -m "fix: fallback to USD when ensureCurrency runs without a TTY (#118)"
```

---

### Task 3: Build and verify

**Files:** None (verification only)

- [ ] **Step 1: Build the binary**

Run: `make build`
Expected: Succeeds, produces `./kea_test` binary

- [ ] **Step 2: Run full test suite one final time**

Run: `go test ./... -count=1`
Expected: All PASS

- [ ] **Step 3: Commit if any cleanup needed, otherwise done**
