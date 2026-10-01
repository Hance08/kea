# Splits Composite Index for Reconcile Queries (Issue #132) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a composite SQL index on `splits(account_id, reconciled)` so the reconcile query in `GetUnreconciledTransactionsByAccount` can satisfy its full `WHERE` clause from a single index instead of falling back to one-index-plus-row-filter.

**Architecture:** Pure migration-only change. Add a new migration pair (`0008`) under `migrations/`. The migrations are embedded into the binary via `migrations/embed.go` (`//go:embed *.sql`). The `golang-migrate` runner picks them up automatically by filename order — no Go code changes needed. Existing single-column indexes (`idx_splits_account_id`, `idx_splits_reconciled`) are left alone; pruning is out of scope per the design doc.

**Tech Stack:** SQLite, `golang-migrate` (file-named migrations), Go embed FS.

**Spec:** [docs/superpowers/specs/2026-06-02-splits-account-reconciled-index-design.md](../specs/2026-06-02-splits-account-reconciled-index-design.md)

**Branch:** `fix/issue-132-splits-composite-index` (already created from `master`).

---

## File Structure

- **Create:** `migrations/0008_add_splits_account_reconciled_index.up.sql` — single `CREATE INDEX` statement.
- **Create:** `migrations/0008_add_splits_account_reconciled_index.down.sql` — single `DROP INDEX` statement.

No Go files are modified. No tests are added (this is a pure-performance, behaviour-preserving change). Verification is via `EXPLAIN QUERY PLAN` and the existing test suite (which already exercises `GetUnreconciledTransactionsByAccount` via mocks at the service layer; the migration itself is verified by `make build` embedding succeeding and the test suite continuing to pass).

---

### Task 1: Capture pre-change `EXPLAIN QUERY PLAN` baseline

Capture the query plan **before** the new index is added, so the PR description can show a clear before/after. Use a throwaway SQLite database loaded with all current migrations (`0001`–`0007`).

**Files:**
- None modified. Output captured to scratch (paste into PR description in Task 6).

- [ ] **Step 1: Build a scratch DB with migrations 0001–0007 and capture the plan**

Run the following command from the repo root. It creates an in-memory database, applies all current migration `up.sql` files in numeric order, then runs `EXPLAIN QUERY PLAN` against the reconcile query's predicate.

```bash
sqlite3 :memory: <<'EOF'
.read migrations/0001_create_initial_schema.up.sql
.read migrations/0002_add_external_id.up.sql
.read migrations/0003_add_split_reconciled.up.sql
.read migrations/0004_add_account_reconcile_state.up.sql
.read migrations/0005_add_transaction_type.up.sql
.read migrations/0006_backfill_transaction_type.up.sql
.read migrations/0007_add_account_type_check.up.sql
EXPLAIN QUERY PLAN
SELECT t.id, SUM(s.amount)
FROM transactions t
INNER JOIN splits s ON t.id = s.transaction_id
WHERE s.account_id = 1 AND s.reconciled = 0
GROUP BY t.id;
EOF
```

Expected: output mentions `SEARCH splits ... USING INDEX idx_splits_account_id (account_id=?)` (no mention of `reconciled` in the index match — the `reconciled = 0` clause is applied as a row filter).

Save the literal output to a scratch buffer / note for the PR description.

- [ ] **Step 2: Confirm no source files were modified**

Run: `git status`
Expected: working tree clean (we have not made any changes yet).

---

### Task 2: Create the up migration

**Files:**
- Create: `migrations/0008_add_splits_account_reconciled_index.up.sql`

- [ ] **Step 1: Write the up migration**

Create `migrations/0008_add_splits_account_reconciled_index.up.sql` with the following exact contents:

```sql
-- Composite index for GetUnreconciledTransactionsByAccount, which filters on
-- (account_id, reconciled). With separate single-column indexes SQLite can
-- only use one per scan; the composite lets it satisfy the full predicate.
CREATE INDEX IF NOT EXISTS idx_splits_account_reconciled
    ON splits (account_id, reconciled);
```

Notes:
- `IF NOT EXISTS` matches the style used in migrations `0001` and `0003`.
- Column order `(account_id, reconciled)` is deliberate: `account_id` is the high-cardinality column and queries that filter on `account_id` alone can still use the leading column.

- [ ] **Step 2: Verify the file is well-formed SQL**

Run:
```bash
sqlite3 :memory: <<'EOF'
.read migrations/0001_create_initial_schema.up.sql
.read migrations/0008_add_splits_account_reconciled_index.up.sql
.indexes splits
EOF
```

Expected output contains both `idx_splits_account_id` (from 0001) and `idx_splits_account_reconciled` (from 0008).

---

### Task 3: Create the down migration

**Files:**
- Create: `migrations/0008_add_splits_account_reconciled_index.down.sql`

- [ ] **Step 1: Write the down migration**

Create `migrations/0008_add_splits_account_reconciled_index.down.sql` with the following exact contents:

```sql
DROP INDEX IF EXISTS idx_splits_account_reconciled;
```

- [ ] **Step 2: Verify the down migration round-trips**

Run:
```bash
sqlite3 :memory: <<'EOF'
.read migrations/0001_create_initial_schema.up.sql
.read migrations/0008_add_splits_account_reconciled_index.up.sql
.read migrations/0008_add_splits_account_reconciled_index.down.sql
.indexes splits
EOF
```

Expected: `idx_splits_account_reconciled` is absent from the output; `idx_splits_account_id` and `idx_splits_transaction_id` remain.

---

### Task 4: Verify the build embeds the new migrations and tests still pass

**Files:**
- None modified.

