# Stored Transaction Type Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store transaction type explicitly on the `transactions` table and wire it through every layer — migrations, model, store, service, cmd/add, cmd/edit, and display — replacing all three `DetermineType` display-time call sites with direct field reads.

**Architecture:** Two SQL migrations add and backfill a `type` column. The `Type TransactionType` field is added to `model.Transaction` and `model.TransactionDetail`; the store reads/writes it via `scanTransactions` and `CreateTransactionWithSplits`; the service enforces structural validity via a new `ValidateSplitsMatchType` function driven by existing `GetTransactionRule` rules; the cmd layers wire the prompted/flagged type end-to-end.

**Tech Stack:** Go, SQLite (golang-migrate embedded SQL), charmbracelet/huh, cobra, testify

---

## File Map

| Action | File | Responsibility |
|---|---|---|
| Create | `migrations/0005_add_transaction_type.up.sql` | Add nullable `type` column |
| Create | `migrations/0005_add_transaction_type.down.sql` | Drop `type` column |
| Create | `migrations/0006_backfill_transaction_type.up.sql` | Classify all existing rows |
| Create | `migrations/0006_backfill_transaction_type.down.sql` | Reset all rows to `''` |
| Modify | `internal/model/transaction.go` | Add `Type` to Transaction + TransactionDetail |
| Modify | `internal/repository/interfaces.go` | Update `UpdateTransactionBasic` signature |
| Modify | `internal/store/sqlite_transaction.go` | INSERT/SELECT/UPDATE include `type` |
| Modify | `internal/service/transaction_classifier.go` | Add `ValidateSplitsMatchType` |
| Modify | `internal/service/transaction_classifier_test.go` | Tests for `ValidateSplitsMatchType` |
| Modify | `internal/service/transaction_ops.go` | `CreateTransaction`, `CreateSimpleTransaction`, `UpdateTransactionComplete` |
| Modify | `internal/service/transaction_ops_test.go` | Update existing tests; add type to inputs |
| Modify | `internal/service/report_service.go` | Replace `DetermineType` call in `buildReportMaps` |
| Modify | `internal/service/testhelper_test.go` | Update mock `UpdateTransactionBasic` + `GetTransactionsByDateRange` |
| Modify | `cmd/add_types.go` | `addFlags`, `addTransactionInput`, `TransactionProvider` interface |
| Modify | `cmd/add.go` | Register `--type` flag |
| Modify | `cmd/add_actions.go` | `runInteractive` sets type; `runFromFlags` parses/infers type; `parseTransactionType` helper |
| Modify | `cmd/transaction/edit_types.go` | Update `EditProvider`; add `OptChangeType` constant |
| Modify | `cmd/transaction/edit_actions.go` | Add `actionEditType`; update `actionSave` |
| Modify | `cmd/transaction/edit.go` | Add Change Type menu item |
| Modify | `cmd/transaction/list.go` | Replace `DetermineType` call; remove from `ListProvider` |

---

### Task 1: Migration files

**Files:**
- Create: `migrations/0005_add_transaction_type.up.sql`
- Create: `migrations/0005_add_transaction_type.down.sql`
- Create: `migrations/0006_backfill_transaction_type.up.sql`
- Create: `migrations/0006_backfill_transaction_type.down.sql`

- [ ] **Step 1: Create schema migration (up)**

`migrations/0005_add_transaction_type.up.sql`:
```sql
ALTER TABLE transactions ADD COLUMN type TEXT NOT NULL DEFAULT '';
```

- [ ] **Step 2: Create schema migration (down)**

`migrations/0005_add_transaction_type.down.sql`:
```sql
-- SQLite does not support DROP COLUMN in older versions; recreate without the column.
CREATE TABLE transactions_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   INTEGER NOT NULL,
    description TEXT,
    status      INTEGER NOT NULL DEFAULT 0,
    external_id TEXT UNIQUE
);

INSERT INTO transactions_new SELECT id, timestamp, description, status, external_id FROM transactions;
DROP TABLE transactions;
ALTER TABLE transactions_new RENAME TO transactions;

CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions (timestamp);
```

- [ ] **Step 3: Create backfill migration (up)**

