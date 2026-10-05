# Bulk Transaction Details — Fix N+1 Query (Issue #58)

## Problem

`cmd/transaction/list.go:buildViewItems` calls `GetTransactionByID` in a loop for every transaction, issuing 2 SQL queries per transaction (one for the transaction row, one JOIN for splits+accounts). Listing 20 transactions produces ~40 extra queries on top of the initial list query.

## Solution

Add a bulk `GetSplitsWithAccountsByTransactionIDs` method that fetches all splits for a set of transaction IDs in a single query. The service layer assembles `TransactionDetail` objects from the bulk result combined with the already-fetched transaction list.

## Changes by Layer

### 1. Repository Interface (`internal/repository/interfaces.go`)

Add to `TransactionRepository`:

```go
GetSplitsWithAccountsByTransactionIDs(ctx context.Context, txIDs []int64) (map[int64][]model.SplitDetail, error)
```

### 2. Store (`internal/store/sqlite_transaction.go`)

New method on `*Store`. Single query:

```sql
SELECT s.id, s.transaction_id, s.account_id, s.amount, s.currency, s.memo,
       a.name, a.type
FROM splits s
JOIN accounts a ON s.account_id = a.id
WHERE s.transaction_id IN (?, ?, ...)
ORDER BY s.transaction_id, s.id
```

Placeholder list built dynamically from `len(txIDs)`. Returns `map[int64][]model.SplitDetail`.

Edge case: empty `txIDs` slice returns empty map immediately without querying.

### 3. Service (`internal/service/transaction_service.go`)

New method on `*TransactionService`:

```go
func (ts *TransactionService) GetTransactionDetailsByIDs(ctx context.Context, txs []*model.Transaction) (map[int64]*model.TransactionDetail, error)
```

Takes the already-fetched transaction slice (avoids re-querying transaction rows). Steps:
1. Extract IDs from `txs`.
2. Call `GetSplitsWithAccountsByTransactionIDs` once.
3. Build `map[int64]*model.TransactionDetail` by combining each transaction's metadata with its splits from the map.

### 4. Command Layer (`cmd/transaction/list.go`)

Update `ListProvider` interface:
- Remove: `GetTransactionByID(ctx, int64) (*model.TransactionDetail, error)`
- Add: `GetTransactionDetailsByIDs(ctx context.Context, txs []*model.Transaction) (map[int64]*model.TransactionDetail, error)`

Rewrite `buildViewItems`:
1. Call `r.svc.GetTransactionDetailsByIDs(ctx, transactions)` once.
2. Iterate `transactions`, look up detail from result map.
3. Skip with warning if a transaction has no entry in the map.

Remove the `//TODO: Efficiency optimize` comment on line 17.

### 5. Tests

**Store test:** Insert multiple transactions with splits, call bulk method, verify all splits returned correctly keyed by transaction ID.

**Service test:** Mock `GetSplitsWithAccountsByTransactionIDs` to return canned data. Verify `GetTransactionDetailsByIDs` assembles `TransactionDetail` objects correctly.

**Command test:** Update `ListProvider` mock to implement new interface signature.

## Edge Cases

| Case | Behavior |
|------|----------|
| Empty transaction list | Return empty map, no query issued |
| Transaction missing from splits result | Skip with warning (same as current error handling) |
| Single transaction | Works fine — IN clause with one element |

## Performance Impact

- Before: 1 + 2N queries (N = number of transactions listed)
- After: 2 queries total (1 list + 1 bulk splits)
- Default N=20: reduces from 41 queries to 2
