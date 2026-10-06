# Decisions

> **Read this when:** you wonder why something is built the way it is, or before reversing an existing choice.
>
> **Related:** [architecture.md](architecture.md) · [domain.md](domain.md) · [history/](history/)

Entries are grouped by area. Each one holds in the current code; superseded choices are omitted. Sources are point-in-time records and may describe details that have since changed.

## Storage

### One SQLite file per ledger, tracked in `ledgers.yaml`
- **Decision:** Each ledger is an independent SQLite database; a YAML registry maps names to files and records the active one.
- **Why:** Separate books (personal, business) stay fully isolated without a tenant column, and the service and store layers only ever see a plain DB path.
- **Where:** `internal/ledger/registry.go` (`Registry`, `Registry.Active`), `internal/app/app.go` (`NewApp`)
- **Source:** [2026-04-15-multi-ledger-design.md](history/superpowers/specs/2026-04-15-multi-ledger-design.md)

### WAL mode with a small connection pool and immediate write locks
- **Decision:** The DSN enables WAL, a 5s busy timeout and `_txlock=immediate`, and the pool allows 4 connections.
- **Why:** The CLI and a running `kea serve` share the same file; WAL lets readers proceed during writes, and immediate locking makes writers queue on the busy timeout instead of failing on lock upgrade.
- **Where:** `internal/store/sqlite.go` (`NewStore`)
- **Source:** [2026-05-13-sqlite-wal-mode.md](history/superpowers/plans/2026-05-13-sqlite-wal-mode.md); the plan's single-connection pool was replaced in commit `b308ac7` — fix: increase SQLite connection pool to leverage WAL concurrency (#109)

### Automatic tiered backup on every startup
- **Decision:** Before opening the DB, kea keeps daily (7), weekly (4) and monthly (12) copies in a `backups/` folder next to it, using the SQLite online backup API; a failure only prints a warning.
- **Why:** Protects against bad migrations and mistakes with zero user action, without ever blocking startup; the online API (not a file copy) gives a consistent snapshot even while a server holds the DB open in WAL mode.
- **Where:** `internal/backup/backup.go` (`Run`, `tiers`, `doBackup`), `internal/app/app.go` (`NewApp`)
- **Source:**
  - [2026-04-14-db-backup-design.md](history/superpowers/specs/2026-04-14-db-backup-design.md)
  - [2026-05-24-fix-backup-wal-safety.md](history/superpowers/plans/2026-05-24-fix-backup-wal-safety.md)

### Storage errors cross the boundary as repository sentinels
- **Decision:** `internal/repository` owns `ErrNotFound` and `ErrAlreadyExists`; store errors wrap them, so the service checks repository sentinels and never imports `internal/store`.
- **Why:** Keeps the dependency arrow pointing down and lets service tests use mocks that return the same sentinels as the real store.
- **Where:** `internal/repository/errors.go`, `internal/store/errors.go` (`ErrRecordNotFound`, `ErrAccountExists`)
- **Source:** [2026-05-13-fix-service-store-error-coupling.md](history/superpowers/plans/2026-05-13-fix-service-store-error-coupling.md)

### Large ID lists are chunked before building `IN` clauses
- **Decision:** Queries that filter by a list of IDs split it into batches of `sqliteChunkSize` (500) and merge the results.
- **Why:** SQLite caps the number of bound variables per statement, so an unbounded `IN (?,...)` fails when reconciling or loading split details for many transactions at once.
- **Where:** `internal/store/chunk.go` (`chunkInt64`, `sqliteChunkSize`), `internal/store/sqlite_reconcile.go`, `internal/store/sqlite_transaction.go`
- **Source:** [2026-05-23-chunk-in-clauses.md](history/superpowers/plans/2026-05-23-chunk-in-clauses.md)

## Domain

