# Kea Web Layer — Pre-Development Review Summary

**Date:** 2026-06-02
**Status:** Codebase ready for Web Layer development

This document summarizes the pre-development code review for the Kea Web Layer. Hand this to a fresh session as context when starting design discussions.

---

## 1. Web Layer Scope (confirmed with user)

- **Architecture:** CLI/TUI and Web (React SPA + REST API) both sit on top of the same service layer (`internal/service/`).
- **Frontend:** React SPA.
- **Deployment:** **Local-only, single-user** — `kea serve` runs on `localhost`, shares the same SQLite file as the CLI. No multi-user, no remote hosting, no auth.
- **Both interfaces coexist** — the user can use CLI commands while the web server is running.

---

## 2. Project Architecture (as of review)

```
cmd/              Cobra commands → call service methods
  cmd/ledger/     Ledger management subcommands
internal/app/     Wires service + store (DI entry point)
internal/service/ Business logic — AccountService, TransactionService, reports, reconcile
internal/store/   SQLite implementation of repository interfaces
internal/model/   Domain types (Account, Transaction, Split, etc.)
internal/repository/interfaces.go  Contracts between service and store
internal/config/  Config struct + Viper loading
internal/ledger/  Ledger registry (multiple named DBs, active selection)
internal/backup/  Pre-startup DB backup
internal/utils/   Amount formatting/parsing helpers
ui/               charmbracelet/huh prompts + pterm views
migrations/       golang-migrate SQL files embedded via FS
```

### Key domain rules
- **Amounts:** always `int64` cents. Use `utils.FormatAmount` / `utils.ParseAmount`.
- **Double-entry:** every transaction's splits must sum to zero.
- **Account types:** A (Asset), L (Liability), C (Equity), R (Revenue), E (Expense). Only leaf accounts hold transactions.
- **Protected records:** Transaction ID 1 and reconciled transactions are immutable.
- **System accounts:** per-currency `Equity:OpeningBalances_<CCY>` (e.g. `Equity:OpeningBalances_USD`).
- **Service facade:** `service.Service` exposes `svc.Account()`, `svc.Transaction()`, `svc.Config()`.

### Service method conventions
- All methods take `context.Context` as first arg.
- Errors use sentinels (`ErrNotFound`, `ErrAlreadyExists`, `ErrReconciled`, `ErrNotEditable`) + `ValidationError` struct with field name.
- `errors.Is` / `errors.As` work consistently across the layer.

---

## 3. Issues Identified and Resolved

**30 web-layer issues filed and closed.** All fixes merged to main.

### Pre-existing issues that impacted web layer (7)
| # | Title |
|---|-------|
| #22 | Account hierarchy lacks circular-reference validation on ParentID |
| #25 | Inconsistent error wrapping in GetTransactionByID |
| #29 | CreateAccountWithBalance non-atomic; orphan account on opening-balance failure |
| #44 | Subaccount currency overrides are not normalized |
| #58 | N+1 query problem in transaction list command |
| #60 | No store (integration) tests |
| #68 | context.Background() used in startup instead of propagating cmd context |

### First-round issues (7) — initial blockers
| # | Title |
|---|-------|
| #74 | Model structs missing JSON tags for API serialization |
| #75 | SQLite missing WAL mode and busy timeout |
| #76 | Business logic in cmd/ layer not reusable by API handlers |
| #77 | Service errors lack structure for HTTP status code mapping |
| #78 | List methods lack pagination support |
| #79 | Service methods use many positional parameters instead of input structs |
| #80 | Backup uses raw file copy instead of SQLite backup API |

### Second-round blockers — service reusability focus (9)
| # | Title |
|---|-------|
| #104 | No public GetAccountByID on AccountService |
| #105 | RenameAccount takes segment-only; mutations don't return updated entity |
| #106 | Inconsistent ErrNotFound translation from repo → service |
| #107 | Transaction list display-field assembly trapped in cmd/ |
| #108 | Hidden-account filtering only in cmd/, not service |
| #109 | SetMaxOpenConns(1) blocks concurrent HTTP request handling |
| #110 | RenameAccount in store bypasses ExecTx |
| #111 | Backup uses raw file copy when db=nil; unsafe with running server |
| #112 | determineMode duplicated in cmd/add and cmd/transaction/edit |