`migrations/0006_backfill_transaction_type.up.sql`:
```sql
UPDATE transactions SET type = (
  SELECT
    CASE
      WHEN memo_check.has_opening = 1 THEN 'Opening'
      WHEN agg.has_exp = 1 AND agg.has_rev = 1
        THEN CASE WHEN agg.rev_sum >= agg.exp_sum THEN 'Income' ELSE 'Expense' END
      WHEN agg.has_exp = 1 AND agg.al_cnt >= 1 THEN 'Expense'
      WHEN agg.has_rev = 1 AND agg.al_cnt >= 1 THEN 'Income'
      WHEN agg.al_cnt >= 2                       THEN 'Transfer'
      ELSE 'Other'
    END
  FROM (
    SELECT
      MAX(CASE WHEN a.type = 'E' THEN 1 ELSE 0 END)             AS has_exp,
      MAX(CASE WHEN a.type = 'R' THEN 1 ELSE 0 END)             AS has_rev,
      SUM(CASE WHEN a.type IN ('A','L') THEN 1 ELSE 0 END)      AS al_cnt,
      SUM(CASE WHEN a.type = 'E' THEN ABS(s.amount) ELSE 0 END) AS exp_sum,
      SUM(CASE WHEN a.type = 'R' THEN ABS(s.amount) ELSE 0 END) AS rev_sum
    FROM splits s
    JOIN accounts a ON s.account_id = a.id
    WHERE s.transaction_id = transactions.id
  ) agg,
  (
    SELECT MAX(CASE WHEN s.memo = 'Opening Balance' THEN 1 ELSE 0 END) AS has_opening
    FROM splits s
    WHERE s.transaction_id = transactions.id
  ) memo_check
);
```

- [ ] **Step 4: Create backfill migration (down)**

`migrations/0006_backfill_transaction_type.down.sql`:
```sql
UPDATE transactions SET type = '';
```

- [ ] **Step 5: Verify build compiles with embedded migrations**

```bash
make build
```
Expected: binary produced, no errors. The migrations embed via `go:embed` in the existing migration runner — no Go changes needed.

- [ ] **Step 6: Commit**

```bash
git add migrations/
git commit -m "feat: add migration to store transaction type column"
```

---

### Task 2: Model — add Type field

**Files:**
- Modify: `internal/model/transaction.go`

- [ ] **Step 1: Add Type to Transaction**

In `internal/model/transaction.go`, update `Transaction`:
```go
type Transaction struct {
	ID          int64
	Timestamp   int64
	Description string
	Status      TransactionStatus
	Type        TransactionType
	ExternalID  *string
}
```

- [ ] **Step 2: Add Type to TransactionDetail**

In `internal/model/transaction.go`, update `TransactionDetail`:
```go
type TransactionDetail struct {
	ID          int64
	Timestamp   int64
	Description string
	Status      TransactionStatus
	Type        TransactionType
	Splits      []SplitDetail
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```
Expected: compile errors in store and service where `Transaction` or `TransactionDetail` is constructed without the new field — that is expected and will be fixed in later tasks. If you see only those, proceed.

- [ ] **Step 4: Commit**

```bash
git add internal/model/transaction.go
git commit -m "feat: add Type field to Transaction and TransactionDetail models"
```

---

### Task 3: Repository interface + Store

**Files:**
- Modify: `internal/repository/interfaces.go`
- Modify: `internal/store/sqlite_transaction.go`

- [ ] **Step 1: Update repository interface**

In `internal/repository/interfaces.go`, change `UpdateTransactionBasic`:
```go
UpdateTransactionBasic(txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) error
```

- [ ] **Step 2: Update CreateTransactionWithSplits INSERT**

In `internal/store/sqlite_transaction.go`, update `CreateTransactionWithSplits` SQL and param:
```go
func (s *Store) CreateTransactionWithSplits(tx model.Transaction, splits []model.Split) (int64, error) {
	stmtTx, err := s.db.Prepare(`
        INSERT INTO transactions (timestamp, description, status, external_id, type)
        VALUES (?, ?, ?, ?, ?)
        RETURNING id;
    `)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare transaction SQL: %w", err)
	}
	defer func() { _ = stmtTx.Close() }()

	var newTxID int64
	err = stmtTx.QueryRow(tx.Timestamp, tx.Description, tx.Status, tx.ExternalID, tx.Type).Scan(&newTxID)
	// ... rest of function unchanged
```

- [ ] **Step 3: Update GetTransactionByID SELECT**

In `internal/store/sqlite_transaction.go`, update `GetTransactionByID`:
```go
func (s *Store) GetTransactionByID(txID int64) (*model.Transaction, error) {
	var tx model.Transaction
	err := s.db.QueryRow(`
        SELECT id, timestamp, description, status, external_id, type
        FROM transactions
        WHERE id = ?
    `, txID).Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status, &tx.ExternalID, &tx.Type)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("transaction with ID %d not found", txID)
		}
		return nil, fmt.Errorf("failed to query transaction: %w", err)
	}
	return &tx, nil
}
```

- [ ] **Step 4: Update scanTransactions to include type**

In `internal/store/sqlite_transaction.go`, update `scanTransactions`:
```go
func (s *Store) scanTransactions(rows *sql.Rows) ([]*model.Transaction, error) {
	var transactions []*model.Transaction
	for rows.Next() {
		tx := &model.Transaction{}
		err := rows.Scan(&tx.ID, &tx.Timestamp, &tx.Description, &tx.Status, &tx.ExternalID, &tx.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return transactions, nil
}
```