### Money is `int64` cents end to end
- **Decision:** Amounts are integer cents in storage, business logic and the HTTP API; formatting uses integer arithmetic only.
- **Why:** Floats lose precision; an earlier float-based `FormatAmount` dropped cents on large values and was rewritten.
- **Where:** `internal/model/types.go` (`CentsPerUnit`), `internal/utils/amount.go` (`FormatAmount`, `ParseAmount`)
- **Source:** [2026-05-17-fix-numeric-precision.md](history/superpowers/plans/2026-05-17-fix-numeric-precision.md)

### Transaction type is stored, not recomputed
- **Decision:** `transactions.type` is a stored column chosen at creation (by the user or `DetermineType`) and validated against the splits.
- **Why:** Inferring the type from splits at display time mislabelled ambiguous multi-split transactions; storing it preserves the user's intent and strict validation keeps it consistent.
- **Where:** `migrations/0005_add_transaction_type.up.sql`, `internal/service/transaction_classifier.go` (`ValidateSplitsMatchType`, `DetermineType`)
- **Source:** [2026-04-23-transaction-type-storage-design.md](history/superpowers/specs/2026-04-23-transaction-type-storage-design.md)

### One opening-balances equity account per currency
- **Decision:** Opening balances post to `Equity:OpeningBalances_<CCY>`, created on demand; the legacy single account is renamed at startup.
- **Why:** A single equity account mixed currencies, so its balance was meaningless and opening splits were recorded in the wrong currency.
- **Where:** `internal/model/types.go` (`OpeningBalancesAccountName`), `internal/service/account_ops.go` (`createOpeningBalanceInRepo`), `cmd/root.go` (`migrateLegacySysAcc`)
- **Source:** [2026-04-01-per-currency-opening-balances-design.md](history/superpowers/specs/2026-04-01-per-currency-opening-balances-design.md)

### Currencies are never mixed or converted
- **Decision:** A transaction's splits share one currency, and every report total is a map keyed by currency; there is no FX conversion.
- **Why:** Summing different currencies produced wrong totals; FX rates are an explicit non-goal for a personal ledger.
- **Where:** `internal/service/transaction_validation.go` (`ValidateSplitsBalance`), `internal/service/report_service.go`, `internal/model/report.go`
- **Source:**
  - [2026-05-08-fix-cross-currency-report-totals.md](history/superpowers/plans/2026-05-08-fix-cross-currency-report-totals.md)
  - [2026-05-08-fix-currency-consistency-on-edit.md](history/superpowers/plans/2026-05-08-fix-currency-consistency-on-edit.md)

### Account type must agree with its root name and parent
- **Decision:** The first name segment fixes the type (`Expenses:*` is always `E`), and a child must have its parent's type.
- **Why:** Reports group by stored type while views group by name; letting them disagree broke data integrity for direct service callers such as the API.
- **Where:** `internal/service/account_ops.go` (`validateAccountFields`, `validateParentType`), `internal/model/types.go` (`AccountTypeFromRootName`)
- **Source:** [2026-05-20-account-type-hierarchy-validation-design.md](history/superpowers/specs/2026-05-20-account-type-hierarchy-validation-design.md)

### Investments are detected by the `Assets:Investments:` name prefix
- **Decision:** Any transaction touching an `Assets:Investments:*` account plus another Asset/Liability account is typed `Investment`, ahead of the Income/Expense/Transfer rules.
- **Why:** Buys and sells were mislabelled as Transfer or Income; a name prefix mirrors the opening-balances convention and needs no schema change.
- **Where:** `internal/model/types.go` (`InvestmentAccountPrefix`, `IsInvestmentAccount`), `internal/service/transaction_classifier.go` (`DetermineType`)
- **Source:** [2026-06-16-investment-transaction-type-design.md](history/superpowers/specs/2026-06-16-investment-transaction-type-design.md)

### `Regular` is a nullable flag for Income and Expense only
- **Decision:** `Regular` is a `*bool` that is set (default `true`) for Income/Expense and absent for every other type, enforced in the service and by a DB `CHECK`.
- **Why:** It answers "how much of my spending is habitual"; it is named Regular to keep "Recurring" free for a future scheduling feature, and defaults to true because most entries are habitual.
- **Where:** `internal/model/transaction.go` (`Transaction.Regular`), `internal/service/transaction_validation.go` (`ValidateRegular`), `migrations/0011_add_transaction_regular.up.sql`
- **Source:** [2026-06-25-regular-transaction-attribute-design.md](history/superpowers/specs/2026-06-25-regular-transaction-attribute-design.md)

