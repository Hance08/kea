# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Language

All code, comments, test names, commit messages, and documentation in this repository must be written in English.

## Commands

```bash
# Build
make build          # produces ./kea_test binary
make run            # go run ./cmd/kea

# Test
go test ./...                                      # all packages
go test ./internal/service/...                     # one package
go test ./internal/service/ -run TestCreateAccount # single test
go test ./internal/service/ -v -run TestDetermineType

# Dependencies
go mod tidy
```

## Architecture

KEA is a CLI/TUI personal double-entry accounting tool. The layers are:

```
cmd/              Cobra commands → call service methods
internal/app/     Wires service + store (entry point for dependency injection)
internal/service/ Business logic (AccountService, TransactionService, report)
internal/store/   SQLite implementation of repository interfaces
internal/model/   Domain types only (no business logic)
internal/repository/interfaces.go  Contracts between service and store
internal/utils/   Pure helpers (amount formatting/parsing)
ui/               charmbracelet/huh prompts and pterm views
migrations/       golang-migrate SQL files embedded via FS
```

**Service facade:** `service.Service` embeds `*AccountService` and `*TransactionService`. Access via `svc.Account()` and `svc.Transaction()`. Report methods live directly on `TransactionService`.

**Repository interfaces** (`internal/repository/interfaces.go`):
- `AccountRepository` — account CRUD and balance queries
- `TransactionRepository` — transaction/split CRUD and bulk date-range queries for reports
- `Repository` — combines both
- `TransactionManager` — `ExecTx(ctx, fn(Repository) error)` for atomic multi-step operations

**Store** (`internal/store/`): `SQLite` implements `Repository` + `TransactionManager`. The `DBTX` interface abstracts `*sql.DB` and `*sql.Tx` so queries work in both contexts.

## Key Domain Rules

**Amount storage:** always cents as `int64`. Use `utils.FormatAmount` / `utils.ParseAmount` for display and input. `FormatAmount` trims trailing zeros (e.g., 100 cents → `"1"`, not `"1.00"`).

**Double-entry:** every transaction's splits must sum to zero. Enforced by `ValidateSplitsBalance`.

**Account types:** `A` (Asset), `L` (Liability), `C` (Equity), `R` (Revenue), `E` (Expense). Only leaf accounts may hold transactions.

**Opening balance split direction:**
- Asset account: asset split = +amount, equity split = -amount
- Liability account: liability split = -amount, equity split = +amount

**Protected records:** Transaction ID 1 (`OpeningBalanceTransactionID`) and reconciled transactions are immutable. Operations on them return `ErrNotEditable` or `ErrReconciled` (both wrapped with `%w` — check with `errors.Is`).

**System account:** `"Equity:OpeningBalances"` must not be deleted.

## Testing

Service layer tests use white-box testing (`package service`) with hand-written mocks in `internal/service/testhelper_test.go`:
- `mockAccountRepo`, `mockTransactionRepo`, `mockCombinedRepo`, `mockTransactionManager`
- Injectable error maps and call-recorder slices (e.g. `deleteSplitCalls []int64`)
- Factory helpers: `newTestAccountService()`, `newTestTransactionService()`, `defaultConfig()`

No real database is needed for service tests — all storage is in-memory maps within the mocks.