- [ ] **Step 5: Update SELECT queries that feed scanTransactions**

In `internal/store/sqlite_transaction.go`, update the three query methods to include `type` in SELECT. `scanTransactions` is called by `GetTransactionsByAccount`, `GetTransactionsByDateRange`, and `GetAllTransactions` — update each:

`GetTransactionsByAccount`:
```go
rows, err := s.db.Query(`
    SELECT DISTINCT t.id, t.timestamp, t.description, t.status, t.external_id, t.type
    FROM transactions t
    INNER JOIN splits s ON t.id = s.transaction_id
    WHERE s.account_id = ?
    ORDER BY t.timestamp DESC, t.id DESC
    LIMIT ?
`, accountID, limit)
```

`GetTransactionsByDateRange`:
```go
rows, err := s.db.Query(`
    SELECT id, timestamp, description, status, external_id, type
    FROM transactions
    WHERE timestamp >= ? AND timestamp <= ?
    ORDER BY timestamp DESC, id DESC
`, startTime, endTime)
```

`GetAllTransactions`:
```go
rows, err := s.db.Query(`
    SELECT id, timestamp, description, status, external_id, type
    FROM transactions
    ORDER BY timestamp DESC, id DESC
    LIMIT ?
`, limit)
```

- [ ] **Step 6: Update UpdateTransactionBasic**

In `internal/store/sqlite_transaction.go`, update `UpdateTransactionBasic`:
```go
func (s *Store) UpdateTransactionBasic(txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) error {
	result, err := s.db.Exec(`
        UPDATE transactions
        SET description = ?, timestamp = ?, status = ?, type = ?
        WHERE id = ?
    `, description, timestamp, status, txType, txID)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("transaction with ID %d not found", txID)
	}
	return nil
}
```

- [ ] **Step 7: Verify build**

```bash
go build ./...
```
Expected: compile errors in the mock (`testhelper_test.go`) — that is expected and fixed in the next task.

- [ ] **Step 8: Commit**

```bash
git add internal/repository/interfaces.go internal/store/sqlite_transaction.go
git commit -m "feat: include type in all transaction store INSERT/SELECT/UPDATE"
```

---

### Task 4: Update mock infrastructure

**Files:**
- Modify: `internal/service/testhelper_test.go`

- [ ] **Step 1: Update mock UpdateTransactionBasic signature**

In `internal/service/testhelper_test.go`, update `mockTransactionRepo.UpdateTransactionBasic`:
```go
func (m *mockTransactionRepo) UpdateTransactionBasic(txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	tx, ok := m.transactions[txID]
	if !ok {
		return fmt.Errorf("transaction ID %d not found", txID)
	}
	tx.Description = description
	tx.Timestamp = timestamp
	tx.Status = status
	tx.Type = txType
	return nil
}
```

- [ ] **Step 2: Update mock GetTransactionsByDateRange to return stored transactions**

In `internal/service/testhelper_test.go`, update `mockTransactionRepo.GetTransactionsByDateRange` to return all seeded transactions (date range is ignored in tests):
```go
func (m *mockTransactionRepo) GetTransactionsByDateRange(startTime, endTime int64) ([]*model.Transaction, error) {
	result := make([]*model.Transaction, 0, len(m.transactions))
	for _, tx := range m.transactions {
		result = append(result, tx)
	}
	return result, nil
}
```

- [ ] **Step 3: Run tests to see what still breaks**

```bash
go test ./internal/service/... -v 2>&1 | head -60
```
Expected: compile errors in `transaction_ops_test.go` (missing `Type` on `TransactionDetail` inputs, and `UpdateTransactionComplete` signature mismatch). These are fixed in Task 6.

---

### Task 5: ValidateSplitsMatchType (TDD)