### Reconciliation is tracked per split, not per transaction
- **Decision:** `splits.reconciled` marks each account's side separately; the transaction becomes `Reconciled` as soon as one of its splits is.
- **Why:** A transfer between two banks must be reconcilable against each bank's statement independently. This replaced the original transaction-level design.
- **Where:** `migrations/0003_add_split_reconciled.up.sql`, `internal/store/sqlite_reconcile.go` (`Store.MarkSplitsReconciledByAccount`)
- **Source:** commit `d2cae4e` — fix(reconcile): track reconciliation at split level, not transaction level; original design in [2026-04-16-reconciliation-design.md](history/superpowers/specs/2026-04-16-reconciliation-design.md)

### Reconciliation is the only thing that locks a transaction
- **Decision:** Reconciled transactions are immutable (`ErrReconciled`); no other transaction, including opening balances, is specially protected.
- **Why:** A hard-coded "system transaction ID 1" guard protected an arbitrary row; it was removed so that only the user's explicit reconcile act freezes history.
- **Where:** `internal/service/transaction_ops.go` (`DeleteTransaction`, `UpdateTransactionComplete`), `internal/service/errors.go` (`ErrReconciled`)
- **Source:** [2026-05-20-remove-system-transaction-id.md](history/superpowers/plans/2026-05-20-remove-system-transaction-id.md)

### Budgets are versioned by effective month
- **Decision:** A budget row applies from its `effective_month` until a later row for the same account replaces or stops it; editing adds a version instead of rewriting the amount.
- **Why:** Past months must keep showing the budget that applied then; one table covers "amount changes from July" without per-month rows.
- **Where:** `migrations/0012_create_budgets.up.sql`, `internal/service/budget_service.go` (`activeBudgets`)
- **Source:** [2026-10-05-monthly-budget-design.md](history/superpowers/specs/2026-10-05-monthly-budget-design.md)

### Budget actuals use signed sums
- **Decision:** Budget actual spending sums Expense split amounts with their sign, so refunds reduce it; the transaction scope (Expense-typed only) matches the expense report.
- **Why:** A refund must give budget back; the existing reports' absolute-value sums would count it as more spending.
- **Where:** `internal/service/budget_report.go` (`GenerateBudgetReport`)
- **Source:** [2026-10-05-monthly-budget-design.md](history/superpowers/specs/2026-10-05-monthly-budget-design.md)

### Savings are measured as the target account's balance change
- **Decision:** A savings target's "saved" is the signed sum of the splits on the target Asset account and its descendants in the month, regardless of transaction type, instead of income minus expense or net-worth change.
- **Why:** The user keeps a dedicated savings account; what matters is whether money actually arrived there. Income minus expense counts money that may stay in a spending account, and net-worth change is noisy with equity transactions.
- **Where:** `internal/service/savings_report.go` (`GenerateSavingsReport`)
- **Source:** [2026-10-06-savings-target-design.md](history/superpowers/specs/2026-10-06-savings-target-design.md)

### Opening transactions do not count as saving
- **Decision:** `Opening`-typed transactions are excluded from saved amounts; every other type counts.
- **Why:** Recording an existing balance is not saving; without the exclusion the month a savings account is set up would show its whole balance as saved.
- **Where:** `internal/service/savings_report.go`
- **Source:** [2026-10-06-savings-target-design.md](history/superpowers/specs/2026-10-06-savings-target-design.md)

### Savings targets have their own table and service
- **Decision:** Savings targets use a separate `savings_targets` table and `SavingsService`; only the version-selection logic is shared with budgets, through generic helpers.
- **Why:** Budgets cap Expense spending and targets set a floor on Asset growth; a shared table would need a kind column, a SQLite table rebuild and kind checks in every budget query.
- **Where:** `migrations/0013_create_savings_targets.up.sql`, `internal/service/savings_service.go`, `internal/service/versions.go`
- **Source:** [2026-10-06-savings-target-design.md](history/superpowers/specs/2026-10-06-savings-target-design.md)