### Third-round deep review — HIGH severity (9)
| # | Title |
|---|-------|
| #113 | CreateTransaction allows creating already-reconciled transactions |
| #114 | No multi-dimension transaction filtering |
| #115 | No account search by partial name (autocomplete) |
| #116 | ListResult and ListOptions missing JSON tags |
| #117 | Config struct has no web server fields (port, host, CORS) |
| #118 | ensureCurrency launches interactive TUI prompt — blocks server |
| #119 | CLI ledger switch invisible to running web server (split-brain) |
| #120 | Ignored GetAccountByName errors cause nil-pointer panics |
| #121 | JSONTxDetail missing Type field |

### Third-round deep review — MEDIUM severity (12)
| # | Title |
|---|-------|
| #122 | GetAccountBalance returns 0 for nonexistent accounts instead of 404 |
| #123 | No length/content validation on Description and Memo fields |
| #124 | ParseTransactionStatus silently defaults unknown values to Cleared |
| #125 | Unbounded IN clauses can exceed SQLite variable limit |
| #126 | Duplicate external_id error not wrapped with sentinel |
| #127 | ReconcileTransactions TOCTOU race — validation reads outside ExecTx |
| #128 | ParseAmount silently overflows on large dollar values |
| #129 | Signal handler only catches SIGINT, not SIGTERM |
| #130 | TransactionType has no MarshalJSON/UnmarshalJSON |
| #131 | No account tree/hierarchy service method |
| #132 | Missing composite index on splits(account_id, reconciled) |
| #133 | ToJSONTxListItem ParseFloat fails on comma-formatted amounts |

---

## 4. Current State Verification (verified by spot-check)

| Concern | Status | Evidence |
|---------|--------|----------|
| SQLite concurrency | ✅ Fixed | `_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate`, `SetMaxOpenConns(4)` |
| Service layer reusability | ✅ Fixed | `GetAccountByID`, `SearchAccounts`, `GetAccountTree`, `ListAccounts`, `BuildTransactionListItems` all in service |
| Multi-dimension filtering | ✅ Fixed | `TransactionFilter` struct (`internal/model/transaction_filter.go`) + `FilterTransactions` repo method |
| Error translation | ✅ Fixed | Service methods consistently translate `repository.ErrNotFound` → `service.ErrNotFound` |
| JSON serialization | ✅ Fixed | `ListResult`/`ListOptions` have JSON tags, `TransactionType` has validated unmarshal |
| Web server config | ✅ Fixed | `ServerConfig` (Host, Port, CORSOrigins) added with defaults: `localhost:8080`, CORS for `localhost:5173` |
| Graceful shutdown | ✅ Fixed | `signal.NotifyContext(..., os.Interrupt, syscall.SIGTERM)` in `cmd/root.go` |
| Non-interactive startup | ✅ Fixed | `ensureCurrencyWith(cfg, isInteractive())` — won't block on stdin |
| All tests pass | ✅ | `go test ./...` clean across all packages |

### Available service methods relevant to the API

**AccountService:**
- `GetAllAccounts(ctx)` / `ListAccounts(ctx, opts)` / `GetAccountTree(ctx, opts)`
- `GetAccountByID(ctx, id)` / `GetAccountByName(ctx, name)`
- `GetAccountsByType(ctx, type)` / `SearchAccounts(ctx, filter, opts)`
- `GetAccountBalance(ctx, id)` / `GetAllAccountBalances(ctx, asOf)`
- `CreateAccount(ctx, input)` / `CreateAccountWithBalance(ctx, input)`
- `UpdateAccountMetadata(ctx, id, desc, hidden)` / `RenameAccount(ctx, oldName, newSegment)`
- `DeleteAccountByName(ctx, name)`

**TransactionService:**
- `GetTransactionByID(ctx, id)` / `GetTransactionDetailsByIDs(ctx, txs)`
- `GetRecentTransactions(ctx, limit)` / `ListRecentTransactions(ctx, opts)` / `ListTransactionHistory(ctx, accountName, opts)`
- `FilterTransactions(ctx, filter, opts)` — multi-dimension filter
- `CreateTransaction(ctx, input)` / `CreateSimpleTransaction(ctx, input)` / `CreateTransactionFromSplits(ctx, input)`
- `UpdateTransactionStatus(ctx, id, status)` / `UpdateTransactionComplete(ctx, input)`
- `DeleteTransaction(ctx, id)` / individual split CRUD
- `BuildTransactionListItems(ctx, txs, details)` — assembles display-ready items
- Reports: `GenerateIncomeStatement`, `GenerateBalanceSheet`, `GenerateFullIncomeStatement`, etc.
- Reconciliation: `GetUnreconciledByAccount`, `PreviewReconcile`, `ReconcileTransactions`

