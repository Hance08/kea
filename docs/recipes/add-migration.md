# Recipe: Add a database migration

> **Read this when:** you need to change the SQLite schema (new column, constraint, index) or backfill existing rows.
>
> **Related:** [architecture.md](../architecture.md), [domain.md](../domain.md), [development.md](../development.md), [add-service-method.md](add-service-method.md), [add-api-endpoint.md](add-api-endpoint.md)

## When to use
- A table or column must be added, changed, or constrained, or existing rows must be rewritten.
- Existing data needs a one-off correction (backfill). Put it in its own migration, not mixed with the schema change.
- Do NOT use this for a new query, filter, or business rule on the current schema; see [add-service-method.md](add-service-method.md).

## Steps
1. Pick the next number. List `migrations/` and take the highest `NNNN` plus one (currently `0011_add_transaction_regular`). Create both an up and a down file named like the existing pairs (for example `migrations/0011_add_transaction_regular.up.sql` and `migrations/0011_add_transaction_regular.down.sql`); every migration has a down file.
2. Write the up file. Choose the form by what SQLite allows:
   - Simple additive change (`ALTER TABLE ... ADD COLUMN`, `CREATE INDEX`): plain statements, as in `migrations/0002_add_external_id.up.sql` and `migrations/0003_add_split_reconciled.up.sql`.
   - Anything `ALTER TABLE` cannot do (add a CHECK, drop a column, change constraints): rebuild the table. Imitate `migrations/0011_add_transaction_regular.up.sql`: `PRAGMA foreign_keys = OFF;`, `BEGIN;`, create a `transactions_new`-style temp table with the full desired schema, `INSERT ... SELECT` all rows, `DROP TABLE`, `ALTER TABLE ... RENAME`, recreate every index, `COMMIT;`, `PRAGMA foreign_keys = ON;`. Other rebuilds: `migrations/0007_add_account_type_check.up.sql`.
   - Data backfill: `UPDATE` statements only, in a separate migration. Examples: `migrations/0006_backfill_transaction_type.up.sql` (correlated subquery over `splits` and `accounts`) and `migrations/0009_backfill_investment_type.up.sql`.
3. Write the down file so it restores the previous schema. For a rebuild, repeat the rebuild without the new column and recreate the indexes (`migrations/0011_add_transaction_regular.down.sql`). For a pure backfill the down resets the column, as `migrations/0006_backfill_transaction_type.down.sql` does (`UPDATE transactions SET type = '';`).
4. Add a store test next to `internal/store/migration_0011_test.go` (see Conventions) that exercises the new schema through `setupTestDB`, including NULL and invalid-value cases.
5. Update the Go side so code reads and writes the new column: the struct in `internal/model` (e.g. `Regular` on `model.Transaction`), and the INSERT, UPDATE and scan code in `internal/store/sqlite_*.go`. Then thread the field through service and API; follow [add-service-method.md](add-service-method.md) and [add-api-endpoint.md](add-api-endpoint.md).
6. Update the matching doc (field rules in [domain.md](../domain.md), JSON shape in [http-api.md](../http-api.md)).

## How migrations run
- `migrations/embed.go` embeds `*.sql` as `migrations.FS`. `cmd/kea/main.go` passes it to `cmd.Execute`, which hands it to `app.NewApp`, `ledger` commands and `kea serve`.
- `store.NewStore` (`internal/store/sqlite.go`) calls `runMigrations` on every open, which runs golang-migrate `Up()` with the iofs source; `migrate.ErrNoChange` is ignored. `Store.Swap` does the same for a newly selected ledger.
- A new migration therefore reaches users on their next start: no manual step, applied to each ledger DB when it is opened. `app.NewApp` runs `backup.Run` (`internal/backup`) before opening the store, so a pre-migration copy exists. Details in [architecture.md](../architecture.md).
- The driver is created with `NoTxWrap: true`, so golang-migrate does not wrap files in a transaction. A multi-statement rebuild must carry its own `BEGIN;` / `COMMIT;`, as 0007 and 0011 do.
- Tests: `setupTestDB` in `internal/store/sqlite_account_test.go` opens a store with `migrations.FS`, so every test DB is fully migrated.

## Worked example
- `5bc4e65` - adds `transactions.regular` plus a CHECK via a full table rebuild (up and down files). Model for any constraint change on an existing table.
- `b613312` - adds `internal/store/migration_0011_test.go`: a round-trip test through the store and a CHECK-violation test, written before the store passed the field (TDD).
- `277ad17` - fixes the CHECK so NULL is rejected for Income/Expense (`regular IS NOT NULL` guard). Lesson: SQL `NULL IN (0, 1)` is NULL, and SQLite treats a NULL CHECK result as passing, so always test the NULL case.

For a backfill, read `migrations/0006_backfill_transaction_type.up.sql` and its follow-ups `0009` and `0010`.

## Conventions
- File name: `NNNN_snake_case_description.{up,down}.sql`, zero-padded to four digits, sequential, no gaps. The file name is the numbered prefix; verbs used so far: `create`, `add`, `backfill`.
- Never edit a migration that has shipped; add a new one (`migrations/0010_backfill_investment_type_expense.up.sql` fixes rows that 0006 and 0009 missed, per its header comment). The only in-place edit above (`277ad17`) landed before release.
- Keep the new schema in sync across rebuilds: a rebuild must copy every existing column and recreate every index and constraint, or they are silently lost.
- Tests live in `internal/store/` as `migration_NNNN_test.go` (e.g. `migration_0009_test.go`), package `store_test`, test names `TestMigrationNNNN_<What>`; use `setupTestDB` and raw `s.DB()` queries to inspect columns.
- Errors surface from `runMigrations` wrapped with `%w` and abort startup; there is no dirty-state recovery in the code, so test both files before committing.
- Backfill SQL must stay deterministic and idempotent-safe; guard with `WHERE` clauses as 0009 does, since it runs once per ledger DB.
- Amounts stay int64 cents; see [domain.md](../domain.md).

## Checklist
- [ ] Both `.up.sql` and `.down.sql` exist with the next number
- [ ] Store test covers the happy path, existing-row backfill, and CHECK violations including NULL
- [ ] Rebuild keeps all columns, indexes and the unique external_id index
- [ ] Model, store insert/update/scan code updated for the new column
- [ ] Update the matching doc (domain.md / http-api.md / …) in the same commit
- [ ] `go test ./...` passes