## Service layer

### The repository is the source of truth for account types
- **Decision:** Validation and classification resolve each split's account type from the repository and ignore any caller-supplied `AccountType`.
- **Why:** Trusting the input let a direct caller fake account types and bypass type validation.
- **Where:** `internal/service/transaction_classifier.go` (`resolveAccountType`)
- **Source:** [2026-05-20-split-type-trust-boundary-design.md](history/superpowers/specs/2026-05-20-split-type-trust-boundary-design.md)

### Services translate errors into typed service errors
- **Decision:** Services return `service.ErrNotFound`, `service.ErrAlreadyExists` or a `*ValidationError` (with `Field`) instead of repository sentinels; the API never maps repository or store errors.
- **Why:** Presentation layers can pick 400/404/409/500 with `errors.Is`/`errors.As` without importing storage packages; an unknown account in a request body is a 400, not a 404.
- **Known gap:** `UpdateTransactionComplete` runs `ValidateSplitsMatchType` before checking that split accounts exist, so an unknown split `account_id` leaks `repository.ErrNotFound` and the API answers 500.
- **Where:** `internal/service/errors.go` (`ValidationError`, `validationErrorf`), `internal/api/errors.go` (`mapError`)
- **Source:**
  - [2026-05-14-structured-validation-errors.md](history/superpowers/plans/2026-05-14-structured-validation-errors.md)
  - [2026-06-05-fix-create-transaction-split-account-errnotfound-design.md](history/superpowers/specs/2026-06-05-fix-create-transaction-split-account-errnotfound-design.md)

### Service inputs are structs in `internal/model`
- **Decision:** Create/update methods take input structs (`CreateAccountInput`, `CreateTransactionFromSplitsInput`, ...) that the HTTP API also decodes request bodies into.
- **Why:** Replaces long positional parameter lists and gives the CLI and the API one shared contract without a translation layer.
- **Where:** `internal/model/input.go`
- **Source:** [2026-05-14-issue-79-input-structs.md](history/superpowers/plans/2026-05-14-issue-79-input-structs.md)

### Services own transaction boundaries; reads that guard writes go inside them
- **Decision:** Multi-step operations run in one `ExecTx`, including the validating reads; store methods use the shared `DBTX` and never begin their own transaction.
- **Why:** `CreateAccountWithBalance` once left an account without its opening balance on failure, reconcile had a check-then-write race, and a self-managed store transaction could not compose with `ExecTx`.
- **Where:** `internal/store/sqlite.go` (`Store.ExecTx`), `internal/service/account_ops.go` (`CreateAccountWithBalance`), `internal/service/reconcile_ops.go` (`ReconcileTransactions`)
- **Source:**
  - [2026-05-13-atomic-create-account-with-balance.md](history/superpowers/plans/2026-05-13-atomic-create-account-with-balance.md)
  - [2026-05-24-fix-reconcile-toctou.md](history/superpowers/plans/2026-05-24-fix-reconcile-toctou.md)
  - [2026-05-24-rename-account-exectx-design.md](history/superpowers/specs/2026-05-24-rename-account-exectx-design.md)

### A balance mismatch is the caller's policy, not the service's
- **Decision:** `ReconcileTransactions` always commits and returns the difference; the CLI (`--force`, TUI y/n) and the API (`allow_mismatch`) decide whether a non-zero difference is allowed.
- **Why:** Reconciliation is a soft check, so the user may accept a mismatch; each front end gates it in its own idiom, and the API rejects by default as defence against SPA bugs.
- **Where:** `internal/service/reconcile_ops.go` (`PreviewReconcile`, `ReconcileTransactions`), `internal/api/reconcile.go` (`Server.handleReconcileCommit`), `internal/api/errors.go` (`balanceMismatchError`), `cmd/reconcile_actions.go`
- **Source:**
  - [2026-04-16-reconciliation-design.md](history/superpowers/specs/2026-04-16-reconciliation-design.md)
  - [2026-06-04-web-api-reconcile-design.md](history/superpowers/specs/2026-06-04-web-api-reconcile-design.md)

