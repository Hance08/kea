# Multi-Ledger Support Design

**Date:** 2026-04-15
**Status:** Approved

## Overview

Allow users to maintain multiple independent ledgers (databases) within a single KEA installation. Each ledger has its own SQLite database, account tree, and transaction history. A persistent "active ledger" concept means users run `kea ledger switch <name>` once and all subsequent commands operate on that ledger.

## File Layout

```
$APPDATADIR/kea/
├── config.yaml          # unchanged — per-run defaults (currency, etc.)
├── ledgers.yaml         # NEW — global registry + active ledger pointer
├── kea.db               # existing default DB (auto-registered on first run)
└── ledgers/
    ├── personal.db      # auto-created by `kea ledger add personal`
    └── business.db
```

`$APPDATADIR` is resolved by the existing `app.GetAppDataDir()` function (`~/.config/kea/` on macOS/Linux, `%AppData%\kea\` on Windows).

## `ledgers.yaml` Format

```yaml
active: default
ledgers:
  default:
    path: ~/.config/kea/kea.db
  personal:
    path: ~/.config/kea/ledgers/personal.db
  business:
    path: ~/.config/kea/ledgers/business.db
  offsite:
    path: /Volumes/NAS/accounting/offsite.db
```

## New Package: `internal/ledger/`

A new `internal/ledger/` package owns all registry logic. It exposes a `Registry` type with the following methods:

| Method | Description |
|---|---|
| `Load(path string) (*Registry, error)` | Read `ledgers.yaml` from disk |
| `Save() error` | Write current state back to disk |
| `Add(name, dbPath string) error` | Register a new ledger; error if name exists |
| `Remove(name string, deleteFile bool) error` | Unregister; optionally delete `.db` file |
| `Switch(name string) error` | Set `active`; error if name unknown |
| `Active() (string, error)` | Return resolved DB path for active ledger; respects `KEA_LEDGER` env var override |

The rest of the app (`cmd/root.go`, `app.NewApp`) calls `registry.Active()` to get the DB path. Nothing changes in `internal/service/`, `internal/store/`, or `internal/repository/`.

## CLI Commands

A new `kea ledger` subcommand group is added under `cmd/ledger/`.

### `kea ledger list`

Prints all registered ledgers. Active ledger is marked with `*`.

```
  NAME       PATH
* default    ~/.config/kea/kea.db
  personal   ~/.config/kea/ledgers/personal.db
  business   ~/.config/kea/ledgers/business.db
```

### `kea ledger add <name> [--path <file>]`

- Without `--path`: creates `$APPDATADIR/kea/ledgers/<name>.db`, runs migrations, registers in `ledgers.yaml`. Does not switch the active ledger.
- With `--path <file>`: registers the given path. The directory must already exist. The file is created if absent; if it already exists, migrations are run (they are idempotent). Does not switch the active ledger.
- Errors if name already exists.

### `kea ledger switch <name>`

- Updates `active:` in `ledgers.yaml`.
- Prints: `Switched to ledger "personal".`
- Errors if name not found.

### `kea ledger remove <name> [--delete-file]`

- Refuses if `<name>` is the currently active ledger (user must switch away first).
- Without `--delete-file`: unregisters only; prints the file path so the user knows the data is still on disk.
- With `--delete-file`: prompts `Delete <path>? [y/N]` before removing the file.

### `kea info` (extended)

The existing `kea info` command is extended to show the active ledger name alongside the DB path.

## Boot / Initialization Flow

### Normal startup (`ledgers.yaml` exists)

1. `cmd/root.go` calls `ledger.Load(registryPath)`
2. Gets active DB path via `registry.Active()`
3. Passes path into `app.NewApp()` — no other changes downstream

### First run / migration (`ledgers.yaml` absent, `kea.db` exists)

1. `ledger.Load()` detects no `ledgers.yaml`
2. Checks if `$APPDATADIR/kea/kea.db` exists
3. If yes: creates `ledgers.yaml` with `default` → `kea.db`, sets `active: default`. Prints: `Migrated existing database as ledger "default".`
4. If no: creates `ledgers.yaml` with no entries and `active: ""`. Root command prints onboarding message and exits.

### Fresh install (nothing exists)

- `ledger.Load()` returns an empty registry with no active ledger
- All non-`ledger` commands exit with: `No ledger configured. Run: kea ledger add <name>`

### Active ledger resolution priority

1. `KEA_LEDGER` env var (overrides everything; useful for scripts/CI)
2. `active:` field in `ledgers.yaml`
3. Error — no ledger configured

## Error Handling

| Situation | Behaviour |
|---|---|
| `kea ledger add` — name already exists | `ledger "personal" already exists` |
| `kea ledger add` — `--path` directory doesn't exist | `directory does not exist: /some/path` |
| `kea ledger switch` — name not found | `unknown ledger "foo" — run: kea ledger list` |
| `kea ledger remove` — target is active ledger | `cannot remove active ledger — switch to another ledger first` |
| `ledgers.yaml` active name not in registry | `active ledger "foo" is not registered — run: kea ledger list` |
| Registered DB file missing from disk | Warn on startup: `warning: ledger "personal" database not found at <path>` — fatal only if it is the active ledger |
| `KEA_LEDGER` names an unregistered ledger | `KEA_LEDGER "foo" is not a registered ledger` |

## Testing

### `internal/ledger/` — unit tests (no DB, no filesystem except temp dirs)

- `Registry.Add` — happy path, duplicate name error
- `Registry.Remove` — unregister only; with `--delete-file`; refuse when target is active
- `Registry.Switch` — happy path, unknown name error
- `Registry.Active` — correct path returned; `KEA_LEDGER` env var override; empty registry error
- `ledger.Load` — migration from existing `kea.db`; fresh install; normal load

### `cmd/ledger/` — integration tests (temp dir as APPDATADIR)

- `kea ledger list` output format
- `kea ledger add` creates the `.db` file and runs migrations
- `kea ledger switch` persists across invocations (reads `ledgers.yaml` back)
- `kea ledger remove` with and without `--delete-file`

### Existing tests

No changes required. `internal/service/` and `internal/store/` layers only receive a resolved DB path and are unaware of `ledgers.yaml`.

## Interaction with Backup

`backup.Run(dbPath)` is called in `app.NewApp()` using the resolved DB path. Since ledger resolution happens in `cmd/root.go` before `NewApp` is called, each ledger gets its own correctly-scoped backup with no changes to the backup package.
