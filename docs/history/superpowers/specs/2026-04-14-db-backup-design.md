# DB Backup Design

**Date:** 2026-04-14
**Status:** Approved

## Overview

Automatically back up the SQLite database file on each startup, before the database is opened. Backups are organised into three tiers — daily, weekly, monthly — with fixed retention limits and no user configuration required.

## Goals

- Protect against data loss from migrations, corruption, or accidental changes
- Zero user action required: backups happen silently on every run
- Never block startup: a backup failure warns but does not abort the app

## Non-Goals

- Manual `kea backup` command
- Configurable backup directory or retention counts
- Remote/cloud backup destinations
- Compression of backup files

## Trigger

`backup.Run(dbPath string)` is called inside `app.NewApp()` after the DB path is resolved but **before** `store.NewStore()` is called. This ensures the backup reflects the state of the DB before any migration or write occurs.

## Backup Location

Backups are stored in a `backups/` subdirectory next to the DB file.

```
~/.config/kea/
  kea.db
  backups/
    kea_daily_2026-04-14.db
    kea_weekly_2026-W15.db
    kea_monthly_2026-04.db
```

The directory is created if it does not exist.

## Tiers

| Tier    | Filename pattern            | Retention |
|---------|-----------------------------|-----------|
| Daily   | `kea_daily_YYYY-MM-DD.db`   | 7         |
| Weekly  | `kea_weekly_YYYY-WNN.db`    | 4         |
| Monthly | `kea_monthly_YYYY-MM.db`    | 12        |

Week numbers follow ISO 8601 (Go's `time.ISOWeek()`), zero-padded to two digits (e.g., `W05`).

## When a Backup Is Due

On each startup, `Run()` lists existing files in `backups/` and checks each tier independently:

- **Daily** — no file matching today's `YYYY-MM-DD` exists
- **Weekly** — no file matching this ISO week `YYYY-WNN` exists
- **Monthly** — no file matching this year-month `YYYY-MM` exists

Multiple tiers may trigger on the same run (e.g., the first run of a new month triggers all three).

## Copy Strategy

To avoid partial writes, the DB file is first copied to a temporary file (`<name>.tmp`) in the `backups/` directory, then atomically renamed to the final name. If the rename fails, the `.tmp` file is removed.

## Rotation

After a new backup is written, files for that tier are sorted lexicographically (which is chronological given the naming scheme). Files beyond the retention limit (oldest first) are deleted.

## Error Handling

| Condition | Behaviour |
|---|---|
| DB file does not exist (first run) | Return nil — no backup needed |
| `backups/` dir creation fails | Return error; `NewApp` logs warning with pterm but continues |
| File copy fails | Return error; `NewApp` logs warning but continues |
| Rotation deletion fails | Log warning, continue — stale extras are not fatal |

Backup errors must never abort startup.

## Package Structure

```
internal/backup/
  backup.go       — Run(), tier check, copy, rotation
  backup_test.go  — unit tests using t.TempDir()
```

### Public API

```go
// Run checks each backup tier and writes a new backup if due, then prunes
// old backups beyond the retention limit. It is a no-op if dbPath does not
// exist. Errors are non-fatal: the caller should log and continue.
func Run(dbPath string) error
```

### Clock Interface (internal)

```go
type clock interface {
    Now() time.Time
}
```

`realClock{}` wraps `time.Now()` for production. Tests inject `fakeClock{}` with a fixed time to make tier logic deterministic.

## Integration Point

In `internal/app/app.go`, inside `NewApp()`:

```go
// After dbPathRaw is resolved, before store.NewStore():
if err := backup.Run(dbPathRaw); err != nil {
    fmt.Fprintf(os.Stderr, "Warning: backup failed: %v\n", err)
}
```

## Testing

All tests use `t.TempDir()` — no real filesystem side effects.

| Test case | What it verifies |
|---|---|
| DB does not exist | No backup created, no error |
| First run, no backups yet | All three tiers copied |
| Each tier already current | No new file written (no-op) |
| Each tier due individually | Correct file created, others untouched |
| All three tiers due together | All three files created |
| Rotation: daily exceeds 7 | Oldest daily deleted |
| Rotation: weekly exceeds 4 | Oldest weekly deleted |
| Rotation: monthly exceeds 12 | Oldest monthly deleted |
| Copy failure (read-only dir) | Error returned, no crash |