## CLI

### `--json` output uses its own DTOs with decimal units
- **Decision:** CLI JSON goes through DTOs in `ui/views` that convert cents to float units and timestamps to date strings, unlike the HTTP API.
- **Why:** The CLI JSON targets scripts and agents reading human-scale numbers; the HTTP API serves the SPA, which formats raw cents itself.
- **Where:** `ui/views/json.go` (`WriteJSON`, `CentsToUnit`), `ui/views/json_types.go`
- **Source:** [2026-03-24-json-output-design.md](history/superpowers/specs/2026-03-24-json-output-design.md)

### Non-interactive startup never prompts
- **Decision:** If no default currency is configured, a terminal user is prompted once; any other context silently gets USD and the choice is saved.
- **Why:** The interactive wizard blocked forever under `kea serve`, pipes, cron and Docker.
- **Where:** `cmd/root.go` (`ensureCurrencyWith`, `isInteractive`)
- **Source:** [2026-05-19-non-interactive-currency-fallback-design.md](history/superpowers/specs/2026-05-19-non-interactive-currency-fallback-design.md)

## HTTP API

### Local-only, single user, no authentication
- **Decision:** `kea serve` binds to `localhost` by default and has no login, sessions or CSRF protection; CORS only allows the Vite dev origin.
- **Why:** The web UI is a local companion to the CLI on the same machine and DB file; remote access would be a major redesign, not a setting.
- **Where:** `internal/config/config.go` (`NewDefault`), `internal/api/middleware.go` (`corsMiddleware`)
- **Source:** [2026-06-02-pre-development-review.md](history/web-layer/2026-06-02-pre-development-review.md)

### chi router with error-returning handlers
- **Decision:** Handlers have the signature `func(w, r) error`, wrapped by `apiHandler`, and one `mapError` turns errors into status codes; logging is stdlib `slog`.
- **Why:** Centralises error-to-HTTP mapping so every handler stays linear; chi keeps the standard `net/http` signature with minimal dependencies.
- **Where:** `internal/api/handler.go` (`apiHandler`), `internal/api/errors.go` (`mapError`), `internal/api/router.go` (`Server.routes`)
- **Source:** [2026-06-02-web-api-foundation-design.md](history/superpowers/specs/2026-06-02-web-api-foundation-design.md)

### Responses are bare model structs; request bodies are strict
- **Decision:** Return the `internal/model` type when it already fits (accounts, transactions, reports); use an API-local struct for composite or derived shapes (`reconcileCommitResponse`, `ledgerInfo`, `configResponse`, `balanceResponse`). Wire format is snake_case tags, cents, Unix seconds; bodies reject unknown fields and IDs come only from the path.
- **Why:** Zero translation layer between model and wire, and no way to smuggle an `id` that disagrees with the URL.
- **Where:** `internal/api/handler.go` (`writeJSON`, `decodeJSON`), `internal/model/input.go` (`UpdateTransactionInput`)
- **Source:**
  - [2026-06-03-web-api-read-endpoints-design.md](history/superpowers/specs/2026-06-03-web-api-read-endpoints-design.md)
  - [2026-06-03-web-api-write-endpoints-design.md](history/superpowers/specs/2026-06-03-web-api-write-endpoints-design.md)

### API ledger management cannot touch arbitrary files
- **Decision:** `POST /api/ledgers` always creates `<appDir>/ledgers/<name>.db`, and `DELETE` only unregisters a ledger, leaving its file.
- **Why:** Keeps the HTTP surface from writing SQLite files to arbitrary paths or deleting data; the destructive option stays CLI-only.
- **Where:** `internal/api/ledgers.go` (`Server.handleCreateLedger`, `Server.handleDeleteLedger`)
- **Source:** [2026-06-04-web-api-ledgers-design.md](history/superpowers/specs/2026-06-04-web-api-ledgers-design.md)