**Files:**
- Modify: `internal/service/transaction_classifier_test.go`
- Modify: `internal/service/transaction_classifier.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/service/transaction_classifier_test.go`:
```go
// ──────────────────────────────────────────────
// ValidateSplitsMatchType
// ──────────────────────────────────────────────

func TestValidateSplitsMatchType(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	tests := []struct {
		name    string
		txType  model.TransactionType
		splits  []model.SplitDetail
		wantErr bool
	}{
		{
			name:   "expense: E + A is valid",
			txType: model.TxTypeExpense,
			splits: []model.SplitDetail{
				split("Expenses:Food", model.AccountTypeExpense, 500),
				split("Assets:Bank", model.AccountTypeAsset, -500),
			},
			wantErr: false,
		},
		{
			name:   "expense: missing E account",
			txType: model.TxTypeExpense,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
				split("Assets:Cash", model.AccountTypeAsset, -500),
			},
			wantErr: true,
		},
		{
			name:   "expense: missing A/L account",
			txType: model.TxTypeExpense,
			splits: []model.SplitDetail{
				split("Expenses:Food", model.AccountTypeExpense, 500),
				split("Expenses:Drink", model.AccountTypeExpense, -500),
			},
			wantErr: true,
		},
		{
			name:   "income: R + A is valid",
			txType: model.TxTypeIncome,
			splits: []model.SplitDetail{
				split("Revenue:Salary", model.AccountTypeRevenue, -1000),
				split("Assets:Bank", model.AccountTypeAsset, 1000),
			},
			wantErr: false,
		},
		{
			name:   "income: missing R account",
			txType: model.TxTypeIncome,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
				split("Assets:Cash", model.AccountTypeAsset, -500),
			},
			wantErr: true,
		},
		{
			name:   "income: missing A/L account",
			txType: model.TxTypeIncome,
			splits: []model.SplitDetail{
				split("Revenue:Salary", model.AccountTypeRevenue, 1000),
				split("Revenue:Bonus", model.AccountTypeRevenue, -1000),
			},
			wantErr: true,
		},
		{
			name:   "transfer: two A accounts is valid",
			txType: model.TxTypeTransfer,
			splits: []model.SplitDetail{
				split("Assets:Checking", model.AccountTypeAsset, 500),
				split("Assets:Savings", model.AccountTypeAsset, -500),
			},
			wantErr: false,
		},
		{
			name:   "transfer: A + L is valid",
			txType: model.TxTypeTransfer,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
				split("Liabilities:Card", model.AccountTypeLiability, -500),
			},
			wantErr: false,
		},
		{
			name:   "transfer: contains E account is invalid",
			txType: model.TxTypeTransfer,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
				split("Expenses:Food", model.AccountTypeExpense, -500),
			},
			wantErr: true,
		},
		{
			name:   "opening: always valid",
			txType: model.TxTypeOpening,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
				split("Equity:OpeningBalances_TWD", model.AccountTypeEquity, -500),
			},
			wantErr: false,
		},
		{
			name:   "other: always valid",
			txType: model.TxTypeOther,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateSplitsMatchType(tt.txType, tt.splits)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/ -run TestValidateSplitsMatchType -v
```
Expected: FAIL — `ValidateSplitsMatchType` undefined.

- [ ] **Step 3: Implement ValidateSplitsMatchType**

Add to `internal/service/transaction_classifier.go`:
```go
func (ts *TransactionService) ValidateSplitsMatchType(txType model.TransactionType, splits []model.SplitDetail) error {
	resolveType := func(s model.SplitDetail) (model.AccountType, error) {
		if s.AccountType != "" {
			return s.AccountType, nil
		}
		acc, err := ts.accRepo.GetAccountByName(s.AccountName)
		if err != nil {
			return "", err
		}
		return acc.Type, nil
	}

	switch txType {
	case model.TxTypeOpening, model.TxTypeOther, model.TxTypeDeposit, model.TxTypeWithdrawal:
		return nil

	case model.TxTypeExpense:
		var hasExpense, hasAssetOrLiab bool
		for _, s := range splits {
			accType, err := resolveType(s)
			if err != nil {
				return err
			}
			if accType == model.AccountTypeExpense {
				hasExpense = true
			}
			if accType == model.AccountTypeAsset || accType == model.AccountTypeLiability {
				hasAssetOrLiab = true
			}
		}
		if !hasExpense {
			return fmt.Errorf("expense transaction requires at least one Expense account")
		}
		if !hasAssetOrLiab {
			return fmt.Errorf("expense transaction requires at least one Asset or Liability account")
		}

	case model.TxTypeIncome:
		var hasRevenue, hasAssetOrLiab bool
		for _, s := range splits {
			accType, err := resolveType(s)
			if err != nil {
				return err
			}
			if accType == model.AccountTypeRevenue {
				hasRevenue = true
			}
			if accType == model.AccountTypeAsset || accType == model.AccountTypeLiability {
				hasAssetOrLiab = true
			}
		}
		if !hasRevenue {
			return fmt.Errorf("income transaction requires at least one Revenue account")
		}
		if !hasAssetOrLiab {
			return fmt.Errorf("income transaction requires at least one Asset or Liability account")
		}

	case model.TxTypeTransfer:
		for _, s := range splits {
			accType, err := resolveType(s)
			if err != nil {
				return err
			}
			if accType != model.AccountTypeAsset && accType != model.AccountTypeLiability {
				return fmt.Errorf("transfer transaction must only contain Asset and Liability accounts (found account type %q)", accType)
			}
		}
	}

	return nil
}
```

Also add `"fmt"` to the import block in `transaction_classifier.go` if not already present.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/service/ -run TestValidateSplitsMatchType -v
```
Expected: all subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/transaction_classifier.go internal/service/transaction_classifier_test.go
git commit -m "feat: add ValidateSplitsMatchType to enforce split structure per transaction type"
```

---

### Task 6: Update CreateTransaction and CreateSimpleTransaction

