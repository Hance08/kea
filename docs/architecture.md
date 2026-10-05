# Architecture

> **Read this when:** you need the big picture — which layer owns what, how a CLI command or HTTP request reaches the database, or how the binary starts.
>
> **Related:** [domain.md](domain.md) · [http-api.md](http-api.md) · [recipes/](recipes/)

## Layers

```
   CLI / TUI                                  HTTP
 +-------------------------------+    +-------------------------+
 | cmd/, cmd/account,            |    | internal/api            |
 | cmd/transaction, cmd/ledger   |    | (chi router, handlers,  |
 |   uses ui/prompts (huh)       |    |  embedded SPA via       |
 |        ui/views   (pterm)     |    |  internal/web)          |
 |      ui/reconcile (bubbletea) |    |                         |
 +---------------+---------------+    +------------+------------+
                 |                                 |
                 v                                 v
        +----------------------------------------------------+
        | internal/service   (business rules, validation)    |
        +--------------------------+-------------------------+
                                   | depends on interfaces only
                                   v
        +----------------------------------------------------+
        | internal/repository (AccountRepository,            |
        |   TransactionRepository, TransactionManager)       |
        +--------------------------+-------------------------+
                                   ^ implemented by
        +--------------------------+-------------------------+
        | internal/store  (SQLite, golang-migrate)           |
        +----------------------------------------------------+

 internal/app      wires config + ledger registry + backup + store + service
 internal/model    plain domain types, imported by every layer above
 migrations/       *.sql embedded as an fs.FS; cmd/kea passes it down to
                   app.NewApp -> store.NewStore (store does not import it)
```

Dependencies point downward only. `internal/service` sees storage through the interfaces in `internal/repository/interfaces.go` and never imports `internal/store`; only `internal/app` (the composition root) imports both and connects them. `internal/model` imports no other kea package, so it can be shared everywhere without cycles. Presentation code (`cmd/`, `ui/`, `internal/api`) talks to `service.Service`, not to the store.

## Package map

"Must not import" states the rule the current import graph already follows (checked with `go list`).

| Package | Owns | Must not import |
| --- | --- | --- |
| `cmd` | Root command and startup (`cmd/root.go`), top-level commands `add`, `info`, `report`, `reconcile`, `serve`; config file writing (`cmd/save_config.go`) | `internal/store`, `internal/repository` |
| `cmd/kea` | `main`: calls `cmd.Execute(migrations.FS)` | anything except `cmd` and `migrations` |
| `cmd/account` | `kea account` subcommands (create, edit, delete, list, search) | `internal/app`, `internal/store`, `internal/api` |
| `cmd/budget` | `kea budget` subcommands (set, stop, list, delete, report) | `internal/app`, `internal/store`, `internal/api` |
| `cmd/ledger` | `kea ledger` subcommands (add, list, switch, remove); runs without an open DB | `internal/service`, `internal/store` (it reaches the store only via `app.InitLedgerDB`) |
| `cmd/transaction` | `kea transaction` subcommands (list, show, edit, delete, clear) | `internal/app`, `internal/store`, `internal/api` |
| `ui` | Shared pterm styles and separators (`ui/styles.go`, `ui/separator.go`) | any kea package |
| `ui/prompts` | Interactive huh forms and input validators | `internal/service`, `internal/store`, `cmd` |
| `ui/views` | pterm/tablewriter rendering and JSON output helpers (`views.WriteJSON`) | `internal/service`, `internal/store`, `cmd` |
| `ui/reconcile` | bubbletea model for the reconcile TUI (`ui/reconcile/model.go`) | `internal/service`, `internal/store`, `cmd` |
| `internal/api` | HTTP server, routes, middleware, handlers, error mapping, SPA serving | `internal/store`, `internal/repository`, `cmd`, `ui` |
| `internal/app` | Composition root: `App`, `NewApp`, `SwitchLedger`, `InitLedgerDB`, `GetAppDataDir` | `cmd`, `ui`, `internal/api` |
| `internal/service` | Business logic: `Service` facade, `AccountService`, `TransactionService`, typed errors | `internal/store`, `internal/app`, `internal/api`, `cmd`, `ui` |
| `internal/repository` | Storage interfaces and sentinel errors (`internal/repository/errors.go`) | anything except `internal/model` |
| `internal/store` | SQLite implementation of the repository interfaces, migrations runner, `Swap` | `internal/service`, `internal/app`, `cmd`, `ui` |
| `internal/model` | Domain types and input structs (`internal/model/input.go`) | any kea package |
| `internal/config` | `Config` struct and `NewDefault` | any kea package |
| `internal/ledger` | Ledger registry (`ledgers.yaml`), active selection, fsnotify watcher | any kea package |
| `internal/backup` | Tiered pre-startup DB backup using the SQLite online backup API | any kea package |
| `internal/utils` | Amount parsing/formatting and math helpers | anything except `internal/model` |
| `internal/web` | `go:embed` of the built SPA (`internal/web/embed.go`) | any kea package |
| `migrations` | Embedded golang-migrate SQL files (`migrations/embed.go`) | any kea package |

