# Recipe: Add an HTTP API endpoint

> **Read this when:** you need to expose a service operation over HTTP for the SPA or another client, including new routes, query params, or error responses.
>
> **Related:** [architecture.md](../architecture.md), [http-api.md](../http-api.md), [development.md](../development.md), [domain.md](../domain.md), [add-service-method.md](add-service-method.md), [add-spa-page.md](add-spa-page.md)

## When to use
- A service method already exists (or you add one first) and an HTTP client needs it.
- An existing endpoint needs a new query param, request field, or error status.
- Do NOT put business logic in a handler; see [add-service-method.md](add-service-method.md).
- Do NOT use this for CLI commands; see [add-cli-command.md](add-cli-command.md).

## Steps
1. Register the route in `internal/api/router.go` (`Server.routes`).
   - Add one `r.Method(http.MethodX, "/path/{id}", apiHandler(s.handleX))` line inside the `/api` route group.
   - Register static paths such as `/accounts/by-name` alongside `/accounts/{id}`; chi prefers the static segment.
   - Example: `/accounts/{id}/unreconciled` -> `handleListUnreconciled`.
2. Write the handler in the matching file in `internal/api/` (for example `reconcile.go`, `transactions_write.go`).
   - Signature: `func (s *Server) handleX(w http.ResponseWriter, r *http.Request) error`. Return errors; never write them yourself.
   - `apiHandler` in `internal/api/handler.go` passes any returned error to `writeError`, which calls `mapError`.
   - Reach the service with `s.svc.Account()` or `s.svc.Transaction()`, and pass `r.Context()`. Do not cache the service or store.
   - Example: `handleListUnreconciled` in `internal/api/reconcile.go` parses the id, checks the account, calls the service, writes JSON.
   - Ledger switching needs no handling: `service.NewService` gets the same `*store.Store` for every repository, and `Store.Swap` replaces its connection in place (see [architecture.md](../architecture.md)).
3. Decode and validate input.
   - Path ids: `parseInt64Path(r, "id")` in `internal/api/params.go`.
   - Query params: `parseInt64Query`, `parseIntQuery`, `parseBoolQuery`, `parseStringQuery`, `parseListOptions`, `parseDateRangeParams`.
   - New filter params go in `parseAccountFilter` or `parseTransactionFilter` and return `*service.ValidationError` on bad input.
   - JSON bodies: define a request struct and call `decodeJSON(r, &req)`. It rejects unknown fields and malformed JSON with a `ValidationError` on field `body`.
   - Example: `reconcilePreviewRequest` and `handleReconcilePreview` in `internal/api/reconcile.go`.
4. Write the response.
   - `writeJSON(w, status, v)` Return the `internal/model` type when it already fits (accounts, transactions, reports); use an API-local struct for composite or derived shapes (`reconcileCommitResponse`, `ledgerInfo`, `configResponse`, `balanceResponse`); either way, snake_case JSON tags.
   - Use 200 for reads and actions, 201 for creates (`handleCreateTransaction`), and 200 with `{"deleted": true, "id": id}` for deletes (`handleDeleteTransaction`).
   - Return empty slices, not nil, so lists encode as `[]` (see `handleListUnreconciled`).
   - The transaction create and update handlers in `internal/api/transactions_write.go` re-read the record with `GetTransactionByID` for the full detail. Account handlers return the service result directly.
   - Use PATCH for updates; no route uses PUT.
5. Map new errors in `mapError` in `internal/api/errors.go`.
   - Add a `case errors.Is(err, service.ErrX)` with a status and a stable `error` code.
   - Skip this and the sentinel returns 500 `internal`. See Conventions.
   - For a response with extra fields, add an API-local error type and an `errors.As` branch before the `switch`, plus an optional field on `errorBody`.
   - Example: the `balanceMismatchError` branch returns 409 `balance_mismatch` with `Difference` set on `errorBody`.
   - A new HTTP method must also be added to the allowed methods in `corsMiddleware` in `internal/api/middleware.go` (now GET, POST, PATCH, DELETE, OPTIONS).
6. Write tests (see Conventions), update [http-api.md](../http-api.md), and add the SPA client code ([add-spa-page.md](add-spa-page.md)).

## Worked example
The reconcile endpoints were added as a series of small commits. Copy the order.
- `61ca0f9` — `balanceMismatchError` plus the `Difference` field on `errorBody` and a `mapError` branch (409 `balance_mismatch`), with `TestMapError_BalanceMismatch`. Land the error contract before any handler uses it.
- `4dbea42` — `GET /api/accounts/{id}/unreconciled`: handler, one route line, and tests. The smallest complete endpoint.
- `b2fa44f` — `POST .../reconcile/preview`: first JSON-body endpoint. It also fixed the service to translate `repository.ErrNotFound`, so unknown ids give 404.
- `c879a26` — `POST .../reconcile`: handler-level gate returning `balanceMismatchError` unless `allow_mismatch` is set, because the service always persists.
- `e002496` — added the missing `reconciled_count` assertion in `TestHandleReconcileCommit_AllowMismatchTrue` and comments documenting the preview-then-commit race. Do this pass before you move on.
- `126a7a0` — new query param `?regular=` in `parseTransactionFilter`, with table tests in `internal/api/params_test.go`.
- `8f9140f` — round-trip tests for a new field through create and update. They exposed a service bug: `GetTransactionByID` dropped the field. Always test the field through the response.

## Conventions
- Handlers stay thin: parse, call one service method, write JSON. Rules belong in `internal/service`; an API-local gate (as in `handleReconcileCommit`) is the exception.
- Error codes, statuses and the 500 fallback are listed in [http-api.md](../http-api.md); `mapError` is the single place to change them.
- Cautionary example: `service.ErrRegularNotApplicable` has no case in `mapError` and surfaces as 500 (see [http-api.md](../http-api.md#errors)); add the case when you add a sentinel.
- Service code must wrap `repository.ErrNotFound` into `service.ErrNotFound`; `mapError` does not know the repository sentinels.
- Input errors are `*service.ValidationError{Field, Message}`, which becomes 400 `validation_failed` with the field name.
- Amounts in request and response bodies are integer cents. See [domain.md](../domain.md).
- Tests live in `package api` and send real HTTP requests to an `httptest.Server`. Helpers are in `internal/api/testhelper_test.go` (see [development.md](../development.md)).
  - Read endpoints: `newServerWithStore(t)` returns the server and `*service.Service`; seed with `seedAccount` and `seedTransaction`.
  - Write endpoints: `newServerForWrite(t)` also returns `*store.Store`, needed for `seedReconciledTransaction` and `injectParentSelfCycle`.
  - Config or ledger endpoints: `newServerForWriteWithCurrency`, `newServerForPatchConfig`, `newTestServerWithLedger`.
  - Model: `internal/api/reconcile_test.go`. Error mapping cases go in `TestMapError` in `internal/api/errors_test.go`.

## Checklist
- [ ] Handler tests: happy path, each validation error (bad id, bad JSON, unknown field, bad param), 404 for a missing record
- [ ] A test for each new `mapError` case, including the status and `error` code
- [ ] Response-field assertions for every field, including nullable ones
- [ ] Route registered in `internal/api/router.go` and SPA client updated (see [add-spa-page.md](add-spa-page.md))
- [ ] Update docs/http-api.md (and any other matching doc) in the same commit
- [ ] `go test ./...` passes
