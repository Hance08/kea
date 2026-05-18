# Non-Interactive Currency Fallback

**Date:** 2026-05-19
**Issue:** #118 — `ensureCurrency` launches interactive TUI prompt that blocks non-interactive contexts

## Problem

`cmd/root.go` calls `ensureCurrency()` on every non-ledger startup. When no default currency is configured, it falls through to `initWizard()` which launches an interactive `huh` TUI prompt. This blocks indefinitely in non-interactive contexts (future `kea serve`, piped input, cron jobs, Docker containers).

## Solution

Add a `isInteractive()` TTY detection helper using Go 1.24+ `os.IsTerminal()`. Modify `ensureCurrency` to branch:

- **Interactive (TTY attached):** Current behavior — call `initWizard` for TUI prompt.
- **Non-interactive (no TTY):** Fall back to `"USD"`, persist to config, print a warning via `pterm.Warning`.

## Changes

### `cmd/root.go`

**New function — `isInteractive()`:**

```go
func isInteractive() bool {
    return os.IsTerminal(os.Stdin.Fd())
}
```

**Modified function — `ensureCurrency(cfg)`:**

When `cfg.Defaults.Currency` is empty:
1. If `isInteractive()` returns true, call `initWizard(cfg)` (existing behavior).
2. Otherwise, set currency to `"USD"`, write via `viper.Set` + `viper.WriteConfig`, and print a `pterm.Warning` message: `"No default currency configured; defaulting to USD."`.

### No other files change

- `initWizard` is unchanged.
- `initSysAcc` is already non-interactive.
- No new flags or command structure changes.
- No new dependencies (`os.IsTerminal` is stdlib since Go 1.24).

## Testing

- Test the non-interactive fallback path: when currency is empty and `isInteractive` returns false, verify USD is set and config is written.
- Test that the interactive path still delegates to `initWizard`.
- `isInteractive()` itself is a thin stdlib wrapper — not unit tested directly.

## Future Impact

When `kea serve` is implemented, it will automatically skip the TUI wizard because server processes typically don't have a TTY attached. No additional work needed in the serve command.
