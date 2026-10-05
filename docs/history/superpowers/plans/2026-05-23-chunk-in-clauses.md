# Chunk Unbounded IN Clauses (Issue #125) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent SQLite `SQLITE_MAX_VARIABLE_NUMBER` errors by chunking large ID slices into batches of 500 before building `IN (?,...)` clauses.

**Architecture:** Add a package-private `chunkInt64` helper in `internal/store/` that splits `[]int64` into batches. Each of the three affected methods calls it to iterate over chunks, executes a query per chunk, and merges results. The helper is tested independently; each affected method gets an integration test with >500 IDs.

**Tech Stack:** Go, SQLite, testify

---

### Task 1: Add `chunkInt64` helper

**Files:**
- Create: `internal/store/chunk.go`
- Create: `internal/store/chunk_test.go`

- [ ] **Step 1: Write the failing tests**

In `internal/store/chunk_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkInt64_Empty(t *testing.T) {
	chunks := chunkInt64(nil, 500)
	assert.Empty(t, chunks)
}

func TestChunkInt64_UnderLimit(t *testing.T) {
	ids := []int64{1, 2, 3}
	chunks := chunkInt64(ids, 500)
	assert.Equal(t, [][]int64{{1, 2, 3}}, chunks)
}

func TestChunkInt64_ExactLimit(t *testing.T) {
	ids := make([]int64, 500)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	chunks := chunkInt64(ids, 500)
	assert.Len(t, chunks, 1)
	assert.Len(t, chunks[0], 500)
}

func TestChunkInt64_OverLimit(t *testing.T) {
	ids := make([]int64, 1300)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	chunks := chunkInt64(ids, 500)
	assert.Len(t, chunks, 3)
	assert.Len(t, chunks[0], 500)
	assert.Len(t, chunks[1], 500)
	assert.Len(t, chunks[2], 300)
	assert.Equal(t, int64(1), chunks[0][0])
	assert.Equal(t, int64(501), chunks[1][0])
	assert.Equal(t, int64(1001), chunks[2][0])
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -run TestChunkInt64 -v`
Expected: FAIL — `chunkInt64` undefined.

- [ ] **Step 3: Write minimal implementation**