**Files:**
- Modify: `internal/service/transaction_ops.go`
- Modify: `internal/service/transaction_ops_test.go`

- [ ] **Step 1: Update CreateTransaction tests to include Type**

In `internal/service/transaction_ops_test.go`, every `model.TransactionDetail` input passed to `CreateTransaction` needs a `Type` field. Find all test cases in `TestCreateTransaction` and add `Type: model.TxTypeExpense` (or the appropriate type for that test). For example, a test with splits of E + A accounts should use `Type: model.TxTypeExpense`. Also add a test for missing type returning an error:

```go
{
    name: "error: missing type",
    input: model.TransactionDetail{
        Splits: []model.SplitDetail{
            {AccountName: "Expenses:Food", Amount: 500},
            {AccountName: "Assets:Bank", Amount: -500},
        },
    },
    wantErr: true,
},
```

For existing test cases that test non-type-related validation (e.g., missing splits, bad account), add `Type: model.TxTypeExpense` to avoid triggering the "missing type" error. The specific type doesn't matter for those tests — they're testing other validation paths.

- [ ] **Step 2: Update CreateSimpleTransaction tests**

In `internal/service/transaction_ops_test.go`, the `TestCreateSimpleTransaction` tests call `svc.CreateSimpleTransaction(...)`. Add `txType model.TransactionType` as the last argument. For each test case, pass the appropriate type (e.g., `model.TxTypeExpense` for expense tests). The function should also be called without a type (`""`) in one test to verify inference still works:

```go
{
    name: "infers type when empty",
    // uses an asset-to-expense setup in the mock
    // expects type to be inferred as Expense
},
```

- [ ] **Step 3: Update CreateTransaction implementation**

In `internal/service/transaction_ops.go`, update `CreateTransaction`:

After the minimum-splits check, add:
```go
// Require explicit type for new transactions.
if input.Type == "" {
    return 0, fmt.Errorf("transaction type is required")
}
```

After the split-resolution loop (before `ValidateSplitsBalance`), add:
```go
if err := ts.ValidateSplitsMatchType(input.Type, input.Splits); err != nil {
    return 0, fmt.Errorf("splits do not match transaction type %q: %w", input.Type, err)
}
```

Update the `tx` construction to include `Type`:
```go
tx := model.Transaction{
    Timestamp:   input.Timestamp,
    Description: input.Description,
    Status:      input.Status,
    Type:        input.Type,
}
```

- [ ] **Step 4: Update CreateSimpleTransaction signature and type inference**

In `internal/service/transaction_ops.go`, update `CreateSimpleTransaction`:

```go
func (ts *TransactionService) CreateSimpleTransaction(fromAccount, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error) {
	if fromAccount == toAccount {
		return model.TransactionDetail{}, fmt.Errorf("source and destination accounts cannot be the same")
	}
	if amount <= 0 {
		return model.TransactionDetail{}, fmt.Errorf("amount must be positive")
	}

	// If no type provided, infer from account types (backward compatibility for flag mode).
	if txType == "" {
		fromAcc, err := ts.accRepo.GetAccountByName(fromAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve from account: %w", err)
		}
		toAcc, err := ts.accRepo.GetAccountByName(toAccount)
		if err != nil {
			return model.TransactionDetail{}, fmt.Errorf("failed to resolve to account: %w", err)
		}
		inferred, err := ts.DetermineType([]model.SplitDetail{
			{AccountType: toAcc.Type, Amount: amount},
			{AccountType: fromAcc.Type, Amount: -amount},
		})
		if err != nil {
			return model.TransactionDetail{}, err
		}
		txType = inferred
	}

	splits := []model.SplitDetail{
		{AccountName: toAccount, Amount: amount},
		{AccountName: fromAccount, Amount: -amount},
	}
	input := model.TransactionDetail{
		Timestamp:   timestamp,
		Description: desc,
		Status:      status,
		Type:        txType,
		Splits:      splits,
	}
	id, err := ts.CreateTransaction(input)
	if err != nil {
		return model.TransactionDetail{}, err
	}
	input.ID = id
	return input, nil
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/service/ -run "TestCreateTransaction|TestCreateSimpleTransaction" -v
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "feat: require and validate Type in CreateTransaction; infer in CreateSimpleTransaction"
```

---

### Task 7: Update UpdateTransactionComplete

**Files:**
- Modify: `internal/service/transaction_ops.go`
- Modify: `internal/service/transaction_ops_test.go`

- [ ] **Step 1: Update UpdateTransactionComplete tests**

In `internal/service/transaction_ops_test.go`, find `TestUpdateTransactionComplete` (or similar tests that call `UpdateTransactionComplete`) and add `txType model.TransactionType` as the new parameter. Pass `model.TxTypeExpense` (or appropriate type) to test calls. Add a test for type mismatch:

```go
{
    name: "error: splits do not match declared type",
    txID: existingTxID,
    txType: model.TxTypeTransfer,
    splits: []model.SplitDetail{
        // expense + asset splits — incompatible with Transfer
        {AccountID: expenseAccID, AccountType: model.AccountTypeExpense, Amount: 500, Currency: "TWD"},
        {AccountID: assetAccID, AccountType: model.AccountTypeAsset, Amount: -500, Currency: "TWD"},
    },
    wantErr: true,
},
```

- [ ] **Step 2: Update UpdateTransactionComplete signature**

In `internal/service/transaction_ops.go`, update the function signature:
```go
func (ts *TransactionService) UpdateTransactionComplete(txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, splits []model.SplitDetail) error {
```

After the existing splits-count and balance validation, add:
```go
if err := ts.ValidateSplitsMatchType(txType, splits); err != nil {
    return fmt.Errorf("splits do not match transaction type %q: %w", txType, err)
}
```

Update the `repo.UpdateTransactionBasic` call to pass `txType`:
```go
if err := repo.UpdateTransactionBasic(txID, description, timestamp, status, txType); err != nil {
    return err
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/service/ -run TestUpdate -v
```
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/service/transaction_ops.go internal/service/transaction_ops_test.go
git commit -m "feat: add txType param and split validation to UpdateTransactionComplete"
```

---

### Task 8: Update report service

**Files:**
- Modify: `internal/service/report_service.go`

- [ ] **Step 1: Update buildReportMaps to read stored type**

In `internal/service/report_service.go`, replace the entire `buildReportMaps` function:

```go
func (ts *TransactionService) buildReportMaps(startTime, endTime int64, includeIncome, includeExpense bool) (incomeByAccount, expenseByAccount map[string]*model.ReportRow, err error) {
	txSplitsMap, err := ts.txRepo.GetSplitsWithAccountsByDateRange(startTime, endTime)
	if err != nil {
		return nil, nil, err
	}

	txs, err := ts.txRepo.GetTransactionsByDateRange(startTime, endTime)
	if err != nil {
		return nil, nil, err
	}
	txTypeMap := make(map[int64]model.TransactionType, len(txs))
	for _, tx := range txs {
		txTypeMap[tx.ID] = tx.Type
	}

	incomeByAccount = map[string]*model.ReportRow{}
	expenseByAccount = map[string]*model.ReportRow{}

	for txID, details := range txSplitsMap {
		txType := txTypeMap[txID]

		if includeIncome && txType == model.TxTypeIncome {
			offset := offsetAccountName(details, model.AccountTypeRevenue)
			for _, split := range details {
				if split.AccountType == model.AccountTypeRevenue {
					key := split.AccountName + "|" + offset
					row := getOrCreateRowWithOffset(incomeByAccount, key, split.AccountName, offset, split.Currency)
					row.Amount += utils.AbsInt64(split.Amount)
					row.TxCount++
				}
			}
		}

		if includeExpense && txType == model.TxTypeExpense {
			offset := offsetAccountName(details, model.AccountTypeExpense)
			for _, split := range details {
				if split.AccountType == model.AccountTypeExpense {
					key := split.AccountName + "|" + offset
					row := getOrCreateRowWithOffset(expenseByAccount, key, split.AccountName, offset, split.Currency)
					row.Amount += utils.AbsInt64(split.Amount)
					row.TxCount++
				}
			}
		}
	}

	return incomeByAccount, expenseByAccount, nil
}
```

- [ ] **Step 2: Run report tests**

```bash
go test ./internal/service/ -run TestGenerate -v
```
Expected: all PASS. (The updated mock `GetTransactionsByDateRange` now returns transactions seeded in `m.transactions`. Existing report tests that inject data via `splitsWithAccts` may need transactions added to the mock — see next step if tests fail.)

- [ ] **Step 3: Fix report tests if needed**

If report tests fail because `txTypeMap` is empty (transactions not seeded), update the test setup to add transactions with types via `mockTransactionRepo.addTransaction`. For each txID in `splitsWithAccts`, add a corresponding transaction:

```go
// example for an income test
repo := newMockTransactionRepo()
repo.splitsWithAccts = map[int64][]model.SplitDetail{
    1: {
        split("Revenue:Salary", model.AccountTypeRevenue, -1000),
        split("Assets:Bank", model.AccountTypeAsset, 1000),
    },
}
repo.addTransaction(&model.Transaction{ID: 1, Type: model.TxTypeIncome}, nil)
```

- [ ] **Step 4: Run full service tests**

```bash
go test ./internal/service/... -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/report_service.go
git commit -m "feat: replace DetermineType in buildReportMaps with stored transaction type"
```

---

### Task 9: Wire Type through cmd/add

**Files:**
- Modify: `cmd/add_types.go`
- Modify: `cmd/add.go`
- Modify: `cmd/add_actions.go`

- [ ] **Step 1: Update add_types.go**

In `cmd/add_types.go`:

Add `Type` to `addFlags`:
```go
type addFlags struct {
	Description string
	Amount      string
	From        string
	To          string
	Status      string
	Timestamp   string
	Type        string
	JSON        bool
}
```

Add `Type` to `addTransactionInput`:
```go
type addTransactionInput struct {
	FromAccountID string
	ToAccountID   string
	AmountCents   int64
	Description   string
	Timestamp     int64
	Status        model.TransactionStatus
	Type          model.TransactionType
}
```

Update `TransactionProvider` interface — add `txType` param to `CreateSimpleTransaction`:
```go
type TransactionProvider interface {
	GetTransactionRule(mode model.TransactionType) (model.TransactionRule, error)
	CreateSimpleTransaction(fromAccount string, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error)
}
```

- [ ] **Step 2: Register --type flag in NewAddCmd**

In `cmd/add.go`, add inside `NewAddCmd` alongside the other flag registrations:
```go
cmd.Flags().StringVar(&flags.Type, "type", "", "Transaction type: expense, income, transfer")
```

- [ ] **Step 3: Pass input.Type to CreateSimpleTransaction in Run**

In `cmd/add.go`, update the `CreateSimpleTransaction` call:
```go
result, err := r.txSvc.CreateSimpleTransaction(
    input.FromAccountID,
    input.ToAccountID,
    input.AmountCents,
    input.Description,
    input.Timestamp,
    input.Status,
    input.Type,
)
```

- [ ] **Step 4: Set input.Type in runInteractive**

In `cmd/add_actions.go`, in `runInteractive`, after the existing `mode := r.determineMode(rawType)` line, add before the return statement:
```go
return addTransactionInput{
    FromAccountID: fromAccount,
    ToAccountID:   toAccount,
    AmountCents:   amountCents,
    Description:   description,
    Timestamp:     timestamp,
    Status:        status,
    Type:          mode,
}, nil
```

(Replace the existing return statement — just add `Type: mode` to the struct literal.)

- [ ] **Step 5: Add parseTransactionType helper and update runFromFlags**

In `cmd/add_actions.go`, add this private helper at the bottom of the file:
```go
func parseTransactionType(s string) (model.TransactionType, error) {
	switch strings.ToLower(s) {
	case "expense":
		return model.TxTypeExpense, nil
	case "income":
		return model.TxTypeIncome, nil
	case "transfer":
		return model.TxTypeTransfer, nil
	default:
		return "", fmt.Errorf("invalid type %q: must be expense, income, or transfer", s)
	}
}
```

In `runFromFlags`, after the status parsing block and before the account validation, add:
```go
var txType model.TransactionType
if flags.Type != "" {
    txType, err = parseTransactionType(flags.Type)
    if err != nil {
        return addTransactionInput{}, err
    }
}
// If txType == "", CreateSimpleTransaction will infer it from account types.
```

Update the return statement to include `Type: txType`.

- [ ] **Step 6: Build and verify**

```bash
go build ./...
```
Expected: clean build.

- [ ] **Step 7: Commit**

```bash
git add cmd/add_types.go cmd/add.go cmd/add_actions.go
git commit -m "feat: add --type flag to kea add; wire type through add flow"
```

---

### Task 10: cmd/transaction/edit — Change Type action

**Files:**
- Modify: `cmd/transaction/edit_types.go`
- Modify: `cmd/transaction/edit_actions.go`
- Modify: `cmd/transaction/edit.go`

- [ ] **Step 1: Update EditProvider interface and add constant**

In `cmd/transaction/edit_types.go`:

Add `OptChangeType` constant:
```go
const (
	OptBasicInfo    = "Basic Info (description, date, status)"
	OptChangeType   = "Change Type"
	OptQuickAccount = "Change Account (quick edit)"
	OptQuickAmount  = "Change Amount (both sides)"
	OptEditSplits   = "Edit Splits (Advanced)"
	OptSave         = "Save & Exit"
	OptCancel       = "Cancel (discard changes)"
)
```

Update `EditProvider` — remove `DetermineType`, add `ValidateSplitsMatchType`, update `UpdateTransactionComplete`:
```go
type EditProvider interface {
	GetTransactionByID(txID int64) (*model.TransactionDetail, error)
	IsEditable(detail *model.TransactionDetail) (bool, service.NotEditableReason)
	GetAllowedAccounts(txType model.TransactionType, currentAccountType model.AccountType, allAccounts []*model.Account) []*model.Account
	ValidateTransactionEdit(splits []model.SplitDetail) error
	ValidateSplitsMatchType(txType model.TransactionType, splits []model.SplitDetail) error
	UpdateTransactionComplete(txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, splits []model.SplitDetail) error
}
```

- [ ] **Step 2: Implement actionEditType**

Add to `cmd/transaction/edit_actions.go`:
```go
func (r *editRunner) actionEditType(detail *model.TransactionDetail) error {
	rawType, err := prompts.PromptTransactionType()
	if err != nil {
		return err
	}

	newType := r.determineMode(rawType)

	if err := r.txSvc.ValidateSplitsMatchType(newType, detail.Splits); err != nil {
		r.view.ShowWarning(fmt.Sprintf("Cannot change type to %s: %s", newType, err.Error()))
		r.view.ShowWarning("Fix the splits first, then change the type.")
		return nil
	}

	detail.Type = newType
	r.view.ShowSuccess(fmt.Sprintf("Type changed to: %s", newType))
	return nil
}
```

Also add a `determineMode` helper to `edit_actions.go` (same logic as in `add_actions.go`):
```go
func (r *editRunner) determineMode(rawInput string) model.TransactionType {
	lower := strings.ToLower(rawInput)
	if strings.Contains(lower, "expense") {
		return model.TxTypeExpense
	}
	if strings.Contains(lower, "income") {
		return model.TxTypeIncome
	}
	return model.TxTypeTransfer
}
```

Add necessary imports at the top of `edit_actions.go`: `"fmt"`, `"strings"`, `"github.com/hance08/kea/ui/prompts"` (if not already present).

- [ ] **Step 3: Update actionSave to pass detail.Type**

In `cmd/transaction/edit_actions.go`, update `actionSave`:
```go
func (r *editRunner) actionSave(detail *model.TransactionDetail) error {
	splits := detail.ToSplitInputs()
	if err := r.txSvc.ValidateTransactionEdit(splits); err != nil {
		return err
	}
	if err := r.txSvc.UpdateTransactionComplete(
		r.txID, detail.Description, detail.Timestamp, detail.Status, detail.Type, splits,
	); err != nil {
		return err
	}
	r.view.ShowSuccess(fmt.Sprintf("Transaction (ID: %d) saved successfully", r.txID))
	return nil
}
```

- [ ] **Step 4: Add Change Type to the edit menu**

In `cmd/transaction/edit.go`, in `getAvailableMenuItems`, add the Change Type item after the Basic Info item:
```go
{
    Label:     OptChangeType,
    Condition: func(d *model.TransactionDetail) bool { return d.Type != model.TxTypeOpening },
    Action:    r.actionEditType,
},
```

Also update `actionQuickChangeAccount` in `edit_actions.go` to read type from `detail.Type` instead of calling `DetermineType`:
```go
func (r *editRunner) actionQuickChangeAccount(detail *model.TransactionDetail) error {
	if len(detail.Splits) != 2 {
		return fmt.Errorf("quick edit supports only 2 splits")
	}

	txType := detail.Type

	if txType == model.TxTypeOpening {
		r.view.ShowWarning("Cannot quick-edit Opening Balance transaction")
		return nil
	}
	// ... rest unchanged
```

- [ ] **Step 5: Build and verify**

```bash
go build ./...
```
Expected: clean build.

- [ ] **Step 6: Commit**

```bash
git add cmd/transaction/edit_types.go cmd/transaction/edit_actions.go cmd/transaction/edit.go
git commit -m "feat: add Change Type action to kea edit with immediate split validation"
```

---

### Task 11: Display logic cleanup

**Files:**
- Modify: `cmd/transaction/list.go`

- [ ] **Step 1: Replace DetermineType call in list.go**

In `cmd/transaction/list.go`, find the `DetermineType` call (around line 117). Replace:
```go
txTypeEnum, err := r.svc.DetermineType(detail.Splits)
if err != nil {
    r.view.ShowWarning("failed to determine transaction type for tx %d: %v", detail.ID, err)
    continue
}
```
with:
```go
txTypeEnum := detail.Type
```

- [ ] **Step 2: Remove DetermineType from ListProvider interface**

In `cmd/transaction/list.go`, remove `DetermineType` from the `ListProvider` interface:
```go
type ListProvider interface {
	GetTransactionHistory(accountName string, limit int) ([]*model.Transaction, error)
	GetRecentTransactions(limit int) ([]*model.Transaction, error)
	GetTransactionByID(txID int64) (*model.TransactionDetail, error)
	GetDisplayAccount(splits []model.SplitDetail, txType string) (string, error)
	GetDisplayOffsetAccount(splits []model.SplitDetail, txType string, primaryAccount string) (string, error)
	GetDisplayAmount(splits []model.SplitDetail) (int64, string)
}
```

- [ ] **Step 3: Run full test suite**

```bash
go test ./... -v 2>&1 | tail -30
```
Expected: all PASS. If any test references `DetermineType` in a mock or test double, remove it.

- [ ] **Step 4: Final build check**

```bash
make build
```
Expected: clean binary.

- [ ] **Step 5: Commit**

```bash
git add cmd/transaction/list.go
git commit -m "feat: replace DetermineType display-time inference with stored Type field"
```
