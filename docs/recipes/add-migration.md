# Recipe: Add a database migration

> **Read this when:** you need to change the SQLite schema (new column, constraint, index) or backfill existing rows.
>
> **Related:** [architecture.md](../architecture.md), [domain.md](../domain.md), [development.md](../development.md), [add-service-method.md](add-service-method.md), [add-api-endpoint.md](add-api-endpoint.md)

## When to use
- A table or column must be added, changed, or constrained.
- Existing rows must be rewritten (backfill).
- Do NOT use this for a new query, filter, or business rule on the current schema; see [add-service-method.md](add-service-method.md).

## Steps
1. Pick the next number.
   - List `migrations/` and take the highest `NNNN` plus one (currently `0011_add_transaction_regular`).
   - Create an up and a down file named like the existing pairs, e.g. `migrations/0011_add_transaction_regular.up.sql` and `migrations/0011_add_transaction_regular.down.sql`.
   - Every migration has a down file.
2. Write the up file. Choose the form by what SQLite allows.
   - Additive change (`ALTER TABLE ... ADD COLUMN`, `CREATE INDEX`): plain statements, as in `migrations/0002_add_external_id.up.sql` and `migrations/0005_add_transaction_type.up.sql`.
   - Anything else (add a CHECK, drop a column, change constraints): rebuild the table. Imitate `migrations/0011_add_transaction_regular.up.sql`; another example is `migrations/0007_add_account_type_check.up.sql`.
   - The skeleton:
     ```sql
     PRAGMA foreign_keys = OFF;
     BEGIN;
     CREATE TABLE x_new (...full desired schema...);
     INSERT INTO x_new (...) SELECT ... FROM x;
     DROP TABLE x;
     ALTER TABLE x_new RENAME TO x;
     CREATE INDEX IF NOT EXISTS ...;  -- recreate every index
     COMMIT;
     PRAGMA foreign_keys = ON;
     ```
   - If the rebuild adds a CHECK that old rows would violate, backfill inside the same `INSERT ... SELECT`. 0011 does this with `CASE WHEN type IN ('Income', 'Expense') THEN 1 ELSE NULL END`.
3. Put any other backfill in its own migration, with `UPDATE` statements only.
   - Example pair: `migrations/0005_add_transaction_type.up.sql` adds the column, then `migrations/0006_backfill_transaction_type.up.sql` fills it (correlated subquery over `splits` and `accounts`).
   - Follow-ups `migrations/0009_backfill_investment_type.up.sql` and `migrations/0010_backfill_investment_type_expense.up.sql` fix rows earlier backfills missed.
4. Write the down file so it restores the previous schema.
   - A rebuild's down is another rebuild without the new column: `migrations/0011_add_transaction_regular.down.sql`. The downs of 0002 and 0003 also rebuild the table, so follow that pattern rather than `DROP COLUMN`.
   - A backfill's down may be lossy by design: `migrations/0006_backfill_transaction_type.down.sql` resets the column; the downs of 0009 and 0010 say they are best-effort reclassifications.
5. Add a store test in `internal/store/`, named after the migration number like `migration_0011_test.go` (package `store_test`); see Testing below.
6. Update the Go side so code reads and writes the new column.
   - Struct in `internal/model` (e.g. `Regular` on `model.Transaction`).
   - INSERT, UPDATE and scan code in `internal/store/sqlite_*.go`.
   - Column lists are repeated: `internal/store/sqlite_account.go` spells out the account columns in several SELECTs.
   - `internal/store/sqlite_transaction.go` mentions `regular` in many places.
   - Search for the existing column names and update every hit.
   - Changing a repository method signature means updating `internal/repository/interfaces.go` and the mocks in `internal/service/testhelper_test.go`.
   - For the service and API layers follow [add-service-method.md](add-service-method.md) and [add-api-endpoint.md](add-api-endpoint.md).
7. Update the matching doc (field rules in [domain.md](../domain.md), JSON shape in [http-api.md](../http-api.md)).