In `internal/store/chunk.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

const sqliteChunkSize = 500

func chunkInt64(ids []int64, size int) [][]int64 {
	if len(ids) == 0 {
		return nil
	}
	chunks := make([][]int64, 0, (len(ids)+size-1)/size)
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[i:end])
	}
	return chunks
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestChunkInt64 -v`
Expected: PASS (all 4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/store/chunk.go internal/store/chunk_test.go
git commit -m "feat: add chunkInt64 helper for batching IN-clause IDs (#125)"
```

---

### Task 2: Chunk `GetSplitsWithAccountsByTransactionIDs`

**Files:**
- Modify: `internal/store/sqlite_transaction.go:410-462`
- Modify: `internal/store/sqlite_transaction_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Append to `internal/store/sqlite_transaction_test.go`:

```go
func TestGetSplitsWithAccountsByTransactionIDs_LargeBatch(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	const n = 600
	txIDs := make([]int64, n)
	for i := 0; i < n; i++ {
		txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp:   int64(1000 + i),
			Description: fmt.Sprintf("tx-%d", i),
			Status:      model.StatusPending,
			Type:        model.TxTypeExpense,
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
		txIDs[i] = txID
	}

	result, err := s.GetSplitsWithAccountsByTransactionIDs(ctx, txIDs)
	require.NoError(t, err)
	assert.Len(t, result, n)
	for _, txID := range txIDs {
		assert.Len(t, result[txID], 2, "each transaction should have 2 splits")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestGetSplitsWithAccountsByTransactionIDs_LargeBatch -v`
Expected: FAIL — SQLite variable limit exceeded.

- [ ] **Step 3: Refactor to use chunking**

Replace the body of `GetSplitsWithAccountsByTransactionIDs` in `internal/store/sqlite_transaction.go` (lines 413–462) with:

```go
func (s *Store) GetSplitsWithAccountsByTransactionIDs(ctx context.Context, txIDs []int64) (map[int64][]model.SplitDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(txIDs) == 0 {
		return make(map[int64][]model.SplitDetail), nil
	}

	result := make(map[int64][]model.SplitDetail)
	for _, chunk := range chunkInt64(txIDs, sqliteChunkSize) {
		placeholders := make([]byte, 0, len(chunk)*2-1)
		args := make([]any, len(chunk))
		for i, id := range chunk {
			if i > 0 {
				placeholders = append(placeholders, ',')
			}
			placeholders = append(placeholders, '?')
			args[i] = id
		}

		query := `
            SELECT
                s.id, s.transaction_id, s.account_id, s.amount, s.currency, s.memo,
                a.name, a.type
            FROM splits s
            JOIN accounts a ON s.account_id = a.id
            WHERE s.transaction_id IN (` + string(placeholders) + `)
            ORDER BY s.transaction_id, s.id`

		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to query splits by transaction IDs: %w", err)
		}

		for rows.Next() {
			var d model.SplitDetail
			var txID int64
			if err := rows.Scan(
				&d.ID, &txID, &d.AccountID, &d.Amount, &d.Currency, &d.Memo,
				&d.AccountName, &d.AccountType,
			); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("failed to scan split with account: %w", err)
			}
			result[txID] = append(result[txID], d)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("rows iteration error: %w", err)
		}
		_ = rows.Close()
	}
	return result, nil
}
```

Also update the comment above the function — remove the stale "assumes bounded by --limit flag" text.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestGetSplitsWithAccountsByTransactionIDs -v`
Expected: PASS (both old and new tests).

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite_transaction.go internal/store/sqlite_transaction_test.go
git commit -m "fix: chunk GetSplitsWithAccountsByTransactionIDs to avoid SQLite variable limit (#125)"
```

---

### Task 3: Chunk `MarkSplitsReconciledByAccount`

**Files:**
- Modify: `internal/store/sqlite_reconcile.go:82-113`
- Modify: `internal/store/sqlite_reconcile_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Append to `internal/store/sqlite_reconcile_test.go`:

```go
func TestMarkSplitsReconciledByAccount_LargeBatch(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	const n = 600
	txIDs := make([]int64, n)
	for i := 0; i < n; i++ {
		txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp:   int64(1000 + i),
			Description: fmt.Sprintf("tx-%d", i),
			Status:      model.StatusPending,
			Type:        model.TxTypeExpense,
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
		txIDs[i] = txID
	}

	rowsAffected, err := s.MarkSplitsReconciledByAccount(ctx, assetID, txIDs)
	require.NoError(t, err)
	assert.Equal(t, int64(n), rowsAffected)

	entries, err := s.GetUnreconciledTransactionsByAccount(ctx, assetID)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestMarkSplitsReconciledByAccount_LargeBatch -v`
Expected: FAIL — SQLite variable limit exceeded.

- [ ] **Step 3: Refactor to use chunking**

Replace the body of `MarkSplitsReconciledByAccount` in `internal/store/sqlite_reconcile.go` (lines 82–113) with:

```go
func (s *Store) MarkSplitsReconciledByAccount(ctx context.Context, accountID int64, txIDs []int64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(txIDs) == 0 {
		return 0, nil
	}

	var totalAffected int64
	for _, chunk := range chunkInt64(txIDs, sqliteChunkSize) {
		placeholders := strings.Repeat("?,", len(chunk))
		placeholders = placeholders[:len(placeholders)-1]

		splitArgs := make([]any, 0, len(chunk)+1)
		splitArgs = append(splitArgs, accountID)
		for _, id := range chunk {
			splitArgs = append(splitArgs, id)
		}
		splitQuery := fmt.Sprintf(
			"UPDATE splits SET reconciled = 1 WHERE account_id = ? AND transaction_id IN (%s)",
			placeholders,
		)
		result, err := s.db.ExecContext(ctx, splitQuery, splitArgs...)
		if err != nil {
			return 0, fmt.Errorf("failed to mark splits as reconciled: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("failed to get rows affected from splits update: %w", err)
		}
		totalAffected += rowsAffected
	}

	return totalAffected, s.bulkUpdateTransactionStatus(ctx, txIDs, model.StatusReconciled)
}
```

Note: `bulkUpdateTransactionStatus` is called once at the end with the full `txIDs` — Task 4 chunks that method independently, so both the direct call here and the public `BulkUpdateTransactionStatus` are safe.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestMarkSplitsReconciledByAccount -v`
Expected: PASS (old tests + new large-batch test).

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlite_reconcile.go internal/store/sqlite_reconcile_test.go
git commit -m "fix: chunk MarkSplitsReconciledByAccount to avoid SQLite variable limit (#125)"
```

---

### Task 4: Chunk `bulkUpdateTransactionStatus`

**Files:**
- Modify: `internal/store/sqlite_reconcile.go:124-156`
- Modify: `internal/store/sqlite_reconcile_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Append to `internal/store/sqlite_reconcile_test.go`:

```go
func TestBulkUpdateTransactionStatus_LargeBatch(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	const n = 600
	txIDs := make([]int64, n)
	for i := 0; i < n; i++ {
		txID, err := s.CreateTransactionWithSplits(ctx, model.Transaction{
			Timestamp:   int64(1000 + i),
			Description: fmt.Sprintf("tx-%d", i),
			Status:      model.StatusPending,
			Type:        model.TxTypeExpense,
		}, []model.Split{
			{AccountID: assetID, Amount: -100, Currency: "USD"},
			{AccountID: expenseID, Amount: 100, Currency: "USD"},
		})
		require.NoError(t, err)
		txIDs[i] = txID
	}

	err = s.BulkUpdateTransactionStatus(ctx, txIDs, model.StatusCleared)
	require.NoError(t, err)

	for _, txID := range txIDs {
		tx, err := s.GetTransactionByID(ctx, txID)
		require.NoError(t, err)
		assert.Equal(t, model.StatusCleared, tx.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestBulkUpdateTransactionStatus_LargeBatch -v`
Expected: FAIL — SQLite variable limit exceeded.

- [ ] **Step 3: Refactor to use chunking**

Replace `bulkUpdateTransactionStatus` in `internal/store/sqlite_reconcile.go` (lines 124–156) with:

```go
func (s *Store) bulkUpdateTransactionStatus(ctx context.Context, txIDs []int64, status model.TransactionStatus) error {
	if len(txIDs) == 0 {
		return nil
	}

	var totalAffected int64
	for _, chunk := range chunkInt64(txIDs, sqliteChunkSize) {
		placeholders := strings.Repeat("?,", len(chunk))
		placeholders = placeholders[:len(placeholders)-1]

		query := fmt.Sprintf(
			"UPDATE transactions SET status = ? WHERE id IN (%s)",
			placeholders,
		)

		args := make([]any, 0, len(chunk)+1)
		args = append(args, status)
		for _, id := range chunk {
			args = append(args, id)
		}

		result, err := s.db.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to bulk update transaction status: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}
		totalAffected += rowsAffected
	}
	if totalAffected != int64(len(txIDs)) {
		return fmt.Errorf("expected to update %d transactions, updated %d", len(txIDs), totalAffected)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestBulkUpdateTransactionStatus -v`
Expected: PASS (old tests including MismatchedRowCount + new large-batch test).

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: All green — no regressions.

- [ ] **Step 6: Commit**

```bash
git add internal/store/sqlite_reconcile.go internal/store/sqlite_reconcile_test.go
git commit -m "fix: chunk bulkUpdateTransactionStatus to avoid SQLite variable limit (#125)"
```
