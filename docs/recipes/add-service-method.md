# Recipe: Add a service method

> **Read this when:** you need new business logic (an operation, rule, query, or report) behind the service layer, optionally with new repository methods and errors.
>
> **Related:** [architecture.md](../architecture.md), [domain.md](../domain.md), [development.md](../development.md), [http-api.md](../http-api.md), [add-migration.md](add-migration.md), [add-api-endpoint.md](add-api-endpoint.md), [add-cli-command.md](add-cli-command.md)

## When to use
- A new rule, operation, or computed view must live in `internal/service`, so CLI, TUI and HTTP all share it.
- An existing method needs a new input field or a new validation.
- Do NOT use this for a schema change alone; see [add-migration.md](add-migration.md).
- Do NOT use this to expose a method over HTTP or the CLI; see [add-api-endpoint.md](add-api-endpoint.md) and [add-cli-command.md](add-cli-command.md).

## Steps
1. Choose the service and file.
   - Account logic goes on `AccountService`; transaction, report and reconcile logic on `TransactionService`. Both are reached via `service.Service` in `internal/service/service.go`.
   - Mutating operations: `account_ops.go` or `transaction_ops.go`.
   - Pure invariant checks: `account_validation.go` or `transaction_validation.go` (e.g. `ValidateRegular`).
   - Reports: `report_service.go`. Reconcile flow: `reconcile_ops.go`. Type inference: `transaction_classifier.go`.
2. Add input types in `internal/model/input.go` if the method takes more than a couple of arguments.
   - Examples: `model.CreateTransactionInput`, `model.UpdateTransactionInput`. Keep `internal/model` free of logic.
3. Add the repository method if the service needs new data access.
   - Declare it in `internal/repository/interfaces.go` on `AccountRepository` or `TransactionRepository`, with `ctx context.Context` first.
   - Implement it in `internal/store/sqlite_account.go`, `sqlite_transaction.go` or `sqlite_reconcile.go`. Use only `*Context` database methods.
   - Add the same method to the matching mock in `internal/service/testhelper_test.go` (`mockAccountRepo` / `mockTransactionRepo`), with an injectable error field or map.
   - A schema change comes first; see [add-migration.md](add-migration.md).
4. Write the service method.
   - Validate input first and return `validationErrorf(field, ...)` for user mistakes. See `UpdateTransactionComplete` in `internal/service/transaction_ops.go`.
   - Load the record and translate repository errors (see Conventions).
   - Guard mutations of transactions with `ErrReconciled` (see Conventions).
   - Put multi-step writes inside `ts.tm.ExecTx(ctx, func(repo repository.Repository) error {...})`. Example: `CreateTransaction`.
5. Add new sentinel errors to `internal/service/errors.go` only when callers must distinguish them.
   - Then map them in `mapError` in `internal/api/errors.go`; see [add-api-endpoint.md](add-api-endpoint.md).
6. Write the tests (see Conventions) and update the docs in the same commit.

## Worked example
The `Regular` attribute on Income/Expense transactions was added as a series of small commits. Copy the order.
- `e2275aa` — model field: `Regular *bool` on the transaction structs and `internal/model/input.go`; types only, no logic.
- `a8a87aa` — new sentinels `ErrRegularRequired` and `ErrRegularNotApplicable` in `internal/service/errors.go`, before any code returns them.
- `14ecedc` — `ValidateRegular` in `internal/service/transaction_validation.go`, a pure function with its own table-style tests.
- `ae1e9d3` — repository signature change (`UpdateTransactionBasic` gains `regular`) plus interface, store stub and mock; defaulting in create/update; tests for each path.
- `8303a63` — store persists and filters the column in every query in `internal/store/sqlite_transaction.go`; fixtures updated.
- `88e8472` — propagates the field to `TransactionListItem` in `internal/service/transaction_classifier.go`.

## Conventions
- Errors:
  - Wrap with `%w`; callers check with `errors.Is` / `errors.As`.
  - Service code never returns `repository.ErrNotFound` or `repository.ErrAlreadyExists` as is. Translate them: `fmt.Errorf("transaction #%d: %w", id, ErrNotFound)`, or `ErrAlreadyExists` as in `CreateTransaction`.
  - Why: `mapError` in `internal/api/errors.go` keys only on `service` errors and `*service.ValidationError`. Anything else becomes HTTP 500.
  - Store sentinels in `internal/store/errors.go` wrap the `internal/repository/errors.go` ones, so `errors.Is` on the repository error works.
  - The error table is in [domain.md](../domain.md).
- `ErrReconciled`: any method that edits or deletes a transaction loads it first and returns `ErrReconciled` if `Status == model.StatusReconciled`. See `UpdateTransactionComplete`; protected records are described in [domain.md](../domain.md).
- `ExecTx`: use it when two or more writes must succeed together.
  - Use the `repo` argument inside `fn`, never `ts.txRepo` or `ts.accRepo`, or the write leaves the transaction.
  - Do not call `ExecTx` again inside `fn`; nesting fails (see [architecture.md](../architecture.md)).
  - Return an error to roll back.
- Amounts are cents (`int64`); splits must sum to zero (`ValidateSplitsBalance`). See [domain.md](../domain.md).
- Service tests are white-box (`package service`), with no database.
  - Build with `newTestTransactionService` / `newTestAccountService` and the hand-written mocks in `internal/service/testhelper_test.go`.
  - Inject failures through the mock's error fields (`createErr`, `getByIDErr`) and assert interactions through recorders (`deleteSplitCalls`, `bulkUpdateCalls`).
  - `mockTransactionManager` snapshots state and restores it when `fn` errors, so rollback is testable; `failTx` forces `ExecTx` to fail.
  - Examples: `TestCreateTransaction_RegularRejectedForTransfer` in `internal/service/transaction_ops_test.go`.
- Store tests for a new repository method use `setupTestDB` (`internal/store/sqlite_account_test.go`), which opens a fully migrated DB. Model: `internal/store/sqlite_transaction_test.go`. See [development.md](../development.md).

## Checklist
- [ ] Service tests: happy path, each validation error, `ErrNotFound`, `ErrReconciled` (if it mutates), and repository failure
- [ ] Rollback test if the method uses `ExecTx`
- [ ] Store test for each new or changed repository method
- [ ] New sentinels are mapped in `internal/api/errors.go` (see [add-api-endpoint.md](add-api-endpoint.md))
- [ ] Update the matching doc (domain.md / http-api.md / …) in the same commit
- [ ] `go test ./...` passes