Two edges cross layers on purpose: `internal/api` and `cmd/ledger` import `internal/app` to call `app.InitLedgerDB` when creating a ledger, which pulls in `internal/store` transitively. `ui/views` also imports `ui/prompts` (some views, such as `ui/views/account_create.go`, collect input through prompts).

## The service facade

`internal/service/service.go` defines `Service`, which holds unexported `*AccountService`, `*TransactionService` and `*config.Config` fields. Callers use `svc.Account()`, `svc.Transaction()` and `svc.Config()`. `NewService` takes an `AccountRepository`, a `TransactionRepository`, a `BudgetRepository` and a `TransactionManager`; `app.NewApp` passes the same `*store.Store` for all four.

- `AccountService` (`internal/service/account_service.go`, `internal/service/account_ops.go`, `internal/service/account_validation.go`) — account CRUD, tree, balances, search.
- `TransactionService` (`internal/service/transaction_service.go`, `internal/service/transaction_ops.go`, `internal/service/transaction_validation.go`, `internal/service/transaction_classifier.go`) — transaction CRUD, validation, type rules and classification.
- `BudgetService` (`internal/service/budget_service.go`, `internal/service/budget_report.go`) — budget versions and budget vs actual.
- Reports are methods on `TransactionService` in `internal/service/report_service.go` (`GenerateIncomeStatement`, `GenerateBalanceSheet`, `GetDailyNetWorthSeries`, ...).
- Reconcile is also on `TransactionService`, in `internal/service/reconcile_ops.go` (`GetUnreconciledByAccount`, `PreviewReconcile`, `ReconcileTransactions`).
- Service errors (`ErrNotFound`, `ErrReconciled`, `ValidationError`, ...) live in `internal/service/errors.go`.

Command runners usually depend on small interfaces instead of the concrete services (for example `AddProvider` and `TransactionProvider` in `cmd/add_types.go`), which keeps them testable.

## Request flow: CLI (`kea add`)

1. `cmd/kea/main.go` (`main`) — calls `cmd.Execute`, which opens the active ledger and registers commands (see Startup sequence).
2. `cmd/add.go` (`NewAddCmd`) — Cobra command; binds flags into `addFlags` (`cmd/add_types.go`). `RunE` builds an `addRunner` with `svc.Account()`, `svc.Transaction()` and `views.NewTransactionDetailView()`.
3. `cmd/add.go` (`addRunner.Run`) — picks flag mode if any of `--desc/--amount/--from/--to/--type/--split` was set, otherwise interactive mode.
4. Flag mode: `cmd/add_actions.go` (`addRunner.runFromFlags`, `addRunner.runFromSplitFlags`) — both parse amounts with `utils.ParseAmount`, status/type via `model.ParseTransactionStatus` / `model.ParseTransactionType`, and the date via `TransactionService.ParseTransactionDate`. Only `runFromFlags` checks `--from`/`--to` up front (`addRunner.validateAccountSelectable` calls `AccountService.ValidateSelectableAccount`); `runFromSplitFlags` does not validate accounts, so `--split` accounts are first checked in `CreateTransaction` (step 8).
5. Interactive mode: `cmd/add_actions.go` (`addRunner.runInteractive`) — a wizard of `ui/prompts` calls (`PromptTransactionType`, `PromptRegular`, `PromptDescription`, `PromptAmount`, `PromptAccountSelection`, `PromptTransactionStatus`, `PromptTransactionDate`). Allowed account types come from `TransactionService.GetTransactionRule`.
6. Both modes produce an `addTransactionInput`, which `Run` converts to `model.CreateSimpleTransactionInput` (from/to/amount) or `model.CreateTransactionFromSplitsInput` (`--split`).
7. `internal/service/transaction_ops.go` (`TransactionService.CreateSimpleTransaction` or `TransactionService.CreateTransactionFromSplits`) — the simple path infers the type with `DetermineType` when none is given and builds two splits; both delegate to `TransactionService.CreateTransaction`.
8. `internal/service/transaction_ops.go` (`TransactionService.CreateTransaction`) — validates split count, type, status, description, resolves account names to IDs, checks selectability, `ValidateSplitsMatchType`, `ValidateRegular` and `ValidateSplitsBalance`.
9. Still in `CreateTransaction` — `ts.tm.ExecTx` runs `repo.CreateTransactionWithSplits` inside one SQL transaction.
10. `internal/store/sqlite_transaction.go` (`Store.CreateTransactionWithSplits`) — inserts the transaction row, then its splits. Only a constraint error on the transaction-row INSERT becomes the duplicate-external_id error `ErrTransactionExists` (wraps `repository.ErrAlreadyExists`); split-insert errors are wrapped generically. Back in `TransactionService.CreateTransaction`, `repository.ErrAlreadyExists` is translated to `service.ErrAlreadyExists`, which `mapError` turns into HTTP 409.
11. Back in `addRunner.Run` — with `--json`, `views.WriteJSON(views.ToJSONTxDetail(...))`; otherwise `TransactionDetailView.Render` (`ui/views/transaction_detail.go`). Errors bubble to `cmd.Execute`, which prints them with pterm and exits 1.

