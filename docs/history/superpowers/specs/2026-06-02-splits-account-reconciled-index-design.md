# Splits Composite Index for Reconcile Queries

**Issue:** #132
**Date:** 2026-06-02
**Branch:** `fix/issue-132-splits-composite-index`

## Problem

`GetUnreconciledTransactionsByAccount` in [internal/store/sqlite_reconcile.go:47-48](../../../internal/store/sqlite_reconcile.go) filters on `s.account_id = ? AND s.reconciled = 0`. The existing indexes are:

- `idx_splits_account_id` — `splits(account_id)` (from migration 0001)
- `idx_splits_reconciled` — `splits(reconciled)` (from migration 0003)

SQLite uses at most one index per table scan. With these two single-column indexes it picks one (`idx_splits_account_id` in practice) and then evaluates `reconciled = 0` as a row-by-row filter. Under API load with frequent reconcile queries on a ledger with many splits, this becomes wasteful.

## Approach

Add a new migration (`0008`) that creates a composite index on `splits(account_id, reconciled)`. SQLite can then service the full `WHERE` clause from the index without any post-filtering.

The existing single-column indexes are **left in place**:

- `idx_splits_account_id` — covered by the leading column of the composite, so technically redundant, but verifying every `WHERE account_id = ?` call site is out of scope for this fix. Cost of keeping it: a small write/storage overhead.
- `idx_splits_reconciled` — low cardinality (binary), unlikely to be picked by the planner anyway. Removing it is its own optimization decision; a follow-up issue can address index pruning holistically.

## Files Added

### `migrations/0008_add_splits_account_reconciled_index.up.sql`

```sql
CREATE INDEX IF NOT EXISTS idx_splits_account_reconciled
    ON splits (account_id, reconciled);
```

### `migrations/0008_add_splits_account_reconciled_index.down.sql`

```sql
DROP INDEX IF EXISTS idx_splits_account_reconciled;
```

Style notes:

- `IF NOT EXISTS` / `IF EXISTS` to match the convention used in `0001` and `0003`.
- Column order is `(account_id, reconciled)`: `account_id` is the high-selectivity column, and querying by `account_id` alone (without `reconciled`) is a common pattern that the leading column still serves.

## Verification

1. `make build` — confirms the embedded `migrations` FS still loads cleanly.
2. `go test ./...` — confirms no regressions; this is a pure-index change with no behavior diff.
3. `EXPLAIN QUERY PLAN` against the reconcile query, before and after, captured for the PR description. Expected change:
   - Before: `SEARCH splits USING INDEX idx_splits_account_id (account_id=?)`
   - After: `SEARCH splits USING INDEX idx_splits_account_reconciled (account_id=? AND reconciled=?)`

## Out of Scope

- Dropping `idx_splits_account_id` and/or `idx_splits_reconciled`. Defer to a separate index-pruning issue once all call sites are audited.
- Changes to `GetUnreconciledTransactionsByAccount` or any Go code. The query is unchanged.
- Benchmarking. The issue documents the problem qualitatively (one-index-per-scan limit); the EXPLAIN output is the acceptance evidence.