### A running server follows ledger switches by swapping the store
- **Decision:** `Store.Swap` replaces the DB connection in place; API switches call it synchronously, and only `kea serve` runs the `ledgers.yaml` watcher.
- **Why:** Avoids split-brain between CLI and web without rewiring the service; a synchronous swap means the SPA's next request hits the new ledger, and short-lived CLI runs do not need a watcher.
- **Where:** `internal/store/sqlite.go` (`Store.Swap`), `internal/app/app.go` (`App.SwitchLedger`), `cmd/serve.go` (`NewServeCmd`)
- **Source:**
  - [2026-05-19-ledger-file-watch-design.md](history/superpowers/specs/2026-05-19-ledger-file-watch-design.md)
  - [2026-06-04-web-api-ledgers-design.md](history/superpowers/specs/2026-06-04-web-api-ledgers-design.md)
  - [2026-06-07-fix-serve-registry-watcher-wire-design.md](history/superpowers/specs/2026-06-07-fix-serve-registry-watcher-wire-design.md)

### Shared mutable state is locked and kept off `Config`
- **Decision:** Every `Registry` accessor holds a plain `sync.Mutex`; the active ledger name and path live on `App` behind an `RWMutex`, and `config.Config` holds only loaded settings.
- **Why:** Concurrent handlers plus the watcher goroutine caused Go's concurrent-map panic and data races; an `RWMutex` or copy-on-write registry bought nothing for a single local user.
- **Where:** `internal/ledger/registry.go` (`Registry`, `Registry.saveLocked`), `internal/app/app.go` (`RuntimeState`, `App.RuntimeState`)
- **Source:**
  - [2026-06-05-registry-mutex-locking-design.md](history/superpowers/specs/2026-06-05-registry-mutex-locking-design.md)
  - [2026-06-09-runtime-state-off-config-design.md](history/superpowers/specs/2026-06-09-runtime-state-off-config-design.md)

## SPA

### The SPA is embedded in the Go binary
- **Decision:** The Vite build is embedded with `go:embed` and served by `kea serve` on the same origin as the API, with an `index.html` fallback for client routes; a committed placeholder keeps `go build` working without a SPA build.
- **Why:** A single self-contained binary runs the whole app, deep links survive refresh, and Go-only contributors are not forced to install Node.
- **Where:** `internal/web/embed.go` (`FS`), `internal/api/spa.go` (`spaHandler`)
- **Source:** [2026-06-13-spa-static-serving-design.md](history/web-layer/2026-06-13-spa-static-serving-design.md)

### Server-wide settings come from the API, not the build
- **Decision:** The SPA reads `defaults.currency` and `display.hide_decimals` from `GET /api/config`, and the hide-decimals toggle writes back through `PATCH /api/config` to `config.yaml`.
- **Why:** A Vite env var drifted from the server's configured currency; one source of truth avoids double configuration.
- **Where:** `internal/api/config.go` (`Server.handlePatchConfig`), `spa/src/lib/server-config.tsx`
- **Source:**
  - [2026-06-09-api-config-endpoint-design.md](history/superpowers/specs/2026-06-09-api-config-endpoint-design.md)
  - [2026-06-18-spa-hide-decimals-toggle-design.md](history/superpowers/specs/2026-06-18-spa-hide-decimals-toggle-design.md)

### View state lives in the URL, remembered per ledger in `localStorage`
- **Decision:** Filters and pagination are URL search params; route loaders save them per ledger in `localStorage` and restore them on return, and the dashboard layout is stored the same way.
- **Why:** URLs keep views deep-linkable and back/forward-safe, while per-ledger memory returns users to the view they left; there is no backend storage because kea is single-user and local.
- **Where:** `spa/src/lib/filter-memory.ts`, `spa/src/lib/dashboard/storage.ts`
- **Source:**
  - [2026-06-26-spa-filter-memory-design.md](history/web-layer/2026-06-26-spa-filter-memory-design.md)
  - [2026-06-28-webui-dashboard-design.md](history/2026-06-28-webui-dashboard-design.md)