## Request flow: HTTP (`POST /api/transactions`)

1. `internal/api/server.go` (`Server.Run`) — `net/http` server built by `NewServer`; handler is `Server.routes`.
2. `internal/api/router.go` (`Server.routes`) — chi router with this middleware chain, in order:
   - `chimw.Recoverer` (chi) — turns panics into 500s.
   - `requestIDMiddleware` — random ID, `X-Request-ID` header, stored in the context.
   - `loggerMiddleware` — per-request `slog.Logger` tagged with `request_id`.
   - `accessLogMiddleware` — logs method, path, status, duration.
   - `corsMiddleware` — allows origins from `config.ServerConfig.CORSOrigins` and answers preflight `OPTIONS`.
3. The route `POST /api/transactions` is registered as `apiHandler(s.handleCreateTransaction)`. `apiHandler` (`internal/api/handler.go`) is a `func(w, r) error`; a returned error goes to `writeError`.
4. `internal/api/transactions_write.go` (`Server.handleCreateTransaction`) — `decodeJSON` into `model.CreateTransactionFromSplitsInput` (unknown fields rejected; decode failures become `service.ValidationError`). Path/query helpers for other endpoints live in `internal/api/params.go`.
5. Service — `svc.Transaction().CreateTransactionFromSplits(r.Context(), input)`, then steps 8–10 of the CLI flow run unchanged.
6. The handler re-reads the result with `TransactionService.GetTransactionByID` and responds `201` via `writeJSON`.
7. Errors — `internal/api/errors.go` (`mapError`) maps `service.ValidationError` to 400, `service.ErrNotFound` to 404, `service.ErrAlreadyExists`/`service.ErrReconciled` to 409, ledger errors, and everything else to a generic 500. The full status table is in [http-api.md](http-api.md#errors).

## Transactions and context

- `repository.TransactionManager` has one method, `ExecTx(ctx, fn func(Repository) error) error`. Services call it (via their `tm` field) for any multi-statement write.
- `internal/store/sqlite.go` (`Store.ExecTx`) begins a `*sql.Tx`, wraps it in a new transaction-scoped `Store`, and passes that to `fn` as the `Repository`. A returned error rolls back; nil commits. Calling `ExecTx` on a transaction-scoped store returns "store is already in a transaction" (no nesting).
- `store.DBTX` is the `ExecContext` / `PrepareContext` / `QueryContext` / `QueryRowContext` method set shared by `*sql.DB` and `*sql.Tx`, so every query method works in both modes.
- Inside the `fn`, use the `repo` argument, not the service's own repository fields; otherwise the statement runs outside the transaction.
- Every repository method takes `context.Context` first. Store code calls only the `*Context` variants of `database/sql`. The CLI context comes from `signal.NotifyContext` in `cmd.Execute`; HTTP handlers use `r.Context()`.
- The DSN in `store.NewStore` enables foreign keys, WAL, a 5s busy timeout and `_txlock=immediate`.

## Startup sequence

All of this happens in `cmd/root.go` (`Execute`) before Cobra dispatches; there is no `PersistentPreRun`.

1. `cmd/kea/main.go` (`main`) — calls `cmd.Execute(migrations.FS)`.
2. Pre-parse flags so `--no-color` / `NO_COLOR` can configure pterm (`configureOutput`).
3. `initConfig` — viper reads `config.yaml` from the app data dir (`app.GetAppDataDir`, created from `defaultConfigTemplate` if missing) or the `--config` file; applies server defaults (`setServerDefaults`) and `KEA_`-prefixed env overrides; unmarshals into `config.Config`.
4. `ledger.Load(appDir)` — reads or creates `ledgers.yaml`; with no ledgers it registers `default` pointing at `<appDir>/kea.db`.
5. The `ledger` command group is always registered. If the arguments are a ledger command (`isLedgerCommand`), Cobra runs now, without opening any DB. If there is no active ledger, only ledger commands are available.
6. `app.NewApp` (`internal/app/app.go`) — resolves the active DB path (`Registry.Active`), runs `backup.Run`, opens the DB with `store.NewStore` (which applies migrations via `runMigrations`), builds `service.NewService`, and registers a `Registry.OnSwitch` callback that calls `Store.Swap`.
7. `ensureCurrency` — if `defaults.currency` is empty: interactive terminals get `prompts.PromptInitCurrency`, others default to USD; the choice is saved to the config file.
8. `signal.NotifyContext` creates the cancellable context for SIGINT/SIGTERM.
9. `initSysAcc` — runs `migrateLegacySysAcc`, then creates the per-currency opening-balances account if missing (rules in domain.md).
10. Register `account`, `transaction`, `add`, `info`, `report`, `reconcile`, `serve`, then `rootCmd.ExecuteContext(ctx)`. The app `cleanup` stops the registry watcher and closes the DB.

`kea serve` differences (`cmd/serve.go`, `NewServeCmd`): it starts `Registry.Watch` in a goroutine so an external `kea ledger switch` makes the running server swap databases, and it passes `App.SwitchLedger` to `api.NewServer` so `POST /api/ledgers/switch` can swap at runtime. It logs JSON to stderr via `slog` and shuts down gracefully when the context is cancelled.

`config.DatabaseConfig.Path` is still parsed and `~`-expanded, but the database actually opened is always the registry's active ledger.

## Multiple ledgers

- A ledger is a name mapped to one SQLite file. `ledger.Registry` (`internal/ledger/registry.go`) stores `active` and a `ledgers` map of `Entry{Path}` in `<appDir>/ledgers.yaml`.
- New ledgers default to `<appDir>/ledgers/<name>.db` (`cmd/ledger/add.go`, `Server.handleCreateLedger` in `internal/api/ledgers.go`); `app.InitLedgerDB` creates the file and runs migrations.
- Active selection: `Registry.Active` / `Registry.ActiveName`. The `KEA_LEDGER` environment variable overrides the file's `active` field.
- Switching from the CLI (`kea ledger switch`) only rewrites `ledgers.yaml`; the next process opens the new ledger.
- Switching in a running server: either `App.SwitchLedger` (HTTP) calls `Store.Swap` directly and then `Registry.Switch`, or `Registry.Watch` notices the file change, `reload` fires the `OnSwitch` callback, and that calls `Store.Swap`. `App.RuntimeState` reports the current ledger name and DB path.
- `Store.Swap` (`internal/store/sqlite.go`) opens and migrates the new DB, replaces the connection under the store's write lock, then closes the old one. Because the `Service` holds the same `*store.Store`, no rewiring is needed. Swap does not re-run `backup.Run` or `initSysAcc`.

## How `kea serve` ships the SPA

- The React app in `spa/` builds with Vite; `spa/vite.config.ts` sets `build.outDir` to `internal/web/dist`. The build command is described in development.md.
- `internal/web/embed.go` embeds that directory with `//go:embed all:dist`, and `web.FS` returns it rooted at `index.html`. A placeholder `internal/web/dist/index.html` is committed so the Go build works without a SPA build.
- `internal/api/router.go` mounts `spaHandler(web.FS())` on `/*` after the `/api` routes.
- `internal/api/spa.go` (`spaHandler`) — rejects paths containing `..`; serves an existing file directly (files under `assets/` get a one-year immutable `Cache-Control`, others `no-cache`); any unknown path falls back to `index.html` so client-side routes work.
- During development the Vite dev server runs separately (port 5173) and calls the API cross-origin, which is why `http://localhost:5173` is in the default CORS origins.