## How migrations run
- `migrations/embed.go` embeds `*.sql` as `migrations.FS`. `cmd/kea/main.go` passes it to `cmd.Execute`, which hands it to `app.NewApp`, the `ledger` commands and `kea serve`.
- `store.NewStore` (`internal/store/sqlite.go`) calls `runMigrations` on every open. That runs golang-migrate `Up()` with the iofs source and ignores `migrate.ErrNoChange`.
- `Store.Swap` migrates a newly selected ledger the same way. `app.InitLedgerDB` (`internal/app/app.go`) also opens a store, so `ledger add` and `Server.handleCreateLedger` migrate new ledgers.
- A new migration reaches users on their next start, with no manual step; it is applied to each ledger DB when that DB is opened.
- `app.NewApp` runs `backup.Run` (`internal/backup`) before opening the store, so a pre-migration copy exists. See [architecture.md](../architecture.md).
- The driver is created with `NoTxWrap: true`, so golang-migrate does not wrap files in a transaction. A multi-statement rebuild must carry its own `BEGIN;` / `COMMIT;`, as 0007 and 0011 do.
- `setupTestDB` in `internal/store/sqlite_account_test.go` opens a store with `migrations.FS`, so every test DB is fully migrated (see [development.md](../development.md)).

## Testing
- Schema and constraints: insert through the store or raw SQL on `s.DB()` and assert accepted and rejected rows. `TestMigration0011_CheckConstraint` in `internal/store/migration_0011_test.go` is the model.
- `TestMigration0011_BackfillRegular` inserts rows after migrating, so it is a round-trip test, not a model for existing-row backfills.
- Backfills of existing rows: `TestMigration0009_BackfillInvestmentType` in `internal/store/migration_0009_test.go` is the model.
  - The DB is already migrated, so first reshape rows into their pre-migration state.
  - Take a single connection with `s.DB().Conn(ctx)` and run `PRAGMA ignore_check_constraints = 1` on it; the pragma is per connection. Run the `UPDATE`s, then set the pragma back to 0.
  - Re-run the migration: `migrations.FS.ReadFile("<file>.up.sql")` then `s.DB().ExecContext`.
  - Assert changed rows and untouched rows.

## Worked example
- `5bc4e65` - adds `transactions.regular` plus a CHECK through a full table rebuild (up and down). Model for any constraint change on an existing table.
- `b613312` - adds `internal/store/migration_0011_test.go`: a round-trip test and a CHECK-violation test, written before the store passed the field (TDD).
- `277ad17` - fixes the CHECK so NULL is rejected for Income/Expense (`regular IS NOT NULL` guard). Lesson: SQL `NULL IN (0, 1)` is NULL, and SQLite treats a NULL CHECK result as not violated, so always test the NULL case.

## Conventions
- Name: `NNNN_snake_case_description.{up,down}.sql`, zero-padded to four digits, sequential. Observed verbs include `create`, `add` and `backfill`; they are examples, not a closed set.
- Never edit a shipped migration; add a new one. `277ad17` edited 0011 in place only because it landed before any store code used the column.
- A rebuild must copy every existing column and recreate every index and constraint, or they are silently lost.
- Guard backfill `UPDATE`s with `WHERE` clauses. golang-migrate runs each file once per DB, but tests re-run the SQL on a populated DB; `migrations/0010_backfill_investment_type_expense.up.sql` guards with `type NOT IN ('Investment', 'Opening')`.
- Test files: `internal/store/` `migration_` plus the number plus `_test.go`; test names `TestMigration0011_<What>` style.
- A failed migration returns an error from `runMigrations` (wrapped with `%w`) and aborts opening the store. Nothing in the code repairs a dirty state, so exercise both files before committing.
- Amounts stay int64 cents; see [domain.md](../domain.md).

## Checklist
- [ ] Both `.up.sql` and `.down.sql` exist with the next number
- [ ] Store test covers new rows, existing-row backfill, and CHECK violations including NULL
- [ ] Rebuild keeps all columns, indexes and the unique `external_id` index
- [ ] Model and store INSERT/UPDATE/scan code updated for the new column
- [ ] Update the matching doc (domain.md / http-api.md / …) in the same commit
- [ ] `go test ./...` passes