- [ ] **Step 1: Build the binary to confirm the embed FS picks up the new files**

Run: `make build`
Expected: builds successfully (produces `./kea_test`). Failure here would indicate a typo in the SQL or a malformed file.

- [ ] **Step 2: Run the full test suite**

Run: `go test ./...`
Expected: all packages pass. The change is pure-index, so no test should change behaviour.

---

### Task 5: Capture post-change `EXPLAIN QUERY PLAN`

Re-run the EXPLAIN with the new migration applied to confirm the planner now picks the composite index.

**Files:**
- None modified.

- [ ] **Step 1: Run EXPLAIN against a fresh in-memory DB with all 8 migrations applied**

Run:
```bash
sqlite3 :memory: <<'EOF'
.read migrations/0001_create_initial_schema.up.sql
.read migrations/0002_add_external_id.up.sql
.read migrations/0003_add_split_reconciled.up.sql
.read migrations/0004_add_account_reconcile_state.up.sql
.read migrations/0005_add_transaction_type.up.sql
.read migrations/0006_backfill_transaction_type.up.sql
.read migrations/0007_add_account_type_check.up.sql
.read migrations/0008_add_splits_account_reconciled_index.up.sql
EXPLAIN QUERY PLAN
SELECT t.id, SUM(s.amount)
FROM transactions t
INNER JOIN splits s ON t.id = s.transaction_id
WHERE s.account_id = 1 AND s.reconciled = 0
GROUP BY t.id;
EOF
```

Expected: output mentions `SEARCH splits ... USING INDEX idx_splits_account_reconciled (account_id=? AND reconciled=?)`. The `reconciled=?` clause now appears inside the index match — the planner is using both columns.

Save the output for the PR description.

- [ ] **Step 2: If the planner did NOT pick the new index, stop and investigate**

If the post-change EXPLAIN still shows `idx_splits_account_id` (or any plan that does not include `reconciled` in the index match), do NOT proceed to commit. Surface the unexpected output to the reviewer — possible causes include SQLite version differences, ANALYZE stats, or the planner preferring the older index for this empty-table case. The reviewer may need to populate the table with a few rows for the planner to make a realistic choice, or accept that on an empty table the plan is non-representative and gather evidence on a real ledger DB instead.

---

### Task 6: Commit and prepare PR description

**Files:**
- Stage: `migrations/0008_add_splits_account_reconciled_index.up.sql`
- Stage: `migrations/0008_add_splits_account_reconciled_index.down.sql`

- [ ] **Step 1: Stage the two new migration files**

Run:
```bash
git add migrations/0008_add_splits_account_reconciled_index.up.sql migrations/0008_add_splits_account_reconciled_index.down.sql
```

- [ ] **Step 2: Verify the diff is exactly the two files**

Run: `git status`
Expected: two new files staged, working tree otherwise clean.

- [ ] **Step 3: Commit**

Run:
```bash
git commit -m "$(cat <<'EOF'
feat: add composite index on splits(account_id, reconciled)

Closes #132.

GetUnreconciledTransactionsByAccount filters on
(s.account_id = ? AND s.reconciled = 0). With only the existing
single-column indexes idx_splits_account_id and idx_splits_reconciled,
SQLite can use at most one of them per scan and applies the other
predicate as a row filter. The new composite idx_splits_account_reconciled
lets the planner satisfy the entire WHERE clause from the index.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

Expected: commit succeeds.

- [ ] **Step 4: Draft the PR description**

Save the following draft for use when opening the PR (do not open the PR yet — that is a separate user-driven step). Fill in the EXPLAIN snippets captured in Tasks 1 and 5:

````markdown
## Summary

Closes #132. Adds a composite index `idx_splits_account_reconciled` on `splits(account_id, reconciled)` so the reconcile query (`GetUnreconciledTransactionsByAccount`) can use a single index for its full `WHERE` clause instead of one index plus a row-by-row filter.

## EXPLAIN QUERY PLAN

Before:

```
<paste output from Task 1>
```

After:

```
<paste output from Task 5>
```

## Out of scope

- Pruning the now-partially-redundant `idx_splits_account_id` and the low-cardinality `idx_splits_reconciled` — left for a follow-up issue once every `WHERE`-clause call site has been audited.

## Test plan

- [x] `make build` — migrations embed cleanly
- [x] `go test ./...` — no regressions
- [x] `EXPLAIN QUERY PLAN` confirms planner now uses the composite index
````

---

## Self-Review

**Spec coverage:**
- "Add migration `0008` with composite index" → Tasks 2 & 3.
- "Style: `IF NOT EXISTS` / `IF EXISTS`" → covered in the SQL bodies in Tasks 2 & 3.
- "Column order `(account_id, reconciled)`" → covered in Task 2 SQL and rationale note.
- "Leave existing indexes alone" → no task touches them.
- "Verification via `make build`, `go test ./...`, and `EXPLAIN QUERY PLAN`" → Tasks 1, 4, 5.
- "Out of scope: dropping existing indexes, Go code changes" → no task touches those.

All spec items have a corresponding task. No gaps.

**Placeholder scan:** No TBDs. All SQL bodies and commands are spelled out in full. The PR description has `<paste output from Task N>` markers, which are intentional handoffs (the EXPLAIN output is captured during execution and pasted in at PR time) — these are not "later/implement" placeholders.

**Type consistency:** Index name is `idx_splits_account_reconciled` in every reference (Tasks 2, 3, 5, 6). Migration prefix is `0008` consistently. File names match: `0008_add_splits_account_reconciled_index.up.sql` / `.down.sql` everywhere. Branch name is `fix/issue-132-splits-composite-index` (matches the branch already created).