**Error sentinels (in `internal/service/errors.go`):**
- `ErrNotFound` → 404
- `ErrAlreadyExists` → 409
- `ErrReconciled` → 409
- `ErrNotEditable` → 403
- `ErrCircularParent` → 409
- `*ValidationError` (with `Field` and `Message`) → 400

---

## 5. Open Design Questions for Web Layer

These are decisions to make in the design discussion:

### Server framework
- **chi** (minimal, idiomatic Go) — recommended for this scope
- **gorilla/mux** (mature, slightly heavier)
- **echo** (batteries included; more opinionated)
- **net/http only** with manual routing (very minimal but more boilerplate)

### API style
- **REST + JSON** (matches the existing model shape; pairs well with TanStack Query)
- **GraphQL** (more flexibility but overkill for a single-user local app)
- **JSON-RPC** (lower ceremony but less standard)

### Frontend stack
- **Vite + React + TanStack Query** (recommended — clean DX, pairs with `ListResult`)
- **Next.js** (SSR/SSG — overkill for local SPA)
- **Remix** (also more than needed)

### Data fetching patterns
- Pagination: query params `?limit=20&offset=0` mapping to `ListOptions`
- Filtering: query params mapping to `TransactionFilter`
- How to handle reconciliation flow (multi-step UI)?

### Endpoint structure (initial sketch)
```
GET    /api/accounts                    List/filter/search accounts
GET    /api/accounts/:id                Get single account
GET    /api/accounts/:id/balance        Get balance
GET    /api/accounts/tree               Hierarchical view
POST   /api/accounts                    Create
PATCH  /api/accounts/:id                Update metadata or rename
DELETE /api/accounts/:id

GET    /api/transactions                List with TransactionFilter
GET    /api/transactions/:id            Get detail
POST   /api/transactions                Create
PATCH  /api/transactions/:id            Update
PATCH  /api/transactions/:id/status     Change status
DELETE /api/transactions/:id

GET    /api/reports/income-statement    ?from=&to= or ?month=
GET    /api/reports/balance-sheet       ?as_of=
GET    /api/reports/net-worth           ?at=

GET    /api/accounts/:id/unreconciled   Reconcile prep
POST   /api/accounts/:id/reconcile/preview
POST   /api/accounts/:id/reconcile

GET    /api/ledgers                     List
POST   /api/ledgers/switch              Switch active
```

### Cross-cutting concerns to design
- **Error middleware** — single place to map service errors to HTTP status codes
- **Logging** — structured logs (slog?), request ID propagation
- **CORS** — already configured for `localhost:5173` (Vite default)
- **Embedding the SPA** — embed React build output via `go:embed` for single-binary distribution?
- **Hot reload during dev** — proxy from Vite dev server to Go API
- **Database connection sharing** — `kea serve` and CLI both call `NewApp`; WAL handles this but reconcile TOCTOU was fixed in #127

---

## 6. Recommended Implementation Order

1. **Foundation:** Add `kea serve` command + minimal router + error middleware + graceful shutdown wiring
2. **Read-only endpoints:** Accounts (list, get, tree, balance) — lowest risk, exercises service layer
3. **Read-only transactions:** List with filtering, get detail
4. **Reports:** Income statement, balance sheet, net worth — already API-ready
5. **Write operations:** Create/update/delete for accounts and transactions
6. **Reconciliation flow:** Multi-step endpoints
7. **React SPA:** Build incrementally against the running API

---

## 7. Files to Read for Context in a Fresh Session

When starting the web layer design discussion, the new session should read:
- `CLAUDE.md` (project conventions; already mentions planned Web Layer)
- `internal/service/account_service.go` and `account_ops.go`
- `internal/service/transaction_service.go`, `transaction_ops.go`, `transaction_classifier.go`
- `internal/service/errors.go` (for HTTP status mapping)
- `internal/model/` (all files — types, input structs, pagination, transaction_filter)
- `internal/config/config.go` (ServerConfig is there)
- `cmd/root.go` (to see signal handling and entry point pattern)

---

## 8. Known Limitations (acknowledged, not blockers)

- **Ledger switch via CLI is invisible to running web server** (#119 documented this) — for local single-user use, restart server after switch.
- **No auth** — local-only design means no login flow needed. If remote access is ever wanted, this becomes a major change.
- **Single SQLite file** — WAL + 4 connections is fine for local single-user. Multi-tenant would need a different DB.
- **Backup runs at every `NewApp`** — CLI commands run during server uptime will trigger raw-file backup; the WAL-aware online backup path now used when DB is open (#80, #111 fixed).

---

**End of summary.** Hand this file to a fresh session along with the request "let's design the web layer" — that session will have everything it needs to start brainstorming the architecture.
