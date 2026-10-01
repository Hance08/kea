# Refactor RenameAccount to use DBTX interface (#110)

## Problem

`Store.RenameAccount` in `internal/store/sqlite_account.go` manages its own
transaction via `s.rawDB.BeginTx()` instead of using the `DBTX` interface
(`s.db`). This prevents the method from participating in `ExecTx` calls,
breaking composability. When called on a transaction-scoped store, `rawDB` is
nil and the method returns an error.

## Approach

**Option A — Use `s.db` directly.** Remove the self-managed transaction and
use `s.db.ExecContext()` for both SQL statements, matching every other store
method. The caller (service layer) owns transaction boundaries via `ExecTx`.

## Store layer changes

File: `internal/store/sqlite_account.go`, method `RenameAccount`

Remove:
- `rawDB` nil check and the "already in a transaction" guard
- `rawDB.BeginTx()`, `tx.Rollback()`, `tx.Commit()`
- Mutex dance around `rawDB`

Replace with:
- `s.mu.RLock()` / `defer s.mu.RUnlock()` (same guard other methods use)
- Two `s.db.ExecContext()` calls: rename self, then cascade descendants
- Same SQL, same error handling, just operating on `s.db` instead of `tx`

## Service layer changes

File: `internal/service/account_ops.go`, method `RenameAccount`

Wrap the mutating calls in `as.tm.ExecTx()`:

```
as.tm.ExecTx(ctx, func(repo repository.Repository) error {
    if err := repo.RenameAccount(ctx, acc.Name, newFullName); err != nil {
        return err
    }
    renamed, err = repo.GetAccountByName(ctx, newFullName)
    return err
})
```

Validation calls (`GetAccountByName`, `AccountExists`) stay outside the
transaction — they are read-only checks. This matches the existing pattern
in `CreateAccountWithOpeningBalance`.

## Testing

### Existing store tests (no changes expected)

- `TestRenameAccount_LIKEWildcardsInName` — verifies cascade correctness
- `TestRenameAccount_DeepNesting` — verifies multi-level cascade
- `TestRenameAccount_SiblingUnaffected` — verifies prefix boundary

These call `RenameAccount` on a top-level store where `s.db` is `*sql.DB`,
so behavior is unchanged.

### New store test

`TestRenameAccount_InsideExecTx` — calls `RenameAccount` inside `ExecTx` to
prove composability works. This is the core validation for issue #110.

### Service tests

Existing mock-based tests in `account_ops_test.go` continue to pass. Verify
that `ExecTx` is called by the rename path (the mock `TransactionManager`
already exists in `testhelper_test.go`).
