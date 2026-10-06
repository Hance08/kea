# HTTP API

> **Read this when:** you call, change, or add an endpoint served by `kea serve`, or wire the SPA to the backend.
>
> **Related:** [recipes/add-api-endpoint.md](recipes/add-api-endpoint.md) · [architecture.md](architecture.md)

The API lives in `internal/api/`. Routes are registered in one place, `internal/api/router.go` (`routes`). For the request flow and middleware chain see [architecture.md](architecture.md); this page covers the wire contract.

## Conventions

- **Base path:** every endpoint is under `/api`. Any other path falls through to the embedded SPA handler (`internal/api/spa.go`, `spaHandler`), which serves static files and falls back to `index.html`; it is not part of the API.
- **Handlers:** each handler is `func(w, r) error` wrapped by `apiHandler` (`internal/api/handler.go`); a returned error goes through `writeError` (see [Errors](#errors)).
- **Bodies:** JSON in and out (`writeJSON`, `decodeJSON` in `internal/api/handler.go`). Request bodies reject unknown fields (`DisallowUnknownFields`); any decode failure is a 400 `validation_failed` with `field: "body"`.
- **Amounts:** integer cents (`int64`), never strings or decimals, in every request and response (`amount`, `statement_balance`, `difference`, `balance`, ...). See [domain.md](domain.md).
- **Timestamps:** Unix seconds (`int64`) for `timestamp`, `start_time`, `end_time`, `as_of`, `at`. The only date strings are the report params `month` (`YYYY-MM`), `from` and `to` (`YYYY-MM-DD`), parsed in server local time.
- **Enums:** account `type` is one of `A L C R E`. Transaction `status` serializes as `"Pending"`, `"Cleared"` or `"Reconciled"`; on input the numeric form is also accepted (`internal/model/types.go`, `TransactionStatus.UnmarshalJSON`).
- **Lists:** paginated lists return `{"items": [...], "total_count": n, "limit": n, "offset": n}` (`model.ListResult`).
- **Delete responses:** account and transaction deletes return `{"deleted": true, "id": N}`; ledger deletes return `{"deleted": true, "name": "..."}`. All are 200, not 204.
- **Active ledger:** there is no per-request ledger selection (no header or query param). The server holds one active ledger for all requests; `POST /api/ledgers/switch` and an external `kea ledger switch` (picked up by the registry watcher started in `cmd/serve.go`) change it for everyone.
- **CORS:** `corsMiddleware` (`internal/api/middleware.go`) only answers origins listed in `server.cors_origins` (default `http://localhost:5173`, the Vite dev server). Allowed methods are `GET, POST, PATCH, DELETE, OPTIONS`, allowed header `Content-Type`; preflight returns 204. The SPA in production is same-origin and needs no CORS.
- **Auth:** none. The server is intended for local use; do not expose it on an untrusted network.
- **Request ID:** every response carries an `X-Request-ID` header (8 hex chars), also attached to log lines.
- **SPA client:** `spa/src/lib/api.ts` (`apiFetch`) calls relative `/api/...` paths and turns non-2xx responses into `ApiError` using `message`, `field` and `difference` from the error body. Per-resource wrappers sit beside it in `spa/src/lib/`.

## Endpoints

Handlers are methods on `Server` in `internal/api/`. "Service" is the call on `svc.Account()` / `svc.Transaction()` unless noted.

### Health

| Method | Path | Handler | Service call | Notes |
|---|---|---|---|---|
| GET | `/api/health` | `health.go` `handleHealth` | none | `{"status":"ok"}` |
| GET | `/api/version` | `health.go` `handleVersion` | none | `{"version":"..."}`; `api.Version`, `dev` unless set via ldflags |

### Config

| Method | Path | Handler | Service call | Notes |
|---|---|---|---|---|
| GET | `/api/config` | `config.go` `handleGetConfig` | `svc.Config()` | `{"defaults":{"currency"},"display":{"hide_decimals"}}` |
| PATCH | `/api/config` | `config.go` `handlePatchConfig` | `svc.Config()` + `saveConfig` | Body `{"display":{"hide_decimals":bool}}`, required; persists to the config file; returns the new config |

### Ledgers

| Method | Path | Handler | Service call | Notes |
|---|---|---|---|---|
| GET | `/api/ledgers/active` | `ledgers.go` `handleActiveLedger` | `registry.ActiveName`, `EntryFor` | `{name, path, active}`; 404 `no_active_ledger` if none |
| GET | `/api/ledgers` | `ledgers.go` `handleListLedgers` | `registry.Names`, `EntryFor` | `{"active": name, "items": [{name, path, active}]}` (not `ListResult`) |
| POST | `/api/ledgers` | `ledgers.go` `handleCreateLedger` | `app.InitLedgerDB`, `registry.Add` | Body `{name}`; creates `<appDir>/ledgers/<name>.db`, runs migrations, does not activate; 201 |
| POST | `/api/ledgers/switch` | `ledgers.go` `handleSwitchLedger` | `Server.switchLedger` (`app.SwitchLedger`) | Body `{name}`; returns the ledger info with `active: true` |
| DELETE | `/api/ledgers/{name}` | `ledgers.go` `handleDeleteLedger` | `registry.Remove(name, false)` | Unregisters only (the `false` flag); 409 `cannot_remove_active` for the active ledger |

Ledger names are checked by `validateLedgerName`: non-empty, no `/`, `\`, `..` or control characters.

### Accounts

| Method | Path | Handler | Service call | Notes |
|---|---|---|---|---|
| GET | `/api/accounts` | `accounts.go` `handleListAccounts` | `SearchAccounts` or `ListAccounts` | Search path used when `q` or `currency` is set, `limit > 0`, `offset > 0`, or `include_count` is true (`limit=0` still uses `ListAccounts`); otherwise `ListAccounts` wrapped as a `ListResult` |
| POST | `/api/accounts` | `accounts_write.go` `handleCreateAccount` | `CreateAccountWithBalance` | Body `model.CreateAccountInput` (`name, type, currency, description, parent_id, balance`); 201 with the account |
| GET | `/api/accounts/tree` | `accounts.go` `handleAccountTree` | `GetAccountTree` | Array of `{account, children}` nodes; `include_hidden` |
| GET | `/api/accounts/by-name` | `accounts.go` `handleAccountByName` | `GetAccountByName` | Required `name` query param (full colon path) |
| GET | `/api/accounts/{id}` | `accounts.go` `handleAccountByID` | `GetAccountByID` | |
| PATCH | `/api/accounts/{id}` | `accounts_write.go` `handleUpdateAccount` | `RenameAccount`, `UpdateAccountMetadata` | Body fields `name`, `description`, `is_hidden`, all optional but at least one; rename and metadata are separate service calls, not one transaction |
| DELETE | `/api/accounts/{id}` | `accounts_write.go` `handleDeleteAccount` | `DeleteAccountByName` | Looks up the name by id first |
| GET | `/api/accounts/{id}/balance` | `accounts.go` `handleAccountBalance` | `GetAccountByID`, `GetAccountBalance` | `{account_id, amount, currency}`; empty account currency falls back to `defaults.currency` |
| GET | `/api/balances` | `accounts.go` `handleListBalances` | `GetAccountBalancesBulk` | `as_of` (default end of today UTC), `include_hidden`; `ListResult` of `{account_id, name, type, parent_id, currency, amount, is_hidden}` |
| GET | `/api/balances/history` | `reports.go` `handleBalanceHistory` | `Transaction().GetMonthlyBalanceHistory` | `{"items":[...]}` monthly series per account; no params |

### Transactions

| Method | Path | Handler | Service call | Notes |
|---|---|---|---|---|
| GET | `/api/transactions` | `transactions.go` `handleListTransactions` | `FilterTransactions`, `GetTransactionDetailsByIDs` | `ListResult` of full transaction details (with splits); filters below |
| POST | `/api/transactions` | `transactions_write.go` `handleCreateTransaction` | `CreateTransactionFromSplits`, `GetTransactionByID` | Body `model.CreateTransactionFromSplitsInput`; splits keyed by `account_name` (see below); 201 with full detail |
| GET | `/api/transactions/{id}` | `transactions.go` `handleTransactionByID` | `GetTransactionByID` | |
| DELETE | `/api/transactions/{id}` | `transactions_write.go` `handleDeleteTransaction` | `DeleteTransaction` | 409 `reconciled` if reconciled |
| PATCH | `/api/transactions/{id}` | `transactions_write.go` `handleUpdateTransaction` | `UpdateTransactionComplete`, `GetTransactionByID` | Full replace despite PATCH: body is `model.UpdateTransactionInput`; splits keyed by `account_id` (see below); id comes from the path |
| PATCH | `/api/transactions/{id}/status` | `transactions_write.go` `handleUpdateTransactionStatus` | `UpdateTransactionStatus`, `GetTransactionByID` | Body `{"status": "Pending"\|"Cleared"}`; `Reconciled` is 400 on `status`, reconciling only happens via the reconcile endpoints; 409 `reconciled` if already reconciled |

Both create and update bodies use `model.SplitDetail` for splits, but read different keys:

| | Create (`POST`, `CreateTransaction`) | Update (`PATCH`, `UpdateTransactionComplete`) |
|---|---|---|
| Account | `account_name` (full colon path, resolved by `GetAccountByName`); `account_id` is ignored | `account_id` |
| Currency | Ignored; the account's currency is used (else `defaults.currency`) | `currency` is stored as sent |
| Split `id` | Ignored | Non-zero keeps and updates that existing split (must belong to the transaction, no duplicates); `0` or omitted creates a new split; existing splits not listed are deleted |
| Other split keys | `amount`, `memo` | `amount`, `memo` |
| `timestamp` | `0` or omitted means now | Stored as sent |
| `status` | `Pending` or `Cleared` | `Pending` or `Cleared` |

Rules shared by both: at least 2 splits, splits sum to zero, `description` required (trimmed), and splits must fit the `type` (`ValidateSplitsMatchType`; see [domain.md](domain.md#transaction-types)). Create also requires `type`. `regular` defaults to true for Income/Expense (see [domain.md](domain.md#regular-attribute)). Responses return full `SplitDetail`: `id`, `account_id`, `account_name`, `account_type`, `amount`, `currency`, `memo`. Positive and negative amounts follow [domain.md](domain.md).

Example create request (expense of 5.00):

```json
{"description": "Coffee", "type": "Expense", "status": "Pending",
 "splits": [
   {"account_name": "Assets:Bank", "amount": -500},
   {"account_name": "Expenses:Coffee", "amount": 500}
 ]}
```

### Reconcile

| Method | Path | Handler | Service call | Notes |
|---|---|---|---|---|
| GET | `/api/accounts/{id}/unreconciled` | `reconcile.go` `handleListUnreconciled` | `GetUnreconciledByAccount` | `{"entries":[...],"last_reconciled_balance":n}`; entries never null |
| POST | `/api/accounts/{id}/reconcile/preview` | `reconcile.go` `handleReconcilePreview` | `PreviewReconcile` | Body `{statement_balance, transaction_ids}`; returns `{"difference":n}`, read-only |
| POST | `/api/accounts/{id}/reconcile` | `reconcile.go` `handleReconcileCommit` | `PreviewReconcile`, `ReconcileTransactions` | Body adds `allow_mismatch` (default false); returns `{reconciled_count, difference, last_reconciled_balance}`; non-zero diff without `allow_mismatch` is 409 `balance_mismatch` and writes nothing |

### Budgets

| Method | Path | Handler | Service call | Notes |
|---|---|---|---|---|
| GET | `/api/budgets` | `budgets.go` `handleListBudgets` | `ListBudgets` | `{"items":[Budget]}`; items never null |
| PUT | `/api/budgets` | `budgets.go` `handleSetBudget` | `SetBudget` | Body `{account_name, effective_month, amount}` (`YYYY-MM`, cents); idempotent upsert; returns `Budget` |
| POST | `/api/budgets/stop` | `budgets.go` `handleStopBudget` | `StopBudget` | Body `{account_name, effective_month}`; returns the stop `Budget` |
| DELETE | `/api/budgets/{id}` | `budgets.go` `handleDeleteBudget` | `DeleteBudget` | Returns `{"deleted":true,"id":n}` |

Errors: 400 `validation_failed` with `field` one of `account_name`, `effective_month`, `amount`, `body` (unknown JSON field or bad JSON); 404 `not_found` for an unknown account or budget id.

### Savings targets

| Method | Path | Handler | Service call | Notes |
|---|---|---|---|---|
| GET | `/api/savings-targets` | `savings.go` `handleListSavingsTargets` | `ListSavingsTargets` | `{"items":[SavingsTarget]}`; items never null |
| PUT | `/api/savings-targets` | `savings.go` `handleSetSavingsTarget` | `SetSavingsTarget` | Body `{account_name, effective_month, amount}` (`YYYY-MM`, cents); Asset accounts only; idempotent upsert; returns `SavingsTarget` |
| POST | `/api/savings-targets/stop` | `savings.go` `handleStopSavingsTarget` | `StopSavingsTarget` | Body `{account_name, effective_month}`; returns the stop `SavingsTarget` |
| DELETE | `/api/savings-targets/{id}` | `savings.go` `handleDeleteSavingsTarget` | `DeleteSavingsTarget` | Returns `{"deleted":true,"id":n}` |

Errors: 400 `validation_failed` with `field` one of `account_name`, `effective_month`, `amount`, `body` (unknown JSON field or bad JSON); 404 `not_found` for an unknown account or target id.

### Reports

| Method | Path | Handler | Service call | Notes |
|---|---|---|---|---|
| GET | `/api/reports/income-statement` | `reports.go` `handleIncomeStatement` | `GenerateFullIncomeStatement` | Date range params |
| GET | `/api/reports/income-breakdown` | `reports.go` `handleIncomeBreakdown` | `GenerateFullIncomeBreakdown` | Date range params |
| GET | `/api/reports/expense-breakdown` | `reports.go` `handleExpenseBreakdown` | `GenerateFullExpenseBreakdown` | Date range params |
| GET | `/api/reports/balance-sheet` | `reports.go` `handleBalanceSheet` | `GenerateBalanceSheet` | `as_of` Unix seconds, default now |
| GET | `/api/reports/net-worth` | `reports.go` `handleNetWorth` | `GetNetWorthAt` | `at` Unix seconds, default now; `{"at":n,"net_worth":{"<CCY>":cents}}` |
| GET | `/api/reports/net-worth-series` | `reports.go` `handleNetWorthSeries` | `GetDailyNetWorthSeries` | `{"items":[...]}` daily series per currency; no params |
| GET | `/api/reports/budget` | `budgets.go` `handleBudgetReport` | `GenerateBudgetReport` | `month` `YYYY-MM`, default current local month; returns `BudgetReport` (fields in `internal/model/budget.go`); totals per currency over budgeted rows with no budgeted ancestor in the same currency; bad month is 400 with `field` `month` |
| GET | `/api/reports/savings` | `savings.go` `handleSavingsReport` | `GenerateSavingsReport` | `month` `YYYY-MM`, default current local month; returns `SavingsReport` (fields in `internal/model/savings.go`): per target saved (signed balance change of the account and its descendants, `Opening` transactions excluded), remaining, and year-to-date over active months; totals per currency over rows with no targeted ancestor in the same currency; bad month is 400 with `field` `month` |

## Query parameters

Parsers are in `internal/api/params.go`. A missing or empty value means "not set". Path ids use `parseInt64Path`; a non-integer id is 400 with `field` set to the path key (`id`).

| Param | Used by | Rules |
|---|---|---|
| `limit`, `offset` | accounts, transactions | Integer, `>= 0`; `0`/absent limit means no limit (`parseListOptions`) |
| `include_count` | accounts, transactions | `true`/`false`/`1`/`0` (case-insensitive); other values are 400 |
| `include_hidden` | accounts, tree, balances | Same boolean rules; hidden accounts excluded by default |
| `q`, `currency`, `type` | accounts | `q` text search, `currency` exact; `type` must be `A`, `L`, `C`, `R` or `E` (`parseAccountFilter`) |
| `account_id` | transactions | Integer |
| `type` | transactions | One of `Expense, Income, Transfer, Opening, Deposit, Withdrawal, Investment, Other` (`TransactionType.IsValid()`; the error message in `internal/api/params.go` still omits `Investment`) |
| `status` | transactions | `Pending`, `Cleared` or `Reconciled` |
| `start_time`, `end_time` | transactions | Unix seconds; `end_time < start_time` is 400 on `end_time` |
| `description` | transactions | Text filter |
| `regular` | transactions | `true`/`false`/`1`/`0` (`parseTransactionFilter`) |
| `as_of` | balances, balance-sheet | Unix seconds; balances default to 23:59:59 UTC today (`endOfTodayUTC`), balance-sheet to now |
| `at` | net-worth | Unix seconds, default now |
| `month`, `from`, `to` | income-statement, income-breakdown, expense-breakdown | `month` is `YYYY-MM` and wins over `from`/`to`; `from`/`to` are `YYYY-MM-DD` and either may be omitted (open start = epoch, open end = now); with none, the current month. Format errors are 400 with `field` `month`, `from` or `to` (`service.DateRangeParams`, `ResolveDateRange` in `internal/service/report_service.go`) |

## Errors

Every error is JSON from `writeError` / `mapError` (`internal/api/errors.go`):

```json
{"error": "validation_failed", "message": "name is required", "field": "name"}
```

`field` is omitted when empty. `balance_mismatch` adds `difference` (cents) instead:

```json
{"error": "balance_mismatch", "message": "statement balance off by 150", "difference": 150}
```

| Error | HTTP | `error` code |
|---|---|---|
| `*service.ValidationError` (bad input, bad JSON, bad params) | 400 | `validation_failed` |
| `*balanceMismatchError` (reconcile commit) | 409 | `balance_mismatch` |
| `service.ErrNotFound` | 404 | `not_found` |
| `ledger.ErrLedgerNotFound` | 404 | `not_found` |
| `ledger.ErrNoActiveLedger` | 404 | `no_active_ledger` |
| `service.ErrAlreadyExists`, `ledger.ErrLedgerExists` | 409 | `already_exists` |
| `service.ErrReconciled` | 409 | `reconciled` |
| `service.ErrCircularParent` | 409 | `circular_parent` |
| `ledger.ErrRemoveActive` | 409 | `cannot_remove_active` |
| `service.ErrNotEditable` | 403 | `not_editable` |
| anything else | 500 | `internal` (message is always `internal server error`; details only in logs) |

Unmapped sentinels: `service.ErrRegularRequired` and `service.ErrRegularNotApplicable` (`internal/service/errors.go`) have no case in `mapError`, so they return 500 `internal`. `ErrRegularRequired` is not reachable over HTTP because create and update default `regular` to true for Income/Expense before validating. `ErrRegularNotApplicable` is reachable: `POST /api/transactions` with `regular` set on a non-Income/Expense type returns 500 rather than 400.
