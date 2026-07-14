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

## Docker Development

An alternative to installing Go/Node/CGO toolchains locally: a Docker Compose setup provisions an `app` (Go) and `spa` (Node) dev container, bind-mounting the repo so edits on the host are picked up immediately.

```bash
docker compose up -d                              # start both containers
docker compose exec app go test ./...             # run Go tests inside the container
docker compose exec app go run ./cmd/kea <args>    # run the CLI/TUI
docker compose exec app go run ./cmd/kea serve     # run the web API (reachable at localhost:8080)
docker compose exec spa npm run dev -- --host 0.0.0.0  # run the SPA dev server (reachable at localhost:5173)
docker compose down                                # stop both containers
```

Known limitation: the `app` container runs as root, which causes `TestSwap_FailedSwapKeepsOldConnection` (`internal/store`) to fail under `go test ./...` inside the container — it's a pre-existing test that assumes an unprivileged user, not a bug in the Docker setup.

## Architecture

KEA is a CLI/TUI personal double-entry accounting tool. The layers are:

```
cmd/              Cobra commands → call service methods
  cmd/ledger/     Ledger management subcommands (add/list/switch/remove)
internal/app/     Wires service + store (entry point for dependency injection)
internal/service/ Business logic (AccountService, TransactionService, report, reconcile)
internal/store/   SQLite implementation of repository interfaces (sqlite*.go)
internal/model/   Domain types only (no business logic)
internal/repository/interfaces.go  Contracts between service and store
internal/config/  Config struct + defaults (loaded by cmd/root.go via viper)
internal/ledger/  Ledger registry (multiple named DBs, active selection)
internal/backup/  Pre-startup DB backup
internal/utils/   Pure helpers (amount formatting/parsing)
ui/               charmbracelet/huh prompts and pterm views
migrations/       golang-migrate SQL files embedded via FS
```

**Service facade:** `service.Service` holds unexported `*AccountService` and `*TransactionService` fields plus a `*config.Config`. Access via `svc.Account()`, `svc.Transaction()`, `svc.Config()`. Report and reconcile methods live directly on `TransactionService`.

**Repository interfaces** (`internal/repository/interfaces.go`): every method takes `context.Context` as its first argument.
- `AccountRepository` — account CRUD and balance queries
- `TransactionRepository` — transaction/split CRUD, bulk date-range queries for reports, and reconcile-state operations (`GetUnreconciledTransactionsByAccount`, `MarkSplitsReconciledByAccount`, `BulkUpdateTransactionStatus`, `GetLastReconciledBalance`, `SetLastReconciledBalance`)
- `Repository` — combines both
- `TransactionManager` — `ExecTx(ctx, fn(Repository) error)` for atomic multi-step operations

**Store** (`internal/store/`): the `Store` struct implements `Repository` + `TransactionManager`. The `DBTX` interface uses the `*Context` method set (`ExecContext`, `QueryContext`, `QueryRowContext`, `PrepareContext`) so the same queries work over both `*sql.DB` and `*sql.Tx`. All store methods thread `context.Context`; do not call non-context `database/sql` methods.

## Key Domain Rules

**Amount storage:** always cents as `int64`. Use `utils.FormatAmount` / `utils.ParseAmount` for display and input. `FormatAmount` trims trailing zeros (e.g., 100 cents → `"1"`, not `"1.00"`).

**Double-entry:** every transaction's splits must sum to zero. Enforced by `ValidateSplitsBalance`.

**Account types:** `A` (Asset), `L` (Liability), `C` (Equity), `R` (Revenue), `E` (Expense). Only leaf accounts may hold transactions.

**Opening balance split direction:**
- Asset account: asset split = +amount, equity split = -amount
- Liability account: liability split = -amount, equity split = +amount

**Protected records:** Reconciled transactions are immutable. Operations on them return `ErrReconciled` (wrapped with `%w` — check with `errors.Is`).

**System account:** per-currency, named `Equity:OpeningBalances_<CCY>` (e.g. `Equity:OpeningBalances_USD`). Use `model.OpeningBalancesAccountName(currency)` to build the name and `model.IsOpeningBalancesAccount(name)` to detect one. The legacy single name `Equity:OpeningBalances` is auto-renamed at startup by `migrateLegacySysAcc` (`cmd/root.go`). System accounts must not be deleted.

## Testing

Service layer tests use white-box testing (`package service`) with hand-written mocks in `internal/service/testhelper_test.go`:
- `mockAccountRepo`, `mockTransactionRepo`, `mockCombinedRepo`, `mockTransactionManager`
- Injectable error maps and call-recorder slices (e.g. `deleteSplitCalls []int64`)
- Factory helpers: `newTestAccountService()`, `newTestTransactionService()`, `defaultConfig()`

No real database is needed for service tests — all storage is in-memory maps within the mocks.
