# Monthly Budget Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Per-ledger monthly budgets on Expense accounts, with budget vs actual in the CLI, HTTP API, SPA report page, SPA settings page and a dashboard card.

**Architecture:** A new `budgets` table (migration 0012) stores versioned rows `(account, effective_month, amount, stopped)`. A new `BudgetService` (facade `svc.Budget()`) owns validation, version selection and the report; it reads splits through the existing `TransactionRepository` methods so the "Expense-typed transactions only" rule matches the expense report, but sums **signed** amounts. A new `BudgetRepository` (implemented by `*store.Store`) only does CRUD. API, CLI (`cmd/budget`) and SPA sit on top.

**Tech Stack:** Go 1.x, SQLite (mattn/go-sqlite3, golang-migrate), chi, cobra, huh, pterm, tablewriter, testify; React 18, TanStack Router/Query, Vitest, Testing Library.

**Spec:** [docs/history/superpowers/specs/2026-10-05-monthly-budget-design.md](../specs/2026-10-05-monthly-budget-design.md)

## Global Constraints

- All code, comments, test names, commit messages and docs in English.
- Amounts are `int64` cents; convert only with `utils.FormatAmount` / `utils.ParseAmount` (Go). HTTP API amounts are integer cents; CLI `--json` amounts are decimal units via `views.CentsToUnit`.
- Never sum amounts across currencies; every total is a `map[string]int64` keyed by currency.
- Dependency direction: `cmd`/`ui`/`internal/api` -> `internal/service` -> `internal/repository` <- `internal/store`. `internal/model` imports no kea package. Service never imports store.
- Every repository/store method takes `context.Context` first and uses `*Context` methods of `database/sql`.
- Reads that guard writes go inside `TransactionManager.ExecTx`, using the `repo` argument (no nesting).
- Wrap errors with `%w`; services return service errors (`ErrNotFound`, `*ValidationError`), never raw repository errors. A new service sentinel needs a `mapError` case (this plan adds none).
- Every source file starts with the two-line SPDX/copyright header used in the repo:
  `// SPDX-License-Identifier: GPL-3.0-or-later` / `// Copyright (C) 2026  Hance Chin`.
- CLI flag patterns (from `docs/recipes/add-cli-command.md`): Pattern A = 3+ flags (store flags struct), Pattern B = 1-2 flags (copy fields), Pattern C = needs `cmd.Flags().Changed()` / interactive-vs-flag mode (pass flags to `Run`). Never mix within a command.
- Month strings are `YYYY-MM`, interpreted in **local time** (same as `parseMonth` in `internal/service/report_service.go`).
- Work on branch `feat/monthly-budget`. Commit after each task. End every commit message with:
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`

## Review Focus

1. **Name-prefix collision** — a budget on `Expenses:Food` must not count `Expenses:FoodTruck`; only the account itself and names starting with `Expenses:Food:`. Pinned in Task 4.
2. **Month boundary in local time** — a transaction late on the last local day of the month belongs to that month; the SPA must not derive "current month" from UTC (the existing `TopExpenseCategories` widget does). The dashboard card omits `month` so the server decides; SPA helpers use local `Date` getters. Pinned in Task 9 and Task 11.
3. **Re-setting a stopped month** — `SetBudget` on an (account, month) whose row is `stopped=1` must clear `stopped`. Pinned in Task 2 (store) and Task 3 (service).
4. **Account rename after budgeting** — `ListBudgets` and the report must show the new name (join on `account_id`). Pinned in Task 2.
5. **Zero budget** — `Budget = 0` with spending must read as over budget (CLI shows `over`, SPA bar is `over`), and `0/0` must not divide by zero. Pinned in Task 6 and Task 9.

---

## File Structure

| File | Responsibility |
|---|---|
| `migrations/0012_create_budgets.up.sql` / `.down.sql` | Schema |
| `internal/model/budget.go` | `Budget`, inputs, `BudgetReport`, `BudgetReportRow` |
| `internal/repository/interfaces.go` | `BudgetRepository`; added to `Repository` |
| `internal/store/sqlite_budget.go` | SQLite CRUD for budgets |
| `internal/service/budget_service.go` | `BudgetService` CRUD + validation + version selection |
| `internal/service/budget_report.go` | `GenerateBudgetReport` |
| `internal/service/service.go` | facade `Budget()`; `NewService` gains `budgetRepo` |
| `internal/api/budgets.go` | handlers; routes in `internal/api/router.go` |
| `ui/views/budget.go` | `BudgetListView`, `BudgetReportView` (write to an `io.Writer`) |
| `ui/views/json_types.go` | `JSONBudget*` DTOs |
| `cmd/budget/*.go` | `kea budget set|stop|list|delete|report` |
| `spa/src/lib/types.ts`, `spa/src/lib/api/budgets.ts`, `spa/src/lib/hooks/useBudgets.ts`, `spa/src/lib/budgets.ts` | SPA data + pure helpers |
| `spa/src/components/budgets/*` | `MonthPicker`, `BudgetProgressBar`, `BudgetForm` |
| `spa/src/routes/reports.budget.tsx`, `spa/src/routes/budgets.tsx` | pages |
| `spa/src/components/dashboard/widgets/BudgetProgress.tsx` | dashboard card |

---

### Task 1: Migration 0012

**Files:**
- Create: `migrations/0012_create_budgets.up.sql`
- Create: `migrations/0012_create_budgets.down.sql`
- Test: `internal/store/migration_0012_test.go`

**Interfaces:**
- Produces: table `budgets(id, account_id, effective_month, amount, stopped)` with `UNIQUE(account_id, effective_month)` and `ON DELETE CASCADE`.

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"io/fs"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration0012_BudgetsConstraints(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	db := s.DB()

	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	insert := func(accountID int64, month string, amount int64, stopped int) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO budgets (account_id, effective_month, amount, stopped) VALUES (?, ?, ?, ?)`,
			accountID, month, amount, stopped)
		return err
	}

	require.NoError(t, insert(foodID, "2026-01", 800000, 0))
	assert.Error(t, insert(foodID, "2026-01", 1, 0), "duplicate (account, month) must be rejected")
	assert.Error(t, insert(foodID, "2026-1", 1, 0), "month shape must be YYYY-MM")
	assert.Error(t, insert(foodID, "2026-02", -1, 0), "negative amount must be rejected")
	assert.Error(t, insert(foodID, "2026-03", 5, 1), "stopped row must have amount 0")
	assert.Error(t, insert(foodID, "2026-04", 0, 2), "stopped must be 0 or 1")
	assert.Error(t, insert(999, "2026-05", 0, 0), "unknown account must be rejected")
	require.NoError(t, insert(foodID, "2026-06", 0, 1))
	require.NoError(t, insert(foodID, "2026-07", 0, 0), "zero budget is valid")
}

func TestMigration0012_CascadeOnAccountDelete(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO budgets (account_id, effective_month, amount) VALUES (?, '2026-01', 100)`, foodID)
	require.NoError(t, err)

	require.NoError(t, s.DeleteAccount(ctx, foodID))

	var n int
	require.NoError(t, s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM budgets`).Scan(&n))
	assert.Equal(t, 0, n)
}

func TestMigration0012_Down(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	down, err := fs.ReadFile(migrations.FS, "0012_create_budgets.down.sql")
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx, string(down))
	require.NoError(t, err)

	var n int
	require.NoError(t, s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('budgets', 'idx_budgets_account_month')`).Scan(&n))
	assert.Equal(t, 0, n)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestMigration0012 -v`
Expected: FAIL (`no such table: budgets` / file not found).

- [ ] **Step 3: Write the migration**

`migrations/0012_create_budgets.up.sql`:

```sql
-- Monthly budgets on Expense accounts. Each row is a version that applies
-- from effective_month (YYYY-MM) until a later row for the same account.
CREATE TABLE IF NOT EXISTS budgets (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id      INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    effective_month TEXT    NOT NULL CHECK (effective_month GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'),
    amount          INTEGER NOT NULL CHECK (amount >= 0),
    stopped         INTEGER NOT NULL DEFAULT 0 CHECK (stopped IN (0, 1)),
    CHECK (stopped = 0 OR amount = 0),
    UNIQUE (account_id, effective_month)
);

CREATE INDEX IF NOT EXISTS idx_budgets_account_month ON budgets (account_id, effective_month);
```

`migrations/0012_create_budgets.down.sql`:

```sql
DROP INDEX IF EXISTS idx_budgets_account_month;
DROP TABLE IF EXISTS budgets;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestMigration0012 -v`
Expected: PASS. Then `go test ./internal/store/` — all PASS (except the known root-only `TestSwap_FailedSwapKeepsOldConnection` inside Docker).

- [ ] **Step 5: Commit**

```bash
git add migrations/0012_create_budgets.up.sql migrations/0012_create_budgets.down.sql internal/store/migration_0012_test.go
git commit -m "feat(store): add budgets table migration"
```

---

### Task 2: Model, repository interface, store implementation

**Files:**
- Create: `internal/model/budget.go`
- Modify: `internal/repository/interfaces.go` (add `BudgetRepository`, extend `Repository`)
- Create: `internal/store/sqlite_budget.go`
- Test: `internal/store/sqlite_budget_test.go`
- Modify: `internal/service/testhelper_test.go` (add `mockBudgetRepo`, embed in `mockCombinedRepo`, field on `mockTransactionManager`) — required so the service package still compiles once `Repository` grows.

**Interfaces:**
- Consumes: table from Task 1.
- Produces:
  ```go
  // internal/model/budget.go
  type Budget struct{ ID, AccountID int64; AccountName, EffectiveMonth string; Amount int64; Stopped bool }
  type SetBudgetInput struct{ AccountName, EffectiveMonth string; Amount int64 }
  type StopBudgetInput struct{ AccountName, EffectiveMonth string }
  type BudgetReport struct{ Month string; Rows []BudgetReportRow; TotalBudget, TotalActual map[string]int64 }
  type BudgetReportRow struct{ AccountID int64; AccountName, Currency, EffectiveMonth string; Budget, Actual, ActualRegular, ActualIrregular, Remaining int64; ExcludedAccounts []string }
  // internal/repository/interfaces.go
  type BudgetRepository interface {
      UpsertBudget(ctx context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error)
      ListBudgets(ctx context.Context) ([]model.Budget, error)
      DeleteBudget(ctx context.Context, id int64) error
  }
  // internal/service/testhelper_test.go
  func newMockBudgetRepo(accRepo *mockAccountRepo) *mockBudgetRepo
  ```

- [ ] **Step 1: Create the model** (`internal/model/budget.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// Budget is one version of a monthly budget on an Expense account. It applies
// from EffectiveMonth (YYYY-MM) until a later version for the same account.
// A Stopped version ends the budget; its Amount is always 0.
type Budget struct {
	ID             int64  `json:"id"`
	AccountID      int64  `json:"account_id"`
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
	Amount         int64  `json:"amount"` // cents
	Stopped        bool   `json:"stopped"`
}

// SetBudgetInput sets (or replaces) the budget version starting at EffectiveMonth.
type SetBudgetInput struct {
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
	Amount         int64  `json:"amount"` // cents, >= 0
}

// StopBudgetInput ends an account's budget from EffectiveMonth on.
type StopBudgetInput struct {
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
}

// BudgetReport compares each active budget in Month with actual spending.
// Totals are per currency and only include top-level budgeted rows (rows with
// no budgeted ancestor), so parent and child budgets are not double counted.
type BudgetReport struct {
	Month       string            `json:"month"`
	Rows        []BudgetReportRow `json:"rows"`
	TotalBudget map[string]int64  `json:"total_budget"`
	TotalActual map[string]int64  `json:"total_actual"`
}

// BudgetReportRow is one budgeted account. Actual is the signed sum of Expense
// splits of Expense-typed transactions on the account and its descendants in
// Currency; refunds reduce it and it may be negative.
type BudgetReportRow struct {
	AccountID        int64    `json:"account_id"`
	AccountName      string   `json:"account_name"`
	Currency         string   `json:"currency"`
	EffectiveMonth   string   `json:"effective_month"`
	Budget           int64    `json:"budget"`
	Actual           int64    `json:"actual"`
	ActualRegular    int64    `json:"actual_regular"`
	ActualIrregular  int64    `json:"actual_irregular"`
	Remaining        int64    `json:"remaining"`
	ExcludedAccounts []string `json:"excluded_accounts"` // descendants skipped for a different currency
}
```

- [ ] **Step 2: Extend the repository interfaces** (`internal/repository/interfaces.go`)

Add after `TransactionRepository`:

```go
// BudgetRepository stores budget versions. Selecting the version that applies
// to a month is a service concern.
type BudgetRepository interface {
	// UpsertBudget inserts the (accountID, month) version or replaces its
	// amount and stopped flag, returning the row ID.
	UpsertBudget(ctx context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error)
	// ListBudgets returns every version joined with its account name, ordered
	// by account name, then effective month.
	ListBudgets(ctx context.Context) ([]model.Budget, error)
	// DeleteBudget removes one version. A missing id wraps ErrNotFound.
	DeleteBudget(ctx context.Context, id int64) error
}
```

and change `Repository` to:

```go
type Repository interface {
	AccountRepository
	TransactionRepository
	BudgetRepository
}
```

- [ ] **Step 3: Keep the service test package compiling** (`internal/service/testhelper_test.go`)

Add a section after `mockTransactionRepo`:

```go
// ──────────────────────────────────────────────
// mockBudgetRepo
// ──────────────────────────────────────────────

type mockBudgetRepo struct {
	accRepo *mockAccountRepo // resolves AccountName on list, like the SQL JOIN
	rows    map[int64]*model.Budget
	nextID  int64

	upsertErr error
	listErr   error
	deleteErr error
}

func newMockBudgetRepo(accRepo *mockAccountRepo) *mockBudgetRepo {
	return &mockBudgetRepo{accRepo: accRepo, rows: make(map[int64]*model.Budget), nextID: 1}
}

func (m *mockBudgetRepo) UpsertBudget(_ context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error) {
	if m.upsertErr != nil {
		return 0, m.upsertErr
	}
	for _, b := range m.rows {
		if b.AccountID == accountID && b.EffectiveMonth == month {
			b.Amount, b.Stopped = amount, stopped
			return b.ID, nil
		}
	}
	id := m.nextID
	m.nextID++
	m.rows[id] = &model.Budget{ID: id, AccountID: accountID, EffectiveMonth: month, Amount: amount, Stopped: stopped}
	return id, nil
}

func (m *mockBudgetRepo) ListBudgets(_ context.Context) ([]model.Budget, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	out := make([]model.Budget, 0, len(m.rows))
	for _, b := range m.rows {
		cp := *b
		if acc, ok := m.accRepo.accountsByID[b.AccountID]; ok {
			cp.AccountName = acc.Name
		}
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AccountName != out[j].AccountName {
			return out[i].AccountName < out[j].AccountName
		}
		return out[i].EffectiveMonth < out[j].EffectiveMonth
	})
	return out, nil
}

func (m *mockBudgetRepo) DeleteBudget(_ context.Context, id int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.rows[id]; !ok {
		return fmt.Errorf("budget %d not found: %w", id, repository.ErrNotFound)
	}
	delete(m.rows, id)
	return nil
}
```

Change `mockCombinedRepo` to embed it:

```go
type mockCombinedRepo struct {
	*mockAccountRepo
	*mockTransactionRepo
	*mockBudgetRepo
}
```

Add a field to `mockTransactionManager` and pass it into `combined` (budget writes are single statements, so no snapshot/rollback is needed for them):

```go
type mockTransactionManager struct {
	accRepo    *mockAccountRepo
	txRepo     *mockTransactionRepo
	budgetRepo *mockBudgetRepo
	failTx     bool
}
```

```go
	combined := &mockCombinedRepo{
		mockAccountRepo:     m.accRepo,
		mockTransactionRepo: m.txRepo,
		mockBudgetRepo:      m.budgetRepo,
	}
```

- [ ] **Step 4: Write the failing store test** (`internal/store/sqlite_budget_test.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertBudget_InsertThenReplace(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	id1, err := s.UpsertBudget(ctx, foodID, "2026-01", 800000, false)
	require.NoError(t, err)
	id2, err := s.UpsertBudget(ctx, foodID, "2026-01", 900000, false)
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "same (account, month) must update the same row")

	list, err := s.ListBudgets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int64(900000), list[0].Amount)
}

func TestUpsertBudget_SetAfterStopClearsStopped(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.UpsertBudget(ctx, foodID, "2026-03", 0, true)
	require.NoError(t, err)
	_, err = s.UpsertBudget(ctx, foodID, "2026-03", 5000, false)
	require.NoError(t, err)

	list, err := s.ListBudgets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.False(t, list[0].Stopped)
	assert.Equal(t, int64(5000), list[0].Amount)
}

func TestListBudgets_JoinsNameAndOrders(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	rentID, err := s.CreateAccount(ctx, "Expenses:Rent", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.UpsertBudget(ctx, rentID, "2026-01", 1, false)
	require.NoError(t, err)
	_, err = s.UpsertBudget(ctx, foodID, "2026-05", 2, false)
	require.NoError(t, err)
	_, err = s.UpsertBudget(ctx, foodID, "2026-02", 3, false)
	require.NoError(t, err)

	list, err := s.ListBudgets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "Expenses:Food", list[0].AccountName)
	assert.Equal(t, "2026-02", list[0].EffectiveMonth)
	assert.Equal(t, "2026-05", list[1].EffectiveMonth)
	assert.Equal(t, "Expenses:Rent", list[2].AccountName)
}

func TestListBudgets_EmptyIsNonNil(t *testing.T) {
	s := setupTestDB(t)
	list, err := s.ListBudgets(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestListBudgets_ReflectsAccountRename(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.UpsertBudget(ctx, foodID, "2026-01", 1, false)
	require.NoError(t, err)

	require.NoError(t, s.RenameAccount(ctx, "Expenses:Food", "Expenses:Groceries"))

	list, err := s.ListBudgets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Expenses:Groceries", list[0].AccountName)
}

func TestDeleteBudget(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	foodID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)
	id, err := s.UpsertBudget(ctx, foodID, "2026-01", 1, false)
	require.NoError(t, err)

	require.NoError(t, s.DeleteBudget(ctx, id))
	err = s.DeleteBudget(ctx, id)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
```

- [ ] **Step 5: Run tests to verify they fail**

Run: `go test ./internal/store/ -run 'Budget' -v`
Expected: FAIL to compile (`s.UpsertBudget undefined`).

- [ ] **Step 6: Implement the store** (`internal/store/sqlite_budget.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
)

// UpsertBudget inserts or replaces the budget version for (accountID, month).
func (s *Store) UpsertBudget(ctx context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO budgets (account_id, effective_month, amount, stopped)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id, effective_month) DO UPDATE
		SET amount = excluded.amount, stopped = excluded.stopped
		RETURNING id`, accountID, month, amount, stopped).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert budget: %w", err)
	}
	return id, nil
}

// ListBudgets returns all budget versions with their account names.
func (s *Store) ListBudgets(ctx context.Context) ([]model.Budget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.id, b.account_id, a.name, b.effective_month, b.amount, b.stopped
		FROM budgets b
		JOIN accounts a ON a.id = b.account_id
		ORDER BY a.name, b.effective_month`)
	if err != nil {
		return nil, fmt.Errorf("failed to list budgets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []model.Budget{}
	for rows.Next() {
		var b model.Budget
		if err := rows.Scan(&b.ID, &b.AccountID, &b.AccountName, &b.EffectiveMonth, &b.Amount, &b.Stopped); err != nil {
			return nil, fmt.Errorf("failed to scan budget: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate budgets: %w", err)
	}
	return out, nil
}

// DeleteBudget removes one budget version.
func (s *Store) DeleteBudget(ctx context.Context, id int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM budgets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete budget: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("budget with ID %d not found: %w", id, ErrRecordNotFound)
	}
	return nil
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/store/ ./internal/service/ && go build ./... && go vet ./...`
Expected: PASS, no vet findings.

- [ ] **Step 8: Commit**

```bash
git add internal/model/budget.go internal/repository/interfaces.go internal/store/sqlite_budget.go internal/store/sqlite_budget_test.go internal/service/testhelper_test.go
git commit -m "feat(store): add budget repository"
```

---

### Task 3: BudgetService CRUD and facade wiring

**Files:**
- Create: `internal/service/budget_service.go`
- Modify: `internal/service/service.go`
- Modify: `internal/app/app.go:76`
- Modify: `internal/api/testhelper_test.go` (three `service.NewService(st, st, st, cfg)` calls)
- Test: `internal/service/budget_service_test.go`
- Modify: `internal/service/testhelper_test.go` (add `newTestBudgetService`)

**Interfaces:**
- Consumes: Task 2 types, `mockBudgetRepo`.
- Produces:
  ```go
  func NewBudgetService(budgetRepo repository.BudgetRepository, accRepo repository.AccountRepository, txRepo repository.TransactionRepository, tm repository.TransactionManager, cfg *config.Config) *BudgetService
  func (bs *BudgetService) SetBudget(ctx context.Context, input model.SetBudgetInput) (*model.Budget, error)
  func (bs *BudgetService) StopBudget(ctx context.Context, input model.StopBudgetInput) (*model.Budget, error)
  func (bs *BudgetService) ListBudgets(ctx context.Context) ([]model.Budget, error)
  func (bs *BudgetService) DeleteBudget(ctx context.Context, id int64) error
  func ValidateBudgetMonth(month string) error   // exported: CLI uses it for flag validation
  func activeBudgets(budgets []model.Budget, month string) []model.Budget // unexported; Task 4 uses it
  func NewService(accRepo repository.AccountRepository, txRepo repository.TransactionRepository, budgetRepo repository.BudgetRepository, tm repository.TransactionManager, cfg *config.Config) *Service
  func (s *Service) Budget() *BudgetService
  ```

- [ ] **Step 1: Add the test factory** (`internal/service/testhelper_test.go`, under "Helper factories")

```go
func newTestBudgetService(accRepo *mockAccountRepo, txRepo *mockTransactionRepo, budgetRepo *mockBudgetRepo) *BudgetService {
	tm := &mockTransactionManager{accRepo: accRepo, txRepo: txRepo, budgetRepo: budgetRepo}
	return NewBudgetService(budgetRepo, accRepo, txRepo, tm, defaultConfig())
}
```

- [ ] **Step 2: Write the failing tests** (`internal/service/budget_service_test.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func budgetFixture() (*mockAccountRepo, *mockTransactionRepo, *mockBudgetRepo) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"})
	return accRepo, newMockTransactionRepo(), newMockBudgetRepo(accRepo)
}

func requireValidationField(t *testing.T, err error, field string) {
	t.Helper()
	var verr *ValidationError
	require.True(t, errors.As(err, &verr), "expected ValidationError, got %v", err)
	assert.Equal(t, field, verr.Field)
}

func TestValidateBudgetMonth(t *testing.T) {
	assert.NoError(t, ValidateBudgetMonth("2026-01"))
	assert.NoError(t, ValidateBudgetMonth("2026-12"))
	for _, bad := range []string{"", "2026-1", "2026-13", "2026-00", "26-01", "2026/01", "2026-01-01"} {
		requireValidationField(t, ValidateBudgetMonth(bad), "effective_month")
	}
}

func TestSetBudget(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a version", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		b, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 800000})
		require.NoError(t, err)
		assert.Equal(t, int64(1), b.AccountID)
		assert.Equal(t, "Expenses:Food", b.AccountName)
		assert.Equal(t, int64(800000), b.Amount)
		assert.False(t, b.Stopped)
	})

	t.Run("zero amount is allowed", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 0})
		require.NoError(t, err)
	})

	t.Run("negative amount", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: -1})
		requireValidationField(t, err, "amount")
	})

	t.Run("bad month", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-13", Amount: 1})
		requireValidationField(t, err, "effective_month")
	})

	t.Run("empty account name", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "  ", EffectiveMonth: "2026-01", Amount: 1})
		requireValidationField(t, err, "account_name")
	})

	t.Run("unknown account", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Nope", EffectiveMonth: "2026-01", Amount: 1})
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("non-expense account", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Assets:Bank", EffectiveMonth: "2026-01", Amount: 1})
		requireValidationField(t, err, "account_name")
	})

	t.Run("set after stop in the same month clears stopped", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 100})
		require.NoError(t, err)
		_, err = svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-03"})
		require.NoError(t, err)
		b, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-03", Amount: 200})
		require.NoError(t, err)
		assert.False(t, b.Stopped)
		active := activeBudgets(mustList(t, svc), "2026-03")
		require.Len(t, active, 1)
		assert.Equal(t, int64(200), active[0].Amount)
	})
}

func mustList(t *testing.T, svc *BudgetService) []model.Budget {
	t.Helper()
	list, err := svc.ListBudgets(context.Background())
	require.NoError(t, err)
	return list
}

func TestStopBudget(t *testing.T) {
	ctx := context.Background()

	t.Run("stops an active budget", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 100})
		require.NoError(t, err)
		b, err := svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-04"})
		require.NoError(t, err)
		assert.True(t, b.Stopped)
		assert.Equal(t, int64(0), b.Amount)
		assert.Empty(t, activeBudgets(mustList(t, svc), "2026-04"))
		assert.Len(t, activeBudgets(mustList(t, svc), "2026-03"), 1)
	})

	t.Run("no budget yet", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-04"})
		requireValidationField(t, err, "effective_month")
	})

	t.Run("already stopped", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 100})
		require.NoError(t, err)
		_, err = svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-02"})
		require.NoError(t, err)
		_, err = svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-05"})
		requireValidationField(t, err, "effective_month")
	})

	t.Run("unknown account", func(t *testing.T) {
		accRepo, txRepo, bRepo := budgetFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.StopBudget(ctx, model.StopBudgetInput{AccountName: "Expenses:Nope", EffectiveMonth: "2026-04"})
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestActiveBudgets(t *testing.T) {
	list := []model.Budget{
		{ID: 1, AccountID: 1, EffectiveMonth: "2026-01", Amount: 100},
		{ID: 2, AccountID: 1, EffectiveMonth: "2026-04", Amount: 200},
		{ID: 3, AccountID: 1, EffectiveMonth: "2026-07", Stopped: true},
		{ID: 4, AccountID: 1, EffectiveMonth: "2026-10", Amount: 300},
	}
	amountAt := func(month string) int64 {
		got := activeBudgets(list, month)
		if len(got) == 0 {
			return -1
		}
		return got[0].Amount
	}
	assert.Equal(t, int64(-1), amountAt("2025-12"), "before the first version")
	assert.Equal(t, int64(100), amountAt("2026-03"))
	assert.Equal(t, int64(200), amountAt("2026-04"))
	assert.Equal(t, int64(-1), amountAt("2026-08"), "after stop")
	assert.Equal(t, int64(300), amountAt("2026-11"), "restarted after stop")
}

func TestDeleteBudget_Service(t *testing.T) {
	ctx := context.Background()
	accRepo, txRepo, bRepo := budgetFixture()
	svc := newTestBudgetService(accRepo, txRepo, bRepo)
	b, err := svc.SetBudget(ctx, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 1})
	require.NoError(t, err)

	require.NoError(t, svc.DeleteBudget(ctx, b.ID))
	assert.ErrorIs(t, svc.DeleteBudget(ctx, b.ID), ErrNotFound)
}

func TestListBudgets_Service(t *testing.T) {
	accRepo, txRepo, bRepo := budgetFixture()
	svc := newTestBudgetService(accRepo, txRepo, bRepo)
	list, err := svc.ListBudgets(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, list)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/service/ -run 'Budget' -v`
Expected: FAIL to compile (`NewBudgetService undefined`).

- [ ] **Step 4: Implement the service** (`internal/service/budget_service.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

var budgetMonthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// BudgetService manages monthly budgets on Expense accounts and reports
// budget vs actual spending.
type BudgetService struct {
	budgetRepo repository.BudgetRepository
	accRepo    repository.AccountRepository
	txRepo     repository.TransactionRepository
	tm         repository.TransactionManager
	config     *config.Config
}

func NewBudgetService(
	budgetRepo repository.BudgetRepository,
	accRepo repository.AccountRepository,
	txRepo repository.TransactionRepository,
	tm repository.TransactionManager,
	cfg *config.Config,
) *BudgetService {
	return &BudgetService{budgetRepo: budgetRepo, accRepo: accRepo, txRepo: txRepo, tm: tm, config: cfg}
}

// ValidateBudgetMonth checks that month is YYYY-MM with a month of 01-12.
func ValidateBudgetMonth(month string) error {
	if !budgetMonthPattern.MatchString(month) {
		return validationErrorf("effective_month", "invalid month %q, expected YYYY-MM", month)
	}
	return nil
}

// SetBudget creates or replaces the budget version for an account starting at
// input.EffectiveMonth.
func (bs *BudgetService) SetBudget(ctx context.Context, input model.SetBudgetInput) (*model.Budget, error) {
	name := strings.TrimSpace(input.AccountName)
	if name == "" {
		return nil, validationErrorf("account_name", "account name is required")
	}
	if err := ValidateBudgetMonth(input.EffectiveMonth); err != nil {
		return nil, err
	}
	if input.Amount < 0 {
		return nil, validationErrorf("amount", "budget amount must not be negative")
	}

	var out *model.Budget
	err := bs.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, err := budgetAccount(ctx, repo, name)
		if err != nil {
			return err
		}
		id, err := repo.UpsertBudget(ctx, acc.ID, input.EffectiveMonth, input.Amount, false)
		if err != nil {
			return fmt.Errorf("failed to save budget: %w", err)
		}
		out = &model.Budget{ID: id, AccountID: acc.ID, AccountName: acc.Name, EffectiveMonth: input.EffectiveMonth, Amount: input.Amount}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StopBudget ends an account's active budget from input.EffectiveMonth on.
func (bs *BudgetService) StopBudget(ctx context.Context, input model.StopBudgetInput) (*model.Budget, error) {
	name := strings.TrimSpace(input.AccountName)
	if name == "" {
		return nil, validationErrorf("account_name", "account name is required")
	}
	if err := ValidateBudgetMonth(input.EffectiveMonth); err != nil {
		return nil, err
	}

	var out *model.Budget
	err := bs.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, err := budgetAccount(ctx, repo, name)
		if err != nil {
			return err
		}
		all, err := repo.ListBudgets(ctx)
		if err != nil {
			return fmt.Errorf("failed to list budgets: %w", err)
		}
		if !hasActiveBudget(all, acc.ID, input.EffectiveMonth) {
			return validationErrorf("effective_month", "account %q has no active budget in %s", acc.Name, input.EffectiveMonth)
		}
		id, err := repo.UpsertBudget(ctx, acc.ID, input.EffectiveMonth, 0, true)
		if err != nil {
			return fmt.Errorf("failed to stop budget: %w", err)
		}
		out = &model.Budget{ID: id, AccountID: acc.ID, AccountName: acc.Name, EffectiveMonth: input.EffectiveMonth, Stopped: true}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListBudgets returns every budget version, ordered by account name then month.
func (bs *BudgetService) ListBudgets(ctx context.Context) ([]model.Budget, error) {
	list, err := bs.budgetRepo.ListBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list budgets: %w", err)
	}
	if list == nil {
		list = []model.Budget{}
	}
	return list, nil
}

// DeleteBudget removes a single budget version.
func (bs *BudgetService) DeleteBudget(ctx context.Context, id int64) error {
	if err := bs.budgetRepo.DeleteBudget(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("budget %d: %w", id, ErrNotFound)
		}
		return fmt.Errorf("failed to delete budget: %w", err)
	}
	return nil
}

// budgetAccount resolves name to an Expense account.
func budgetAccount(ctx context.Context, repo repository.AccountRepository, name string) (*model.Account, error) {
	acc, err := repo.GetAccountByName(ctx, name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("account %q: %w", name, ErrNotFound)
		}
		return nil, fmt.Errorf("failed to look up account %q: %w", name, err)
	}
	if acc.Type != model.AccountTypeExpense {
		return nil, validationErrorf("account_name", "budgets can only be set on Expense accounts; %q is type %s", acc.Name, acc.Type)
	}
	return acc, nil
}

// latestVersions returns, per account, the version with the greatest
// EffectiveMonth <= month (including stopped versions).
func latestVersions(budgets []model.Budget, month string) map[int64]model.Budget {
	latest := map[int64]model.Budget{}
	for _, b := range budgets {
		if b.EffectiveMonth > month {
			continue
		}
		if cur, ok := latest[b.AccountID]; !ok || b.EffectiveMonth > cur.EffectiveMonth {
			latest[b.AccountID] = b
		}
	}
	return latest
}

// activeBudgets returns the non-stopped version that applies to month for
// each account, in the input order.
func activeBudgets(budgets []model.Budget, month string) []model.Budget {
	latest := latestVersions(budgets, month)
	out := []model.Budget{}
	for _, b := range budgets {
		if v, ok := latest[b.AccountID]; ok && v.ID == b.ID && !b.Stopped {
			out = append(out, b)
		}
	}
	return out
}

func hasActiveBudget(budgets []model.Budget, accountID int64, month string) bool {
	v, ok := latestVersions(budgets, month)[accountID]
	return ok && !v.Stopped
}
```

- [ ] **Step 5: Wire the facade** (`internal/service/service.go`)

```go
type Service struct {
	account     *AccountService
	transaction *TransactionService
	budget      *BudgetService
	config      *config.Config
}

func NewService(
	accRepo repository.AccountRepository,
	txRepo repository.TransactionRepository,
	budgetRepo repository.BudgetRepository,
	tm repository.TransactionManager,
	cfg *config.Config,
) *Service {
	svc := &Service{
		account:     NewAccountService(accRepo, cfg, tm),
		transaction: NewTransactionService(txRepo, accRepo, tm, cfg),
		budget:      NewBudgetService(budgetRepo, accRepo, txRepo, tm, cfg),
		config:      cfg,
	}

	return svc
}

func (s *Service) Account() *AccountService         { return s.account }
func (s *Service) Transaction() *TransactionService { return s.transaction }
func (s *Service) Budget() *BudgetService           { return s.budget }
func (s *Service) Config() *config.Config            { return s.config }
```

Update callers to pass the store as the budget repository:
- `internal/app/app.go`: `svc := service.NewService(dbStore, dbStore, dbStore, dbStore, cfg)`
- `internal/api/testhelper_test.go`: each `service.NewService(st, st, st, cfg)` becomes `service.NewService(st, st, st, st, cfg)`.

Run `grep -rn "NewService(" --include='*.go' .` to confirm no caller was missed.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go build ./... && go vet ./... && go test ./internal/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/service/budget_service.go internal/service/budget_service_test.go internal/service/service.go internal/service/testhelper_test.go internal/app/app.go internal/api/testhelper_test.go
git commit -m "feat(service): add BudgetService with versioned monthly budgets"
```

---

### Task 4: Budget report

**Files:**
- Create: `internal/service/budget_report.go`
- Test: `internal/service/budget_report_test.go`

**Interfaces:**
- Consumes: `activeBudgets`, `parseMonth` (in `report_service.go`), `addTxSplits` and `split` test helpers (`report_service_test.go`, `transaction_classifier_test.go`; `split` sets `Currency: "USD"`).
- Produces: `func (bs *BudgetService) GenerateBudgetReport(ctx context.Context, month string) (*model.BudgetReport, error)` — empty `month` = current local month.

- [ ] **Step 1: Write the failing tests** (`internal/service/budget_report_test.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolp(b bool) *bool { return &b }

// reportFixture: Expenses:Food (USD) with children Dining (USD) and Japan (JPY),
// Expenses:FoodTruck (USD, a name-prefix trap), Expenses:Rent (USD).
func reportFixture() (*mockAccountRepo, *mockTransactionRepo, *mockBudgetRepo) {
	accRepo := newMockAccountRepo()
	for _, a := range []*model.Account{
		{ID: 1, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"},
		{ID: 2, Name: "Expenses:Food:Dining", Type: model.AccountTypeExpense, Currency: "USD"},
		{ID: 3, Name: "Expenses:Food:Japan", Type: model.AccountTypeExpense, Currency: "JPY"},
		{ID: 4, Name: "Expenses:FoodTruck", Type: model.AccountTypeExpense, Currency: "USD"},
		{ID: 5, Name: "Expenses:Rent", Type: model.AccountTypeExpense, Currency: ""}, // default currency (USD)
		{ID: 6, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"},
	} {
		accRepo.addAccount(a)
	}
	return accRepo, newMockTransactionRepo(), newMockBudgetRepo(accRepo)
}

func addExpenseTx(txRepo *mockTransactionRepo, id int64, txType model.TransactionType, regular *bool, splits ...model.SplitDetail) {
	addTxSplits(txRepo.splitsWithAccts, id, splits...)
	txRepo.addTransaction(&model.Transaction{ID: id, Type: txType, Regular: regular}, nil)
}

func findRow(t *testing.T, r *model.BudgetReport, name string) model.BudgetReportRow {
	t.Helper()
	for _, row := range r.Rows {
		if row.AccountName == name {
			return row
		}
	}
	t.Fatalf("row %q not found", name)
	return model.BudgetReportRow{}
}

func TestGenerateBudgetReport(t *testing.T) {
	ctx := context.Background()

	t.Run("aggregates descendants, signed, split by regular", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100000, false)
		addExpenseTx(txRepo, 1, model.TxTypeExpense, boolp(true),
			split("Expenses:Food", model.AccountTypeExpense, 3000),
			split("Assets:Bank", model.AccountTypeAsset, -3000))
		addExpenseTx(txRepo, 2, model.TxTypeExpense, boolp(false),
			split("Expenses:Food:Dining", model.AccountTypeExpense, 5000),
			split("Assets:Bank", model.AccountTypeAsset, -5000))
		// Refund reduces actual.
		addExpenseTx(txRepo, 3, model.TxTypeExpense, boolp(false),
			split("Expenses:Food:Dining", model.AccountTypeExpense, -1000),
			split("Assets:Bank", model.AccountTypeAsset, 1000))
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, "2026-03", r.Month)
		row := findRow(t, r, "Expenses:Food")
		assert.Equal(t, "USD", row.Currency)
		assert.Equal(t, "2026-01", row.EffectiveMonth)
		assert.Equal(t, int64(100000), row.Budget)
		assert.Equal(t, int64(7000), row.Actual)
		assert.Equal(t, int64(3000), row.ActualRegular)
		assert.Equal(t, int64(4000), row.ActualIrregular)
		assert.Equal(t, int64(93000), row.Remaining)
	})

	t.Run("name prefix is not a descendant", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100, false)
		addExpenseTx(txRepo, 1, model.TxTypeExpense, boolp(true),
			split("Expenses:FoodTruck", model.AccountTypeExpense, 900),
			split("Assets:Bank", model.AccountTypeAsset, -900))
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, int64(0), findRow(t, r, "Expenses:Food").Actual)
	})

	t.Run("other-currency descendants are excluded and listed", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100, false)
		jpy := split("Expenses:Food:Japan", model.AccountTypeExpense, 50000)
		jpy.Currency = "JPY"
		bank := split("Assets:Bank", model.AccountTypeAsset, -50000)
		bank.Currency = "JPY"
		addExpenseTx(txRepo, 1, model.TxTypeExpense, boolp(true), jpy, bank)
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-01")
		require.NoError(t, err)
		row := findRow(t, r, "Expenses:Food")
		assert.Equal(t, int64(0), row.Actual)
		assert.Equal(t, []string{"Expenses:Food:Japan"}, row.ExcludedAccounts)
	})

	t.Run("only Expense-typed transactions count", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100, false)
		for i, typ := range []model.TransactionType{model.TxTypeTransfer, model.TxTypeInvestment, model.TxTypeOther, model.TxTypeOpening} {
			addExpenseTx(txRepo, int64(i+1), typ, nil,
				split("Expenses:Food", model.AccountTypeExpense, 700),
				split("Assets:Bank", model.AccountTypeAsset, -700))
		}
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, int64(0), findRow(t, r, "Expenses:Food").Actual)
	})

	t.Run("empty account currency uses default", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 5, "2026-01", 2000, false)
		addExpenseTx(txRepo, 1, model.TxTypeExpense, boolp(true),
			split("Expenses:Rent", model.AccountTypeExpense, 1500),
			split("Assets:Bank", model.AccountTypeAsset, -1500))
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-01")
		require.NoError(t, err)
		row := findRow(t, r, "Expenses:Rent")
		assert.Equal(t, "USD", row.Currency)
		assert.Equal(t, int64(1500), row.Actual)
	})

	t.Run("totals count only top-level budgeted rows", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 10000, false) // Food
		_, _ = bRepo.UpsertBudget(ctx, 2, "2026-01", 6000, false)  // Food:Dining
		_, _ = bRepo.UpsertBudget(ctx, 5, "2026-01", 20000, false) // Rent
		addExpenseTx(txRepo, 1, model.TxTypeExpense, boolp(true),
			split("Expenses:Food:Dining", model.AccountTypeExpense, 4000),
			split("Assets:Bank", model.AccountTypeAsset, -4000))
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-01")
		require.NoError(t, err)
		require.Len(t, r.Rows, 3)
		assert.Equal(t, []string{"Expenses:Food", "Expenses:Food:Dining", "Expenses:Rent"},
			[]string{r.Rows[0].AccountName, r.Rows[1].AccountName, r.Rows[2].AccountName})
		assert.Equal(t, int64(30000), r.TotalBudget["USD"])
		assert.Equal(t, int64(4000), r.TotalActual["USD"])
	})

	t.Run("stopped and future versions are not reported", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100, false)
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-02", 0, true)
		_, _ = bRepo.UpsertBudget(ctx, 5, "2026-09", 100, false)
		svc := newTestBudgetService(accRepo, txRepo, bRepo)

		r, err := svc.GenerateBudgetReport(ctx, "2026-03")
		require.NoError(t, err)
		assert.NotNil(t, r.Rows)
		assert.Empty(t, r.Rows)
		assert.NotNil(t, r.TotalBudget)
		assert.NotNil(t, r.TotalActual)
	})

	t.Run("default month is the current local month", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		r, err := svc.GenerateBudgetReport(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, time.Now().Format("2006-01"), r.Month)
	})

	t.Run("invalid month", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.GenerateBudgetReport(ctx, "2026-13")
		var verr *ValidationError
		require.True(t, errors.As(err, &verr))
		assert.Equal(t, "month", verr.Field)
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		accRepo, txRepo, bRepo := reportFixture()
		_, _ = bRepo.UpsertBudget(ctx, 1, "2026-01", 100, false)
		txRepo.splitsRangeErr = errors.New("boom")
		svc := newTestBudgetService(accRepo, txRepo, bRepo)
		_, err := svc.GenerateBudgetReport(ctx, "2026-01")
		assert.ErrorContains(t, err, "boom")
	})
}
```

Note: the mock `GetSplitsWithAccountsByDateRange` / `GetTransactionsByDateRange` ignore the date range; date filtering is the store's job and is already covered by store tests.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestGenerateBudgetReport -v`
Expected: FAIL to compile (`GenerateBudgetReport undefined`).

- [ ] **Step 3: Implement** (`internal/service/budget_report.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hance08/kea/internal/model"
)

// GenerateBudgetReport compares every budget active in month (YYYY-MM, local
// time; empty means the current month) with actual spending. Actual uses the
// same transaction scope as the expense report (Expense splits of
// Expense-typed transactions) but sums signed amounts, so refunds reduce it.
func (bs *BudgetService) GenerateBudgetReport(ctx context.Context, month string) (*model.BudgetReport, error) {
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	if err := ValidateBudgetMonth(month); err != nil {
		return nil, validationErrorf("month", "invalid month %q, expected YYYY-MM", month)
	}
	start, end, _, err := parseMonth(month)
	if err != nil {
		return nil, err
	}

	report := &model.BudgetReport{
		Month:       month,
		Rows:        []model.BudgetReportRow{},
		TotalBudget: map[string]int64{},
		TotalActual: map[string]int64{},
	}

	all, err := bs.budgetRepo.ListBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list budgets: %w", err)
	}
	active := activeBudgets(all, month)
	if len(active) == 0 {
		return report, nil
	}

	accounts, err := bs.accRepo.GetAllAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load accounts: %w", err)
	}
	accByID := make(map[int64]*model.Account, len(accounts))
	for _, a := range accounts {
		accByID[a.ID] = a
	}

	splitsByTx, err := bs.txRepo.GetSplitsWithAccountsByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load splits: %w", err)
	}
	txs, err := bs.txRepo.GetTransactionsByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load transactions: %w", err)
	}
	txByID := make(map[int64]*model.Transaction, len(txs))
	for _, tx := range txs {
		txByID[tx.ID] = tx
	}

	for _, b := range active {
		acc, ok := accByID[b.AccountID]
		if !ok {
			continue
		}
		row := model.BudgetReportRow{
			AccountID:        acc.ID,
			AccountName:      acc.Name,
			Currency:         bs.currencyOrDefault(acc.Currency),
			EffectiveMonth:   b.EffectiveMonth,
			Budget:           b.Amount,
			ExcludedAccounts: []string{},
		}
		excluded := map[string]struct{}{}
		for txID, details := range splitsByTx {
			tx, ok := txByID[txID]
			if !ok || tx.Type != model.TxTypeExpense {
				continue
			}
			regular := tx.Regular != nil && *tx.Regular
			for _, d := range details {
				if d.AccountType != model.AccountTypeExpense || !isSelfOrDescendant(d.AccountName, acc.Name) {
					continue
				}
				if bs.currencyOrDefault(d.Currency) != row.Currency {
					excluded[d.AccountName] = struct{}{}
					continue
				}
				row.Actual += d.Amount
				if regular {
					row.ActualRegular += d.Amount
				} else {
					row.ActualIrregular += d.Amount
				}
			}
		}
		for name := range excluded {
			row.ExcludedAccounts = append(row.ExcludedAccounts, name)
		}
		sort.Strings(row.ExcludedAccounts)
		row.Remaining = row.Budget - row.Actual
		report.Rows = append(report.Rows, row)
	}

	sort.Slice(report.Rows, func(i, j int) bool { return report.Rows[i].AccountName < report.Rows[j].AccountName })

	for _, row := range report.Rows {
		if hasBudgetedAncestor(row.AccountName, report.Rows) {
			continue
		}
		report.TotalBudget[row.Currency] += row.Budget
		report.TotalActual[row.Currency] += row.Actual
	}
	return report, nil
}

func (bs *BudgetService) currencyOrDefault(ccy string) string {
	if ccy == "" {
		return bs.config.Defaults.Currency
	}
	return ccy
}

// isSelfOrDescendant reports whether name is ancestor or below it in the
// account tree. "Expenses:FoodTruck" is not below "Expenses:Food".
func isSelfOrDescendant(name, ancestor string) bool {
	return name == ancestor || strings.HasPrefix(name, ancestor+":")
}

func hasBudgetedAncestor(name string, rows []model.BudgetReportRow) bool {
	for _, r := range rows {
		if r.AccountName != name && isSelfOrDescendant(name, r.AccountName) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ && go vet ./internal/service/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/budget_report.go internal/service/budget_report_test.go
git commit -m "feat(service): add budget vs actual report"
```

---

### Task 5: HTTP API

**Files:**
- Create: `internal/api/budgets.go`
- Modify: `internal/api/router.go` (routes)
- Test: `internal/api/budgets_test.go`
- Modify: `docs/http-api.md` (Budgets section + `/reports/budget` row)

**Interfaces:**
- Consumes: `svc.Budget()` methods from Tasks 3–4; helpers `decodeJSON`, `writeJSON`, `parseInt64Path`; test helpers `newServerWithStore`, `seedAccount`, `seedTransaction`, `getJSON`, `postJSON`, `deleteURL`.
- Produces routes:
  - `GET /api/budgets` -> `200 {"items": [Budget]}`
  - `PUT /api/budgets` (`SetBudgetInput`) -> `200 Budget`
  - `POST /api/budgets/stop` (`StopBudgetInput`) -> `200 Budget`
  - `DELETE /api/budgets/{id}` -> `200 {"deleted": true, "id": <id>}`
  - `GET /api/reports/budget?month=YYYY-MM` -> `200 BudgetReport`

  **Spec deviation (deliberate):** the spec says `DELETE` returns `204`. Every other DELETE in this API returns `200` with a JSON body (`handleDeleteAccount`), and the SPA's `apiFetch` always calls `resp.json()`, which throws on an empty 204 body. Use `200 {"deleted": true, "id": id}` and update the spec's HTTP API table in this task's commit.

- [ ] **Step 1: Write the failing tests** (`internal/api/budgets_test.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
)

func putJSON(t *testing.T, url string, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, b
}

func decodeErrBody(t *testing.T, b []byte) errorBody {
	t.Helper()
	var e errorBody
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("decode error body %s: %v", b, err)
	}
	return e
}

func TestHandleSetBudget_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Expenses:Food", model.AccountTypeExpense, 0)

	status, body := putJSON(t, ts.URL+"/api/budgets",
		`{"account_name":"Expenses:Food","effective_month":"2026-01","amount":800000}`)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var got model.Budget
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.AccountName != "Expenses:Food" || got.Amount != 800000 || got.EffectiveMonth != "2026-01" {
		t.Errorf("unexpected budget: %+v", got)
	}
}

func TestHandleSetBudget_ValidationAndNotFound(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Expenses:Food", model.AccountTypeExpense, 0)
	seedAccount(t, svc, "Assets:Bank", model.AccountTypeAsset, 0)

	cases := []struct {
		name   string
		body   string
		status int
		field  string
	}{
		{"negative amount", `{"account_name":"Expenses:Food","effective_month":"2026-01","amount":-1}`, 400, "amount"},
		{"bad month", `{"account_name":"Expenses:Food","effective_month":"2026-13","amount":1}`, 400, "effective_month"},
		{"missing month", `{"account_name":"Expenses:Food","amount":1}`, 400, "effective_month"},
		{"non-expense", `{"account_name":"Assets:Bank","effective_month":"2026-01","amount":1}`, 400, "account_name"},
		{"unknown field", `{"account_name":"Expenses:Food","effective_month":"2026-01","amount":1,"x":1}`, 400, "body"},
		{"unknown account", `{"account_name":"Expenses:Nope","effective_month":"2026-01","amount":1}`, 404, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := putJSON(t, ts.URL+"/api/budgets", tc.body)
			if status != tc.status {
				t.Fatalf("status %d, want %d: %s", status, tc.status, body)
			}
			if tc.field != "" && decodeErrBody(t, body).Field != tc.field {
				t.Errorf("field = %q, want %q", decodeErrBody(t, body).Field, tc.field)
			}
		})
	}
}

func TestHandleListStopDeleteBudgets(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Expenses:Food", model.AccountTypeExpense, 0)

	status, body := getJSON(t, ts.URL+"/api/budgets")
	if status != http.StatusOK || string(bytes.TrimSpace(body)) != `{"items":[]}` {
		t.Fatalf("empty list: %d %s", status, body)
	}

	if status, body := putJSON(t, ts.URL+"/api/budgets",
		`{"account_name":"Expenses:Food","effective_month":"2026-01","amount":100}`); status != 200 {
		t.Fatalf("set: %d %s", status, body)
	}

	status, body = postJSON(t, ts.URL+"/api/budgets/stop",
		map[string]string{"account_name": "Expenses:Food", "effective_month": "2026-03"})
	if status != http.StatusOK {
		t.Fatalf("stop: %d %s", status, body)
	}
	var stopped model.Budget
	_ = json.Unmarshal(body, &stopped)
	if !stopped.Stopped {
		t.Errorf("expected stopped budget, got %+v", stopped)
	}

	status, body = postJSON(t, ts.URL+"/api/budgets/stop",
		map[string]string{"account_name": "Expenses:Food", "effective_month": "2026-05"})
	if status != http.StatusBadRequest || decodeErrBody(t, body).Field != "effective_month" {
		t.Fatalf("stop twice: %d %s", status, body)
	}

	status, body = getJSON(t, ts.URL+"/api/budgets")
	var list struct{ Items []model.Budget }
	_ = json.Unmarshal(body, &list)
	if status != 200 || len(list.Items) != 2 {
		t.Fatalf("list: %d %s", status, body)
	}

	id := list.Items[0].ID
	status, body = deleteURL(t, ts.URL+"/api/budgets/"+itoa(id))
	if status != http.StatusOK {
		t.Fatalf("delete: %d %s", status, body)
	}
	status, _ = deleteURL(t, ts.URL+"/api/budgets/"+itoa(id))
	if status != http.StatusNotFound {
		t.Errorf("delete again: %d, want 404", status)
	}
	status, _ = deleteURL(t, ts.URL+"/api/budgets/abc")
	if status != http.StatusBadRequest {
		t.Errorf("delete bad id: %d, want 400", status)
	}
}

func TestHandleBudgetReport(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	seedAccount(t, svc, "Expenses:Food", model.AccountTypeExpense, 0)
	if _, err := svc.Budget().SetBudget(t.Context(), model.SetBudgetInput{
		AccountName: "Expenses:Food", EffectiveMonth: "2026-05", Amount: 5000,
	}); err != nil {
		t.Fatal(err)
	}
	ts15 := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local).Unix()
	seedTransaction(t, svc, "Assets:Cash", "Expenses:Food", 1200, ts15, "lunch", model.TxTypeExpense, model.StatusCleared)

	status, body := getJSON(t, ts.URL+"/api/reports/budget?month=2026-05")
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var got model.BudgetReport
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || got.Rows[0].Actual != 1200 || got.Rows[0].Remaining != 3800 {
		t.Fatalf("unexpected report: %s", body)
	}
	if got.Rows[0].ExcludedAccounts == nil {
		t.Errorf("excluded_accounts must serialize as []")
	}

	status, body = getJSON(t, ts.URL+"/api/reports/budget")
	if status != http.StatusOK {
		t.Fatalf("default month: %d %s", status, body)
	}
	_ = json.Unmarshal(body, &got)
	if got.Month != time.Now().Format("2006-01") {
		t.Errorf("default month = %q", got.Month)
	}

	status, body = getJSON(t, ts.URL+"/api/reports/budget?month=2026-13")
	if status != http.StatusBadRequest || decodeErrBody(t, body).Field != "month" {
		t.Errorf("bad month: %d %s", status, body)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -run 'Budget' -v`
Expected: FAIL (404s / route not found).

- [ ] **Step 3: Implement handlers** (`internal/api/budgets.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

type budgetListResponse struct {
	Items []model.Budget `json:"items"`
}

func (s *Server) handleListBudgets(w http.ResponseWriter, r *http.Request) error {
	items, err := s.svc.Budget().ListBudgets(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, budgetListResponse{Items: items})
}

func (s *Server) handleSetBudget(w http.ResponseWriter, r *http.Request) error {
	var input model.SetBudgetInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	b, err := s.svc.Budget().SetBudget(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleStopBudget(w http.ResponseWriter, r *http.Request) error {
	var input model.StopBudgetInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	b, err := s.svc.Budget().StopBudget(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleDeleteBudget(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.Budget().DeleteBudget(r.Context(), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

func (s *Server) handleBudgetReport(w http.ResponseWriter, r *http.Request) error {
	report, err := s.svc.Budget().GenerateBudgetReport(r.Context(), r.URL.Query().Get("month"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, report)
}
```

Register in `internal/api/router.go` (inside `r.Route("/api", ...)`, after the `/reports/net-worth-series` line):

```go
		r.Method(http.MethodGet, "/reports/budget", apiHandler(s.handleBudgetReport))
		r.Method(http.MethodGet, "/budgets", apiHandler(s.handleListBudgets))
		r.Method(http.MethodPut, "/budgets", apiHandler(s.handleSetBudget))
		r.Method(http.MethodPost, "/budgets/stop", apiHandler(s.handleStopBudget))
		r.Method(http.MethodDelete, "/budgets/{id}", apiHandler(s.handleDeleteBudget))
```

If `internal/api/router_test.go` asserts the full route list, add these five routes there.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/api/ && go vet ./internal/api/`
Expected: PASS.

- [ ] **Step 5: Update docs**

In `docs/http-api.md`:
- Under `### Reports`, add a row/bullet for `GET /api/reports/budget` — query `month` (`YYYY-MM`, default current local month), response `BudgetReport` (fields as in `internal/model/budget.go`), totals per currency over top-level rows only.
- Add a `### Budgets` section after `### Reconcile` listing the four budget endpoints with request/response shapes and errors (400 `validation_failed` with `field` in `account_name|effective_month|amount|body`, 404 `not_found`), following the format of the neighboring sections.

In `docs/history/superpowers/specs/2026-10-05-monthly-budget-design.md`, change the DELETE row's response from `204` to `200 {"deleted": true, "id": <id>}` and add one sentence: "Matches the other DELETE endpoints; `apiFetch` in the SPA always parses a JSON body."

Run: `scripts/check-docs.sh`
Expected: `check-docs: all referenced paths exist`.

- [ ] **Step 6: Commit**

```bash
git add internal/api/budgets.go internal/api/budgets_test.go internal/api/router.go docs/http-api.md docs/history/superpowers/specs/2026-10-05-monthly-budget-design.md
git commit -m "feat(api): add budget endpoints and budget report"
```

(Include `internal/api/router_test.go` if you changed it.)

---

### Task 6: CLI views and JSON DTOs

**Files:**
- Create: `ui/views/budget.go`
- Modify: `ui/views/json_types.go` (append budget DTOs)
- Test: `ui/views/budget_test.go`

**Interfaces:**
- Consumes: `model.Budget`, `model.BudgetReport` (Task 2), `utils.FormatAmount`, `CentsToUnit`.
- Produces:
  ```go
  func NewBudgetListView(w io.Writer) *BudgetListView
  func (v *BudgetListView) Render(items []model.Budget)
  func NewBudgetReportView(w io.Writer) *BudgetReportView
  func (v *BudgetReportView) Render(r *model.BudgetReport)
  func BudgetUsedLabel(budget, actual int64) string   // "73%", "over", ""
  type JSONBudget struct{...}; func ToJSONBudget(b model.Budget) JSONBudget; func ToJSONBudgets(bs []model.Budget) []JSONBudget
  type JSONBudgetReport struct{...}; func ToJSONBudgetReport(r *model.BudgetReport) JSONBudgetReport
  ```
  Views take an `io.Writer` (commands pass `os.Stdout`) so tests use a `bytes.Buffer` instead of redirecting stdout.

- [ ] **Step 1: Write the failing tests** (`ui/views/budget_test.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/pterm/pterm"
	"github.com/stretchr/testify/assert"
)

func TestBudgetUsedLabel(t *testing.T) {
	assert.Equal(t, "73%", BudgetUsedLabel(10000, 7320))
	assert.Equal(t, "107%", BudgetUsedLabel(6000, 6410))
	assert.Equal(t, "", BudgetUsedLabel(0, 0))
	assert.Equal(t, "over", BudgetUsedLabel(0, 1))
	assert.Equal(t, "0%", BudgetUsedLabel(0, -500)) // refund only, zero budget
	assert.Equal(t, "-5%", BudgetUsedLabel(10000, -500))
}

func sampleBudgetReport() *model.BudgetReport {
	return &model.BudgetReport{
		Month: "2026-10",
		Rows: []model.BudgetReportRow{
			{AccountName: "Expenses:Food", Currency: "TWD", Budget: 1000000, Actual: 732050, ActualIrregular: 732050, Remaining: 267950, ExcludedAccounts: []string{"Expenses:Food:Japan"}},
			{AccountName: "Expenses:Food:Dining", Currency: "TWD", Budget: 600000, Actual: 641000, ActualIrregular: 641000, Remaining: -41000, ExcludedAccounts: []string{}},
			{AccountName: "Expenses:Travel", Currency: "USD", Budget: 50000, Actual: 0, Remaining: 50000, ExcludedAccounts: []string{}},
		},
		TotalBudget: map[string]int64{"TWD": 1000000, "USD": 50000},
		TotalActual: map[string]int64{"TWD": 732050, "USD": 0},
	}
}

func TestBudgetReportView_Render(t *testing.T) {
	pterm.DisableColor()
	defer pterm.EnableColor()

	var buf bytes.Buffer
	NewBudgetReportView(&buf).Render(sampleBudgetReport())
	out := buf.String()

	assert.Contains(t, out, "2026-10")
	assert.Contains(t, out, "  Expenses:Food:Dining", "child row is indented under its budgeted parent")
	assert.Contains(t, out, "107%")
	assert.Contains(t, out, "-410")
	assert.Contains(t, out, "Total (TWD)")
	assert.Contains(t, out, "Total (USD)")
	assert.Contains(t, out, "Expenses:Food: 1 sub-account excluded (different currency): Expenses:Food:Japan")
}

func TestBudgetReportView_Empty(t *testing.T) {
	var buf bytes.Buffer
	NewBudgetReportView(&buf).Render(&model.BudgetReport{Month: "2026-10", Rows: []model.BudgetReportRow{}})
	assert.Contains(t, buf.String(), "No budgets in 2026-10")
}

func TestBudgetListView_Render(t *testing.T) {
	pterm.DisableColor()
	defer pterm.EnableColor()

	var buf bytes.Buffer
	NewBudgetListView(&buf).Render([]model.Budget{
		{ID: 7, AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 800000},
		{ID: 9, AccountName: "Expenses:Food", EffectiveMonth: "2026-06", Stopped: true},
	})
	out := buf.String()
	assert.Contains(t, out, "7")
	assert.Contains(t, out, "8,000")
	assert.Contains(t, out, "stopped")
	assert.Equal(t, 1, strings.Count(out, "2026-06"))
}

func TestToJSONBudgetReport(t *testing.T) {
	j := ToJSONBudgetReport(sampleBudgetReport())
	assert.Equal(t, "2026-10", j.Month)
	assert.InDelta(t, 7320.5, j.Rows[0].Actual, 0.001)
	assert.InDelta(t, -410.0, j.Rows[1].Remaining, 0.001)
	assert.InDelta(t, 10000.0, j.TotalBudget["TWD"], 0.001)
	assert.Equal(t, []string{"Expenses:Food:Japan"}, j.Rows[0].ExcludedAccounts)
}

func TestToJSONBudget(t *testing.T) {
	j := ToJSONBudget(model.Budget{ID: 1, AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 12345})
	assert.InDelta(t, 123.45, j.Amount, 0.001)
	assert.Equal(t, "Expenses:Food", j.AccountName)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ui/views/ -run 'Budget' -v`
Expected: FAIL to compile.

- [ ] **Step 3: Add the JSON DTOs** (append to `ui/views/json_types.go`)

```go
// JSONBudget is the CLI JSON shape of a budget version (amount in currency units).
type JSONBudget struct {
	ID             int64   `json:"id"`
	AccountName    string  `json:"account_name"`
	EffectiveMonth string  `json:"effective_month"`
	Amount         float64 `json:"amount"`
	Stopped        bool    `json:"stopped"`
}

func ToJSONBudget(b model.Budget) JSONBudget {
	return JSONBudget{
		ID:             b.ID,
		AccountName:    b.AccountName,
		EffectiveMonth: b.EffectiveMonth,
		Amount:         CentsToUnit(b.Amount),
		Stopped:        b.Stopped,
	}
}

func ToJSONBudgets(bs []model.Budget) []JSONBudget {
	out := make([]JSONBudget, len(bs))
	for i, b := range bs {
		out[i] = ToJSONBudget(b)
	}
	return out
}

// JSONBudgetReportRow mirrors model.BudgetReportRow in currency units.
type JSONBudgetReportRow struct {
	AccountName      string   `json:"account_name"`
	Currency         string   `json:"currency"`
	EffectiveMonth   string   `json:"effective_month"`
	Budget           float64  `json:"budget"`
	Actual           float64  `json:"actual"`
	ActualRegular    float64  `json:"actual_regular"`
	ActualIrregular  float64  `json:"actual_irregular"`
	Remaining        float64  `json:"remaining"`
	ExcludedAccounts []string `json:"excluded_accounts"`
}

// JSONBudgetReport mirrors model.BudgetReport in currency units.
type JSONBudgetReport struct {
	Month       string                `json:"month"`
	Rows        []JSONBudgetReportRow `json:"rows"`
	TotalBudget map[string]float64    `json:"total_budget"`
	TotalActual map[string]float64    `json:"total_actual"`
}

func ToJSONBudgetReport(r *model.BudgetReport) JSONBudgetReport {
	rows := make([]JSONBudgetReportRow, len(r.Rows))
	for i, row := range r.Rows {
		excluded := row.ExcludedAccounts
		if excluded == nil {
			excluded = []string{}
		}
		rows[i] = JSONBudgetReportRow{
			AccountName:      row.AccountName,
			Currency:         row.Currency,
			EffectiveMonth:   row.EffectiveMonth,
			Budget:           CentsToUnit(row.Budget),
			Actual:           CentsToUnit(row.Actual),
			ActualRegular:    CentsToUnit(row.ActualRegular),
			ActualIrregular:  CentsToUnit(row.ActualIrregular),
			Remaining:        CentsToUnit(row.Remaining),
			ExcludedAccounts: excluded,
		}
	}
	totalBudget := centsMapToUnitMap(r.TotalBudget)
	if totalBudget == nil {
		totalBudget = map[string]float64{}
	}
	totalActual := centsMapToUnitMap(r.TotalActual)
	if totalActual == nil {
		totalActual = map[string]float64{}
	}
	return JSONBudgetReport{Month: r.Month, Rows: rows, TotalBudget: totalBudget, TotalActual: totalActual}
}
```

(`centsMapToUnitMap` already exists in `ui/views/report_json.go`, same package.)

- [ ] **Step 4: Implement the views** (`ui/views/budget.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
	"fmt"
	"io"
	"strings"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

// Usage thresholds shared by the CLI color coding.
const (
	budgetWarnPct = 80
	budgetOverPct = 100
)

// BudgetUsedLabel formats actual/budget as an integer percent. A zero budget
// shows "over" when anything was spent and "" when nothing was.
func BudgetUsedLabel(budget, actual int64) string {
	if budget == 0 {
		switch {
		case actual > 0:
			return "over"
		case actual < 0:
			return "0%"
		default:
			return ""
		}
	}
	return fmt.Sprintf("%d%%", actual*100/budget)
}

// budgetColor picks the color for a row: red over budget, yellow from 80%.
func budgetColor(budget, actual int64) func(...any) string {
	switch {
	case budget == 0 && actual > 0, budget > 0 && actual*100 > budget*budgetOverPct:
		return pterm.Red
	case budget > 0 && actual*100 >= budget*budgetWarnPct:
		return pterm.Yellow
	default:
		return fmt.Sprint
	}
}

// BudgetReportView renders a budget vs actual table.
type BudgetReportView struct{ w io.Writer }

func NewBudgetReportView(w io.Writer) *BudgetReportView { return &BudgetReportView{w: w} }

func (v *BudgetReportView) Render(r *model.BudgetReport) {
	if len(r.Rows) == 0 {
		fmt.Fprintf(v.w, "No budgets in %s. Set one with: kea budget set <account> <amount>\n", r.Month)
		return
	}
	fmt.Fprintf(v.w, "Budget report  %s\n\n", r.Month)

	t := tablewriter.NewWriter(v.w)
	t.SetHeader([]string{"Account", "Budget", "Actual", "Regular", "Irregular", "Remaining", "Used", "Currency"})
	t.SetBorder(false)
	t.SetHeaderLine(true)
	t.SetAutoWrapText(false)
	t.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT,
	})

	for _, row := range r.Rows {
		color := budgetColor(row.Budget, row.Actual)
		t.Append([]string{
			strings.Repeat("  ", budgetDepth(row.AccountName, r.Rows)) + row.AccountName,
			utils.FormatAmount(row.Budget),
			utils.FormatAmount(row.Actual),
			utils.FormatAmount(row.ActualRegular),
			utils.FormatAmount(row.ActualIrregular),
			color(utils.FormatAmount(row.Remaining)),
			color(BudgetUsedLabel(row.Budget, row.Actual)),
			row.Currency,
		})
	}
	for _, ccy := range sortedKeys(r.TotalBudget) {
		budget, actual := r.TotalBudget[ccy], r.TotalActual[ccy]
		color := budgetColor(budget, actual)
		t.Append([]string{
			fmt.Sprintf("Total (%s)", ccy),
			utils.FormatAmount(budget),
			utils.FormatAmount(actual),
			"", "",
			color(utils.FormatAmount(budget - actual)),
			color(BudgetUsedLabel(budget, actual)),
			ccy,
		})
	}
	t.Render()

	for _, row := range r.Rows {
		if n := len(row.ExcludedAccounts); n > 0 {
			fmt.Fprintf(v.w, "! %s: %d sub-account excluded (different currency): %s\n",
				row.AccountName, n, strings.Join(row.ExcludedAccounts, ", "))
		}
	}
}

// budgetDepth counts the other budgeted rows that are ancestors of name.
func budgetDepth(name string, rows []model.BudgetReportRow) int {
	depth := 0
	for _, r := range rows {
		if r.AccountName != name && strings.HasPrefix(name, r.AccountName+":") {
			depth++
		}
	}
	return depth
}

// BudgetListView renders every budget version.
type BudgetListView struct{ w io.Writer }

func NewBudgetListView(w io.Writer) *BudgetListView { return &BudgetListView{w: w} }

func (v *BudgetListView) Render(items []model.Budget) {
	if len(items) == 0 {
		fmt.Fprintln(v.w, "No budgets. Set one with: kea budget set <account> <amount>")
		return
	}
	t := tablewriter.NewWriter(v.w)
	t.SetHeader([]string{"ID", "Account", "From", "Amount"})
	t.SetBorder(false)
	t.SetHeaderLine(true)
	t.SetAutoWrapText(false)
	t.SetColumnAlignment([]int{tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT})
	for _, b := range items {
		amount := utils.FormatAmount(b.Amount)
		if b.Stopped {
			amount = "stopped"
		}
		t.Append([]string{fmt.Sprintf("%d", b.ID), b.AccountName, b.EffectiveMonth, amount})
	}
	t.Render()
}
```

Note: `sortedKeys` (in `ui/views/report.go`) takes `map[string]int64`; `pterm.Red`/`pterm.Yellow` have signature `func(a ...any) string`, matching `fmt.Sprint`. If the installed pterm version's `Red` signature differs, wrap with `func(a ...any) string { return pterm.Red(a...) }`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./ui/views/ && go vet ./ui/views/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add ui/views/budget.go ui/views/budget_test.go ui/views/json_types.go
git commit -m "feat(views): add budget list and report views"
```

---

### Task 7: CLI commands (`kea budget`)

**Files:**
- Create: `cmd/budget/doc.go`, `cmd/budget/budget.go`, `cmd/budget/set.go`, `cmd/budget/stop.go`, `cmd/budget/list.go`, `cmd/budget/delete.go`, `cmd/budget/report.go`
- Test: `cmd/budget/budget_test.go`
- Modify: `cmd/root.go` (register after `NewReportCmd`)
- Modify: `SKILL.md` (new `### Budgets` under CLI Reference, and a line under Review finances)
- Modify: `docs/architecture.md` package map (add `cmd/budget` row)

**Interfaces:**
- Consumes: `svc.Budget()` methods, `svc.Account().GetAccountsByType`, `service.ValidateBudgetMonth`, views from Task 6, `utils.ParseAmount`, `prompts.PromptSelect`, `prompts.PromptAmount`, `prompts.PromptInput`, `prompts.PromptConfirm`, `views.WriteJSON`.
- Produces: `func NewBudgetCmd(svc *service.Service) *cobra.Command`.

Runner signatures (each command has its own narrow provider interface):

```go
type BudgetSetProvider interface {
	SetBudget(ctx context.Context, input model.SetBudgetInput) (*model.Budget, error)
}
type ExpenseAccountLister interface {
	GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error)
}
type BudgetStopProvider interface {
	StopBudget(ctx context.Context, input model.StopBudgetInput) (*model.Budget, error)
}
type BudgetListProvider interface {
	ListBudgets(ctx context.Context) ([]model.Budget, error)
}
type BudgetDeleteProvider interface {
	DeleteBudget(ctx context.Context, id int64) error
}
type BudgetReportProvider interface {
	GenerateBudgetReport(ctx context.Context, month string) (*model.BudgetReport, error)
}
```

- [ ] **Step 1: Write the failing tests** (`cmd/budget/budget_test.go`)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBudgetSvc struct {
	lastSet    model.SetBudgetInput
	lastStop   model.StopBudgetInput
	lastMonth  string
	deletedID  int64
	list       []model.Budget
	report     *model.BudgetReport
	err        error
}

func (m *mockBudgetSvc) SetBudget(_ context.Context, in model.SetBudgetInput) (*model.Budget, error) {
	m.lastSet = in
	if m.err != nil {
		return nil, m.err
	}
	return &model.Budget{ID: 1, AccountName: in.AccountName, EffectiveMonth: in.EffectiveMonth, Amount: in.Amount}, nil
}

func (m *mockBudgetSvc) StopBudget(_ context.Context, in model.StopBudgetInput) (*model.Budget, error) {
	m.lastStop = in
	if m.err != nil {
		return nil, m.err
	}
	return &model.Budget{ID: 2, AccountName: in.AccountName, EffectiveMonth: in.EffectiveMonth, Stopped: true}, nil
}

func (m *mockBudgetSvc) ListBudgets(_ context.Context) ([]model.Budget, error) { return m.list, m.err }

func (m *mockBudgetSvc) DeleteBudget(_ context.Context, id int64) error {
	m.deletedID = id
	return m.err
}

func (m *mockBudgetSvc) GenerateBudgetReport(_ context.Context, month string) (*model.BudgetReport, error) {
	m.lastMonth = month
	if m.err != nil {
		return nil, m.err
	}
	return m.report, nil
}

func TestSetRunner_FlagMode(t *testing.T) {
	m := &mockBudgetSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), []string{"Expenses:Food", "8000.50"}, &setFlags{Month: "2026-01"}))
	assert.Equal(t, model.SetBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 800050}, m.lastSet)
}

func TestSetRunner_DefaultsToCurrentMonth(t *testing.T) {
	m := &mockBudgetSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), []string{"Expenses:Food", "10"}, &setFlags{}))
	assert.Equal(t, time.Now().Format("2006-01"), m.lastSet.EffectiveMonth)
}

func TestSetRunner_BadAmount(t *testing.T) {
	r := &setRunner{svc: &mockBudgetSvc{}, out: &bytes.Buffer{}}
	err := r.Run(context.Background(), []string{"Expenses:Food", "abc"}, &setFlags{})
	assert.ErrorContains(t, err, "invalid amount")
}

func TestSetRunner_BadMonth(t *testing.T) {
	m := &mockBudgetSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	err := r.Run(context.Background(), []string{"Expenses:Food", "10"}, &setFlags{Month: "2026-13"})
	assert.Error(t, err)
	assert.Empty(t, m.lastSet.AccountName, "service must not be called with a bad month")
}

func TestSetRunner_PropagatesError(t *testing.T) {
	r := &setRunner{svc: &mockBudgetSvc{err: errors.New("boom")}, out: &bytes.Buffer{}}
	assert.ErrorContains(t, r.Run(context.Background(), []string{"Expenses:Food", "10"}, &setFlags{}), "boom")
}

func TestSetRunner_JSON(t *testing.T) {
	var out bytes.Buffer
	r := &setRunner{svc: &mockBudgetSvc{}, out: &out}
	require.NoError(t, r.Run(context.Background(), []string{"Expenses:Food", "12.5"}, &setFlags{Month: "2026-02", JSON: true}))
	assert.Contains(t, out.String(), `"amount": 12.5`)
	assert.Contains(t, out.String(), `"effective_month": "2026-02"`)
}

func TestStopRunner(t *testing.T) {
	m := &mockBudgetSvc{}
	r := &stopRunner{svc: m, month: "2026-04", out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), "Expenses:Food"))
	assert.Equal(t, model.StopBudgetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-04"}, m.lastStop)
}

func TestListRunner_FiltersByAccount(t *testing.T) {
	m := &mockBudgetSvc{list: []model.Budget{
		{ID: 1, AccountName: "Expenses:Food", EffectiveMonth: "2026-01", Amount: 100},
		{ID: 2, AccountName: "Expenses:Rent", EffectiveMonth: "2026-01", Amount: 200},
	}}
	var out bytes.Buffer
	r := &listRunner{svc: m, account: "Expenses:Rent", json: true, out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Contains(t, out.String(), "Expenses:Rent")
	assert.NotContains(t, out.String(), "Expenses:Food")
}

func TestDeleteRunner(t *testing.T) {
	m := &mockBudgetSvc{}
	r := &deleteRunner{svc: m, yes: true, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), "42"))
	assert.Equal(t, int64(42), m.deletedID)

	err := (&deleteRunner{svc: m, yes: true, out: &bytes.Buffer{}}).Run(context.Background(), "x")
	assert.ErrorContains(t, err, "invalid budget id")
}

func TestReportRunner(t *testing.T) {
	m := &mockBudgetSvc{report: &model.BudgetReport{Month: "2026-03", Rows: []model.BudgetReportRow{}}}
	var out bytes.Buffer
	r := &reportRunner{svc: m, month: "2026-03", out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Equal(t, "2026-03", m.lastMonth)
	assert.Contains(t, out.String(), "No budgets in 2026-03")
}

func TestReportRunner_JSON(t *testing.T) {
	m := &mockBudgetSvc{report: &model.BudgetReport{
		Month:       "2026-03",
		Rows:        []model.BudgetReportRow{{AccountName: "Expenses:Food", Currency: "USD", Budget: 1000, Actual: 250, Remaining: 750}},
		TotalBudget: map[string]int64{"USD": 1000},
		TotalActual: map[string]int64{"USD": 250},
	}}
	var out bytes.Buffer
	r := &reportRunner{svc: m, month: "2026-03", json: true, out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Contains(t, out.String(), `"actual": 2.5`)
}
```

Note: runners take an `out io.Writer` so JSON and table output is testable. Add a small helper in `cmd/budget/budget.go`:

```go
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
```

(Same format as `views.WriteJSON`, which only writes to stdout.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/budget/ -v`
Expected: FAIL (package does not exist).

- [ ] **Step 3: Implement the group and helpers** (`cmd/budget/doc.go`, `cmd/budget/budget.go`)

`cmd/budget/doc.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

// Package budget implements the `kea budget` subcommands: set, stop, list,
// delete and report.
package budget
```

`cmd/budget/budget.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"encoding/json"
	"io"
	"time"

	"github.com/hance08/kea/internal/service"
	"github.com/spf13/cobra"
)

func NewBudgetCmd(svc *service.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "budget",
		Aliases: []string{"bg"},
		Short:   "Manage monthly budgets on Expense accounts",
		Long: `Set, stop, list and delete monthly budgets on Expense accounts, and compare them with actual spending.
A budget applies from its month until a later version replaces or stops it.`,
	}
	cmd.AddCommand(NewSetCmd(svc))
	cmd.AddCommand(NewStopCmd(svc))
	cmd.AddCommand(NewListCmd(svc))
	cmd.AddCommand(NewDeleteCmd(svc))
	cmd.AddCommand(NewReportCmd(svc))
	return cmd
}

func currentMonth() string { return time.Now().Format("2006-01") }

func monthOrCurrent(m string) string {
	if m == "" {
		return currentMonth()
	}
	return m
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
```

- [ ] **Step 4: Implement `set`** (`cmd/budget/set.go`, Pattern C)

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/ui/prompts"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type BudgetSetProvider interface {
	SetBudget(ctx context.Context, input model.SetBudgetInput) (*model.Budget, error)
}

type ExpenseAccountLister interface {
	GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error)
}

type setFlags struct {
	Month string
	JSON  bool
}

type setRunner struct {
	svc      BudgetSetProvider
	accounts ExpenseAccountLister
	out      io.Writer
}

func NewSetCmd(svc *service.Service) *cobra.Command {
	flags := &setFlags{}
	cmd := &cobra.Command{
		Use:   "set [<account> <amount>]",
		Short: "Set a monthly budget on an Expense account",
		Long: `Set the monthly budget of an Expense account from --month (default: current month) on.
Setting the same account and month again replaces that version. Run without arguments for an interactive prompt.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 && len(args) != 2 {
				return fmt.Errorf("expected <account> <amount>, or no arguments for interactive mode")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &setRunner{svc: svc.Budget(), accounts: svc.Account(), out: os.Stdout}
			return r.Run(cmd.Context(), args, flags)
		},
	}
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "first month the budget applies to (YYYY-MM, default: current month)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *setRunner) Run(ctx context.Context, args []string, flags *setFlags) error {
	var input model.SetBudgetInput
	var err error
	if len(args) == 0 {
		input, err = r.promptInput(ctx, flags)
	} else {
		input, err = r.inputFromArgs(args, flags)
	}
	if err != nil {
		return err
	}

	b, err := r.svc.SetBudget(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to set budget: %w", err)
	}
	if flags.JSON {
		return writeJSON(r.out, views.ToJSONBudget(*b))
	}
	pterm.Success.Printf("Budget for %s set to %s from %s\n", b.AccountName, utils.FormatAmount(b.Amount), b.EffectiveMonth)
	return nil
}

func (r *setRunner) inputFromArgs(args []string, flags *setFlags) (model.SetBudgetInput, error) {
	amount, err := utils.ParseAmount(args[1])
	if err != nil {
		return model.SetBudgetInput{}, fmt.Errorf("invalid amount %q: %w", args[1], err)
	}
	month := monthOrCurrent(flags.Month)
	if err := service.ValidateBudgetMonth(month); err != nil {
		return model.SetBudgetInput{}, err
	}
	return model.SetBudgetInput{AccountName: args[0], EffectiveMonth: month, Amount: amount}, nil
}

func (r *setRunner) promptInput(ctx context.Context, flags *setFlags) (model.SetBudgetInput, error) {
	accs, err := r.accounts.GetAccountsByType(ctx, model.AccountTypeExpense)
	if err != nil {
		return model.SetBudgetInput{}, fmt.Errorf("failed to load Expense accounts: %w", err)
	}
	if len(accs) == 0 {
		return model.SetBudgetInput{}, fmt.Errorf("no Expense accounts; create one with `kea account create`")
	}
	names := make([]string, 0, len(accs))
	for _, a := range accs {
		if !a.IsHidden {
			names = append(names, a.Name)
		}
	}
	sort.Strings(names)

	account, err := prompts.PromptSelect("Expense account", names, names[0])
	if err != nil {
		return model.SetBudgetInput{}, err
	}
	amountStr, err := prompts.PromptAmount("Monthly budget", "e.g. 8000 or 8000.50", func(s string) error {
		_, err := utils.ParseAmount(s)
		return err
	})
	if err != nil {
		return model.SetBudgetInput{}, err
	}
	month, err := prompts.PromptInput("Effective from (YYYY-MM)", monthOrCurrent(flags.Month), service.ValidateBudgetMonth)
	if err != nil {
		return model.SetBudgetInput{}, err
	}
	amount, err := utils.ParseAmount(amountStr)
	if err != nil {
		return model.SetBudgetInput{}, fmt.Errorf("invalid amount %q: %w", amountStr, err)
	}
	return model.SetBudgetInput{AccountName: account, EffectiveMonth: month, Amount: amount}, nil
}
```

Check `prompts.PromptSelect`, `prompts.PromptAmount`, `prompts.PromptInput` signatures in `ui/prompts/common.go` before compiling (they are `PromptSelect(message string, options []string, defaultOption string) (string, error)`, `PromptAmount(message, helpText string, validator func(string) error) (string, error)`, `PromptInput(message, defaultValue string, validator func(string) error) (string, error)`).

The interactive path is not unit tested (huh needs a TTY); flag mode covers the assembly. Pattern C: `Run` receives the flags struct because mode is chosen from the arguments.

- [ ] **Step 5: Implement `stop`, `list`, `delete`, `report`** (Pattern B each)

`cmd/budget/stop.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type BudgetStopProvider interface {
	StopBudget(ctx context.Context, input model.StopBudgetInput) (*model.Budget, error)
}

type stopFlags struct {
	Month string
	JSON  bool
}

type stopRunner struct {
	svc   BudgetStopProvider
	month string
	json  bool
	out   io.Writer
}

func NewStopCmd(svc *service.Service) *cobra.Command {
	flags := &stopFlags{}
	cmd := &cobra.Command{
		Use:   "stop <account>",
		Short: "Stop an account's budget from a month on",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &stopRunner{svc: svc.Budget(), month: monthOrCurrent(flags.Month), json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context(), args[0])
		},
	}
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "first month without a budget (YYYY-MM, default: current month)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *stopRunner) Run(ctx context.Context, account string) error {
	b, err := r.svc.StopBudget(ctx, model.StopBudgetInput{AccountName: account, EffectiveMonth: r.month})
	if err != nil {
		return fmt.Errorf("failed to stop budget: %w", err)
	}
	if r.json {
		return writeJSON(r.out, views.ToJSONBudget(*b))
	}
	pterm.Success.Printf("Budget for %s stopped from %s\n", b.AccountName, b.EffectiveMonth)
	return nil
}
```

`cmd/budget/list.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type BudgetListProvider interface {
	ListBudgets(ctx context.Context) ([]model.Budget, error)
}

type listFlags struct {
	Account string
	JSON    bool
}

type listRunner struct {
	svc     BudgetListProvider
	account string
	json    bool
	out     io.Writer
}

func NewListCmd(svc *service.Service) *cobra.Command {
	flags := &listFlags{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List every budget version",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &listRunner{svc: svc.Budget(), account: flags.Account, json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&flags.Account, "account", "", "only show versions of this account")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *listRunner) Run(ctx context.Context) error {
	items, err := r.svc.ListBudgets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list budgets: %w", err)
	}
	if r.account != "" {
		filtered := make([]model.Budget, 0, len(items))
		for _, b := range items {
			if b.AccountName == r.account {
				filtered = append(filtered, b)
			}
		}
		items = filtered
	}
	if r.json {
		return writeJSON(r.out, views.ToJSONBudgets(items))
	}
	views.NewBudgetListView(r.out).Render(items)
	return nil
}
```

`cmd/budget/delete.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/prompts"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type BudgetDeleteProvider interface {
	DeleteBudget(ctx context.Context, id int64) error
}

type deleteFlags struct {
	Yes  bool
	JSON bool
}

type deleteRunner struct {
	svc  BudgetDeleteProvider
	yes  bool
	json bool
	out  io.Writer
}

func NewDeleteCmd(svc *service.Service) *cobra.Command {
	flags := &deleteFlags{}
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"del"},
		Short:   "Delete one budget version (see `kea budget list` for IDs)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &deleteRunner{svc: svc.Budget(), yes: flags.Yes || flags.JSON, json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context(), args[0])
		},
	}
	cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "confirm deletion without interactive prompt")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON (implies --yes)")
	return cmd
}

func (r *deleteRunner) Run(ctx context.Context, rawID string) error {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid budget id %q", rawID)
	}
	if !r.yes {
		ok, err := prompts.PromptConfirm(fmt.Sprintf("Delete budget version #%d?", id), false)
		if err != nil {
			return err
		}
		if !ok {
			pterm.Info.Println("Deletion cancelled")
			return nil
		}
	}
	if err := r.svc.DeleteBudget(ctx, id); err != nil {
		return fmt.Errorf("failed to delete budget: %w", err)
	}
	if r.json {
		return writeJSON(r.out, map[string]any{"id": id, "deleted": true})
	}
	pterm.Success.Printf("Budget version #%d deleted\n", id)
	return nil
}
```

`cmd/budget/report.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/ui/views"
	"github.com/spf13/cobra"
)

type BudgetReportProvider interface {
	GenerateBudgetReport(ctx context.Context, month string) (*model.BudgetReport, error)
}

type reportFlags struct {
	Month string
	JSON  bool
}

type reportRunner struct {
	svc   BudgetReportProvider
	month string
	json  bool
	out   io.Writer
}

func NewReportCmd(svc *service.Service) *cobra.Command {
	flags := &reportFlags{}
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show budget vs actual spending for a month",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &reportRunner{svc: svc.Budget(), month: flags.Month, json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "month to report on (YYYY-MM, default: current month)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output report as JSON")
	return cmd
}

func (r *reportRunner) Run(ctx context.Context) error {
	report, err := r.svc.GenerateBudgetReport(ctx, r.month)
	if err != nil {
		return fmt.Errorf("failed to generate budget report: %w", err)
	}
	if r.json {
		return writeJSON(r.out, views.ToJSONBudgetReport(report))
	}
	views.NewBudgetReportView(r.out).Render(report)
	return nil
}
```

(The report passes `r.month` through unchanged; an empty month lets the service pick the current month.)

- [ ] **Step 6: Register the command** (`cmd/root.go`)

Add the import `budgetcmd "github.com/hance08/kea/cmd/budget"` and, after `rootCmd.AddCommand(NewReportCmd(application.Service))`:

```go
		rootCmd.AddCommand(budgetcmd.NewBudgetCmd(application.Service))
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./cmd/... && go build ./... && go vet ./...`
Expected: PASS.

Smoke test against a throwaway ledger (do not touch the user's real ledger):

```bash
make build
KEA_HOME=$(mktemp -d) ./kea --help | grep budget
```

If kea has no `KEA_HOME`-style override, skip the smoke test and rely on the unit tests; never run write commands against the user's default ledger.

- [ ] **Step 8: Update docs**

- `SKILL.md`: add `### Budgets` under `## CLI Reference` (after `### Reports`) documenting `kea budget set <account> <amount> [--month YYYY-MM] [--json]`, `stop`, `list [--account]`, `delete <id> [--yes]`, `report [--month] [--json]`, with one example each, and the rules: Expense accounts only, versions apply from their month on, parent budgets include same-currency descendants, only Expense-typed transactions count, refunds reduce actual. Add one bullet under `### Review finances`: `kea budget report --json`.
- `docs/architecture.md` package map: add a row `| \`cmd/budget\` | \`kea budget\` subcommands (set, stop, list, delete, report) | \`internal/app\`, \`internal/store\`, \`internal/api\` |`, and in "The service facade" list add `BudgetService` (`internal/service/budget_service.go`, `internal/service/budget_report.go`) — budget versions and budget vs actual; and update the `NewService` sentence to mention the `BudgetRepository` argument.

Run: `scripts/check-docs.sh`
Expected: `check-docs: all referenced paths exist`.

- [ ] **Step 9: Commit**

```bash
git add cmd/budget cmd/root.go SKILL.md docs/architecture.md
git commit -m "feat(cli): add kea budget commands"
```

---

### Task 8: SPA data layer

**Files:**
- Modify: `spa/src/lib/types.ts` (budget types)
- Create: `spa/src/lib/api/budgets.ts`
- Create: `spa/src/lib/hooks/useBudgets.ts`
- Create: `spa/src/lib/budgets.ts` (pure helpers)
- Modify: `spa/src/lib/reports-search-params.ts` (month-only search)
- Modify: `spa/src/lib/filter-memory.ts` (`PageId` gains `'reports/budget'`)
- Test: `spa/src/lib/budgets.test.ts`, `spa/src/test/api.budgets.test.ts`

**Interfaces:**
- Produces:
  ```ts
  // types.ts
  export interface Budget { id: number; account_id: number; account_name: string; effective_month: string; amount: number; stopped: boolean }
  export interface BudgetListResponse { items: Budget[] }
  export interface SetBudgetInput { account_name: string; effective_month: string; amount: number }
  export interface StopBudgetInput { account_name: string; effective_month: string }
  export interface BudgetReportRow { account_id: number; account_name: string; currency: string; effective_month: string; budget: number; actual: number; actual_regular: number; actual_irregular: number; remaining: number; excluded_accounts: string[] }
  export interface BudgetReport { month: string; rows: BudgetReportRow[]; total_budget: Record<string, number>; total_actual: Record<string, number> }
  // api/budgets.ts
  fetchBudgets(): Promise<BudgetListResponse>
  setBudget(input: SetBudgetInput): Promise<Budget>
  stopBudget(input: StopBudgetInput): Promise<Budget>
  deleteBudget(id: number): Promise<{ deleted: boolean; id: number }>
  fetchBudgetReport(month?: string): Promise<BudgetReport>
  // hooks/useBudgets.ts
  useBudgets(); useBudgetReport(month?: string)   // query keys ['budgets','list'] and ['budgets','report', month ?? 'current']
  // budgets.ts
  export type BudgetStatus = 'ok' | 'warning' | 'over';
  usedPct(budget: number, actual: number): number | null
  budgetStatus(budget: number, actual: number): BudgetStatus
  currentMonth(now?: Date): string             // local time
  shiftMonth(month: string, delta: number): string
  elapsedFraction(month: string, now?: Date): number | null  // null unless month is the current local month
  activeBudgets(items: Budget[], month: string): Budget[]
  depthIn(name: string, names: string[]): number
  parseCents(s: string): number                // NaN when invalid
  // reports-search-params.ts
  export type MonthSearchParams = { month?: string }
  parseMonthSearch(input: unknown): MonthSearchParams
  ```

- [ ] **Step 1: Write the failing tests**

`spa/src/lib/budgets.test.ts`:

```ts
import { describe, expect, test } from 'vitest';
import type { Budget } from './types';
import {
  activeBudgets,
  budgetStatus,
  currentMonth,
  depthIn,
  elapsedFraction,
  parseCents,
  shiftMonth,
  usedPct,
} from './budgets';

describe('usedPct / budgetStatus', () => {
  test('normal ratios', () => {
    expect(usedPct(10000, 7300)).toBeCloseTo(73);
    expect(budgetStatus(10000, 7300)).toBe('ok');
    expect(budgetStatus(10000, 8000)).toBe('warning');
    expect(budgetStatus(10000, 10000)).toBe('warning');
    expect(budgetStatus(10000, 10001)).toBe('over');
  });
  test('zero budget', () => {
    expect(usedPct(0, 0)).toBeNull();
    expect(budgetStatus(0, 0)).toBe('ok');
    expect(budgetStatus(0, 1)).toBe('over');
    expect(budgetStatus(0, -100)).toBe('ok');
  });
});

describe('months (local time)', () => {
  test('currentMonth uses local getters', () => {
    // Local midnight on Nov 1 must be November even where UTC is still October.
    expect(currentMonth(new Date(2026, 10, 1, 0, 30))).toBe('2026-11');
  });
  test('shiftMonth crosses years', () => {
    expect(shiftMonth('2026-01', -1)).toBe('2025-12');
    expect(shiftMonth('2026-12', 1)).toBe('2027-01');
  });
  test('elapsedFraction only for the current month', () => {
    const now = new Date(2026, 9, 16, 0, 0); // Oct 16 local, 31-day month
    expect(elapsedFraction('2026-10', now)).toBeCloseTo(15 / 31);
    expect(elapsedFraction('2026-09', now)).toBeNull();
  });
});

describe('activeBudgets', () => {
  const b = (id: number, account: string, month: string, amount: number, stopped = false): Budget => ({
    id,
    account_id: account === 'Expenses:Food' ? 1 : 2,
    account_name: account,
    effective_month: month,
    amount,
    stopped,
  });
  const items = [
    b(1, 'Expenses:Food', '2026-01', 100),
    b(2, 'Expenses:Food', '2026-04', 200),
    b(3, 'Expenses:Food', '2026-07', 0, true),
    b(4, 'Expenses:Rent', '2026-09', 500),
  ];
  test('picks latest version <= month and drops stopped', () => {
    expect(activeBudgets(items, '2026-05').map((x) => x.id)).toEqual([2]);
    expect(activeBudgets(items, '2026-08')).toEqual([]);
    expect(activeBudgets(items, '2026-09').map((x) => x.id)).toEqual([4]);
  });
});

test('depthIn counts budgeted ancestors, not name prefixes', () => {
  const names = ['Expenses:Food', 'Expenses:Food:Dining', 'Expenses:FoodTruck'];
  expect(depthIn('Expenses:Food:Dining', names)).toBe(1);
  expect(depthIn('Expenses:FoodTruck', names)).toBe(0);
});

test('parseCents', () => {
  expect(parseCents('8000.5')).toBe(800050);
  expect(parseCents('abc')).toBeNaN();
  expect(parseCents('')).toBeNaN();
});
```

`spa/src/test/api.budgets.test.ts`:

```ts
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { deleteBudget, fetchBudgetReport, setBudget, stopBudget } from '../lib/api/budgets';

let fetchSpy: ReturnType<typeof vi.fn>;
const ok = (body: unknown) =>
  new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } });

beforeEach(() => {
  fetchSpy = vi.fn(() => Promise.resolve(ok({})));
  vi.stubGlobal('fetch', fetchSpy);
});
afterEach(() => vi.unstubAllGlobals());

test('setBudget PUTs JSON', async () => {
  await setBudget({ account_name: 'Expenses:Food', effective_month: '2026-01', amount: 100 });
  const [url, init] = fetchSpy.mock.calls[0];
  expect(url).toBe('/api/budgets');
  expect(init.method).toBe('PUT');
  expect(JSON.parse(init.body)).toEqual({ account_name: 'Expenses:Food', effective_month: '2026-01', amount: 100 });
});

test('stopBudget POSTs to /stop', async () => {
  await stopBudget({ account_name: 'Expenses:Food', effective_month: '2026-02' });
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/budgets/stop');
  expect(fetchSpy.mock.calls[0][1].method).toBe('POST');
});

test('deleteBudget DELETEs by id', async () => {
  await deleteBudget(7);
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/budgets/7');
  expect(fetchSpy.mock.calls[0][1].method).toBe('DELETE');
});

test('fetchBudgetReport omits month when undefined', async () => {
  await fetchBudgetReport();
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/reports/budget');
  await fetchBudgetReport('2026-03');
  expect(fetchSpy.mock.calls[1][0]).toBe('/api/reports/budget?month=2026-03');
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd spa && npx vitest run src/lib/budgets.test.ts src/test/api.budgets.test.ts`
Expected: FAIL (modules not found).

- [ ] **Step 3: Implement**

Append to `spa/src/lib/types.ts`:

```ts
export interface Budget {
  id: number;
  account_id: number;
  account_name: string;
  effective_month: string; // YYYY-MM
  amount: number; // int64 cents
  stopped: boolean;
}

export interface BudgetListResponse {
  items: Budget[];
}

export interface SetBudgetInput {
  account_name: string;
  effective_month: string;
  amount: number;
}

export interface StopBudgetInput {
  account_name: string;
  effective_month: string;
}

export interface BudgetReportRow {
  account_id: number;
  account_name: string;
  currency: string;
  effective_month: string;
  budget: number;
  actual: number;
  actual_regular: number;
  actual_irregular: number;
  remaining: number;
  excluded_accounts: string[];
}

export interface BudgetReport {
  month: string;
  rows: BudgetReportRow[];
  total_budget: Record<string, number>;
  total_actual: Record<string, number>;
}
```

`spa/src/lib/api/budgets.ts`:

```ts
import { apiFetch } from '../api';
import type { Budget, BudgetListResponse, BudgetReport, SetBudgetInput, StopBudgetInput } from '../types';

const jsonInit = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
});

export function fetchBudgets(): Promise<BudgetListResponse> {
  return apiFetch<BudgetListResponse>('/api/budgets');
}

export function setBudget(input: SetBudgetInput): Promise<Budget> {
  return apiFetch<Budget>('/api/budgets', jsonInit('PUT', input));
}

export function stopBudget(input: StopBudgetInput): Promise<Budget> {
  return apiFetch<Budget>('/api/budgets/stop', jsonInit('POST', input));
}

export function deleteBudget(id: number): Promise<{ deleted: boolean; id: number }> {
  return apiFetch<{ deleted: boolean; id: number }>(`/api/budgets/${id}`, { method: 'DELETE' });
}

// Omitting month lets the server use its current local month.
export function fetchBudgetReport(month?: string): Promise<BudgetReport> {
  const q = month ? `?month=${encodeURIComponent(month)}` : '';
  return apiFetch<BudgetReport>(`/api/reports/budget${q}`);
}
```

`spa/src/lib/hooks/useBudgets.ts`:

```ts
import { useQuery } from '@tanstack/react-query';
import { fetchBudgetReport, fetchBudgets } from '../api/budgets';

export function useBudgets() {
  return useQuery({ queryKey: ['budgets', 'list'], queryFn: fetchBudgets });
}

export function useBudgetReport(month?: string) {
  return useQuery({
    queryKey: ['budgets', 'report', month ?? 'current'],
    queryFn: () => fetchBudgetReport(month),
  });
}
```

`spa/src/lib/budgets.ts`:

```ts
import type { Budget } from './types';

export type BudgetStatus = 'ok' | 'warning' | 'over';

export const WARN_PCT = 80;

/** Percent of budget used, or null when the budget is 0 (no meaningful ratio). */
export function usedPct(budget: number, actual: number): number | null {
  if (budget === 0) return null;
  return (actual / budget) * 100;
}

export function budgetStatus(budget: number, actual: number): BudgetStatus {
  if (budget === 0) return actual > 0 ? 'over' : 'ok';
  const pct = (actual / budget) * 100;
  if (pct > 100) return 'over';
  if (pct >= WARN_PCT) return 'warning';
  return 'ok';
}

const pad = (n: number) => String(n).padStart(2, '0');

/** Current month in LOCAL time; the server interprets months in local time too. */
export function currentMonth(now: Date = new Date()): string {
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}`;
}

export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number);
  const d = new Date(y, m - 1 + delta, 1);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
}

/** Fraction of the month elapsed (completed days / days in month), only for the current month. */
export function elapsedFraction(month: string, now: Date = new Date()): number | null {
  if (month !== currentMonth(now)) return null;
  const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
  return (now.getDate() - 1) / daysInMonth;
}

/** Mirrors the server: latest version with effective_month <= month per account, minus stopped ones. */
export function activeBudgets(items: Budget[], month: string): Budget[] {
  const latest = new Map<number, Budget>();
  for (const b of items) {
    if (b.effective_month > month) continue;
    const cur = latest.get(b.account_id);
    if (!cur || b.effective_month > cur.effective_month) latest.set(b.account_id, b);
  }
  return [...latest.values()]
    .filter((b) => !b.stopped)
    .sort((a, b) => a.account_name.localeCompare(b.account_name));
}

/** Number of other names in `names` that are ancestors of `name` in the account tree. */
export function depthIn(name: string, names: string[]): number {
  return names.filter((n) => n !== name && name.startsWith(`${n}:`)).length;
}

export function parseCents(s: string): number {
  if (s.trim() === '') return Number.NaN;
  const n = Number(s);
  if (!Number.isFinite(n)) return Number.NaN;
  return Math.round(n * 100);
}
```

In `spa/src/lib/reports-search-params.ts` add:

```ts
export const monthSearchSchema = z.object({
  month: z.string().regex(monthPattern).optional(),
});

export type MonthSearchParams = { month?: string };

/** Month-only search for budget pages; absent month means the current month. */
export function parseMonthSearch(input: unknown): MonthSearchParams {
  const result = monthSearchSchema.safeParse(input);
  if (result.success && result.data.month !== undefined) return { month: result.data.month };
  return {};
}
```

In `spa/src/lib/filter-memory.ts` extend `PageId` with `| 'reports/budget'`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npx vitest run src/lib/budgets.test.ts src/test/api.budgets.test.ts && npx tsc -b && npm run check`
Expected: PASS, no type or Biome errors (run `npm run check:write` for formatting-only fixes).

- [ ] **Step 5: Commit**

```bash
git add spa/src/lib/types.ts spa/src/lib/api/budgets.ts spa/src/lib/hooks/useBudgets.ts spa/src/lib/budgets.ts spa/src/lib/budgets.test.ts spa/src/test/api.budgets.test.ts spa/src/lib/reports-search-params.ts spa/src/lib/filter-memory.ts
git commit -m "feat(spa): add budget API client and helpers"
```

---

### Task 9: SPA budget report page

**Files:**
- Create: `spa/src/components/budgets/MonthPicker.tsx`
- Create: `spa/src/components/budgets/BudgetProgressBar.tsx`
- Create: `spa/src/routes/reports.budget.tsx`
- Modify: `spa/src/components/reports/TabNav.tsx` (add Budget tab)
- Modify: `spa/src/routeTree.gen.ts` (regenerated, committed)
- Test: `spa/src/test/reports.budget.test.tsx`; update `spa/src/test/reports.tab-nav.test.tsx` if it asserts the tab list

**Interfaces:**
- Consumes: Task 8 (`useBudgetReport`, `budgetStatus`, `usedPct`, `elapsedFraction`, `currentMonth`, `shiftMonth`, `depthIn`, `parseMonthSearch`, `MonthSearchParams`), `makeFilterMemoryLoader`, `useAmountFormat`, `regularSubLine`, `Alert`, `Button`, `Skeleton`.
- Produces:
  ```tsx
  export function MonthPicker(props: { value?: string; onChange: (month: string | undefined) => void }): JSX.Element
  export function BudgetProgressBar(props: { budget: number; actual: number; elapsed?: number | null; compact?: boolean }): JSX.Element
  // data-testid="budget-bar" with data-status="ok|warning|over"; marker data-testid="time-marker"
  ```

- [ ] **Step 1: Write the failing test** (`spa/src/test/reports.budget.test.tsx`)

```tsx
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { currentMonth } from '../lib/budgets';
import type { BudgetReport } from '../lib/types';
import { makeTestApp } from './test-app';

const ok = (body: unknown) =>
  new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } });

function row(name: string, budget: number, actual: number, extra: Partial<BudgetReport['rows'][0]> = {}) {
  return {
    account_id: 1,
    account_name: name,
    currency: 'USD',
    effective_month: '2026-01',
    budget,
    actual,
    actual_regular: 0,
    actual_irregular: actual,
    remaining: budget - actual,
    excluded_accounts: [],
    ...extra,
  };
}

let report: BudgetReport;
let reportUrls: string[];

beforeEach(() => {
  // The page remembers its month per ledger; start every test from a clean slate.
  localStorage.clear();
  reportUrls = [];
  report = {
    month: currentMonth(),
    rows: [
      row('Expenses:Food', 100000, 50000, { excluded_accounts: ['Expenses:Food:Japan'] }),
      row('Expenses:Food:Dining', 60000, 64100),
      row('Expenses:Rent', 200000, 170000),
      row('Expenses:Gifts', 0, 1000),
    ],
    total_budget: { USD: 300000, TWD: 500000 },
    total_actual: { USD: 221000, TWD: 0 },
  };
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config')
        return Promise.resolve(ok({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }));
      if (url === '/api/ledgers')
        return Promise.resolve(ok({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }));
      if (url.startsWith('/api/reports/budget')) {
        reportUrls.push(url);
        return Promise.resolve(ok(report));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('renders rows with status colors, indentation and per-currency totals', async () => {
  render(makeTestApp('/reports/budget'));
  await waitFor(() => expect(screen.getAllByTestId('budget-bar').length).toBe(4));

  const bars = screen.getAllByTestId('budget-bar');
  expect(bars.map((b) => b.getAttribute('data-status'))).toEqual(['ok', 'over', 'warning', 'over']);

  const dining = screen.getByText('Food:Dining').closest('[data-testid="budget-row"]') as HTMLElement;
  expect(dining.getAttribute('data-depth')).toBe('1');

  expect(screen.getByText('Total (USD)')).toBeInTheDocument();
  expect(screen.getByText('Total (TWD)')).toBeInTheDocument();
});

test('shows the time marker for the current month', async () => {
  render(makeTestApp('/reports/budget'));
  await waitFor(() => expect(screen.getAllByTestId('budget-bar').length).toBe(4));
  expect(screen.getAllByTestId('time-marker').length).toBe(4);
  expect(reportUrls[0]).toBe('/api/reports/budget');
});

test('hides the time marker for a past month', async () => {
  report = { ...report, month: '2020-01' };
  render(makeTestApp('/reports/budget?month=2020-01'));
  await waitFor(() => expect(screen.getAllByTestId('budget-bar').length).toBe(4));
  expect(reportUrls).toContain('/api/reports/budget?month=2020-01');
  expect(screen.queryAllByTestId('time-marker').length).toBe(0);
});

test('excluded accounts produce a warning with their names', async () => {
  render(makeTestApp('/reports/budget'));
  const warn = await screen.findByTestId('excluded-warning');
  expect(warn.getAttribute('title')).toContain('Expenses:Food:Japan');
});

test('zero budget with spending reads as over', async () => {
  render(makeTestApp('/reports/budget'));
  const gifts = (await screen.findByText('Gifts')).closest('[data-testid="budget-row"]') as HTMLElement;
  expect(within(gifts).getByText('over')).toBeInTheDocument();
});

test('empty month links to the budgets page', async () => {
  report = { month: currentMonth(), rows: [], total_budget: {}, total_actual: {} };
  render(makeTestApp('/reports/budget'));
  const link = await screen.findByRole('link', { name: /set up budgets/i });
  expect(link.getAttribute('href')).toBe('/budgets');
});

test('previous-month button navigates with month param', async () => {
  render(makeTestApp('/reports/budget'));
  await waitFor(() => expect(screen.getAllByTestId('budget-bar').length).toBe(4));
  await userEvent.click(screen.getByRole('button', { name: /previous month/i }));
  await waitFor(() => expect(reportUrls.some((u) => u.includes('month='))).toBe(true));
});
```

(Report pages strip the `Expenses:` root in rendered names, like `reports.expense-breakdown`: render `account_name.split(':').slice(1).join(':')`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npx vitest run src/test/reports.budget.test.tsx`
Expected: FAIL (route not found).

- [ ] **Step 3: Implement the components**

`spa/src/components/budgets/MonthPicker.tsx`:

```tsx
import { Button } from '@/components/ui/button';
import { currentMonth, shiftMonth } from '@/lib/budgets';

interface Props {
  value?: string; // undefined = current month
  onChange: (month: string | undefined) => void;
}

export function MonthPicker({ value, onChange }: Props) {
  const month = value ?? currentMonth();
  const isCurrent = month === currentMonth();
  const go = (m: string) => onChange(m === currentMonth() ? undefined : m);
  return (
    <div className="flex items-center gap-2">
      <Button variant="outline" size="sm" aria-label="Previous month" onClick={() => go(shiftMonth(month, -1))}>
        ‹
      </Button>
      <span className="min-w-[5.5rem] text-center text-sm font-medium tabular-nums">{month}</span>
      <Button variant="outline" size="sm" aria-label="Next month" onClick={() => go(shiftMonth(month, 1))}>
        ›
      </Button>
      {!isCurrent && (
        <Button variant="ghost" size="sm" onClick={() => onChange(undefined)}>
          This month
        </Button>
      )}
    </div>
  );
}
```

`spa/src/components/budgets/BudgetProgressBar.tsx`:

```tsx
import { budgetStatus } from '@/lib/budgets';
import { cn } from '@/lib/cn';

interface Props {
  budget: number;
  actual: number;
  elapsed?: number | null; // 0..1, shown as a marker for the current month
  compact?: boolean;
}

const FILL = { ok: 'bg-primary', warning: 'bg-amber-500', over: 'bg-red-600' } as const;

export function BudgetProgressBar({ budget, actual, elapsed, compact }: Props) {
  const status = budgetStatus(budget, actual);
  const ratio = budget === 0 ? (actual > 0 ? 1 : 0) : Math.max(0, actual / budget);
  return (
    <div
      data-testid="budget-bar"
      data-status={status}
      className={cn('relative w-full rounded bg-muted', compact ? 'h-1.5' : 'h-2.5')}
    >
      <div className={cn('h-full rounded', FILL[status])} style={{ width: `${Math.min(ratio, 1) * 100}%` }} />
      {elapsed != null && (
        <div
          data-testid="time-marker"
          title="Time elapsed this month"
          className="absolute -top-0.5 h-[calc(100%+4px)] w-px bg-foreground/70"
          style={{ left: `${elapsed * 100}%` }}
        />
      )}
    </div>
  );
}
```

- [ ] **Step 4: Implement the route** (`spa/src/routes/reports.budget.tsx`)

```tsx
import { BudgetProgressBar } from '@/components/budgets/BudgetProgressBar';
import { MonthPicker } from '@/components/budgets/MonthPicker';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { depthIn, elapsedFraction, usedPct } from '@/lib/budgets';
import { makeFilterMemoryLoader } from '@/lib/filter-memory';
import { useBudgetReport } from '@/lib/hooks/useBudgets';
import { regularSubLine } from '@/lib/reportSubLine';
import { type MonthSearchParams, parseMonthSearch } from '@/lib/reports-search-params';
import { useAmountFormat } from '@/lib/server-config';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/reports/budget')({
  validateSearch: (s): MonthSearchParams => parseMonthSearch(s),
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<MonthSearchParams>({
    pageId: 'reports/budget',
    defaults: {},
    redirectTo: '/reports/budget',
  }),
  component: BudgetReportPage,
});

const shortName = (name: string) => name.split(':').slice(1).join(':') || name;

function usedLabel(budget: number, actual: number): string {
  const pct = usedPct(budget, actual);
  if (pct === null) return actual > 0 ? 'over' : '';
  return `${Math.round(pct)}%`;
}

function BudgetReportPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/reports/budget' });
  const query = useBudgetReport(search.month);
  const { formatCents } = useAmountFormat();
  const setMonth = (month: string | undefined) => navigate({ search: () => (month ? { month } : {}) });

  const picker = <MonthPicker value={search.month} onChange={setMonth} />;

  if (query.isPending) {
    return (
      <div className="space-y-4">
        {picker}
        <Skeleton className="h-48" />
      </div>
    );
  }
  if (query.isError) {
    return (
      <div className="space-y-3">
        {picker}
        <Alert variant="destructive">
          <AlertTitle>Failed to load budget report</AlertTitle>
          <AlertDescription className="mt-2 space-y-3">
            <div>{query.error instanceof Error ? query.error.message : 'Unknown error'}</div>
            <Button onClick={() => query.refetch()} size="sm">
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  const report = query.data;
  const elapsed = elapsedFraction(report.month);
  const names = report.rows.map((r) => r.account_name);

  return (
    <div className="space-y-4" data-testid="budget-report">
      {picker}
      {report.rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No budgets in {report.month}.{' '}
          <Link to="/budgets" className="underline">
            Set up budgets
          </Link>
        </p>
      ) : (
        <div className="space-y-3">
          {report.rows.map((r) => {
            const depth = depthIn(r.account_name, names);
            return (
              <div
                key={r.account_id}
                data-testid="budget-row"
                data-depth={depth}
                className="space-y-1"
                style={{ paddingLeft: `${depth * 1.25}rem` }}
              >
                <div className="flex items-baseline justify-between gap-3 text-sm">
                  <span className="flex items-center gap-1 truncate font-medium">
                    {shortName(r.account_name)}
                    {r.excluded_accounts.length > 0 && (
                      <span
                        data-testid="excluded-warning"
                        className="text-amber-600"
                        title={`Not counted (different currency): ${r.excluded_accounts.join(', ')}`}
                      >
                        ⚠
                      </span>
                    )}
                  </span>
                  <span className="tabular-nums text-muted-foreground">
                    {formatCents(r.actual, r.currency)} / {formatCents(r.budget, r.currency)}
                    <span className="ml-2 font-medium text-foreground">{usedLabel(r.budget, r.actual)}</span>
                  </span>
                </div>
                <BudgetProgressBar budget={r.budget} actual={r.actual} elapsed={elapsed} />
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>{regularSubLine(r.actual_regular, r.actual_irregular, r.currency, formatCents)}</span>
                  <span className={r.remaining < 0 ? 'text-red-600' : undefined}>
                    Remaining {formatCents(r.remaining, r.currency)}
                  </span>
                </div>
              </div>
            );
          })}
          <div className="border-t pt-3 text-sm">
            {Object.keys(report.total_budget)
              .sort()
              .map((ccy) => {
                const budget = report.total_budget[ccy];
                const actual = report.total_actual[ccy] ?? 0;
                return (
                  <div key={ccy} className="flex justify-between">
                    <span className="font-medium">Total ({ccy})</span>
                    <span className="tabular-nums">
                      {formatCents(actual, ccy)} / {formatCents(budget, ccy)} · Remaining{' '}
                      {formatCents(budget - actual, ccy)}
                    </span>
                  </div>
                );
              })}
          </div>
        </div>
      )}
    </div>
  );
}
```

Add the tab in `spa/src/components/reports/TabNav.tsx` (after Expense Breakdown):

```ts
  { to: '/reports/budget', label: 'Budget' },
```

- [ ] **Step 5: Regenerate the route tree**

Run: `cd spa && npm run build`
Expected: build succeeds; `src/routeTree.gen.ts` now includes `/reports/budget`. (The TanStack Router Vite plugin regenerates it.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd spa && npm test && npm run check`
Expected: PASS. If `src/test/reports.tab-nav.test.tsx` asserts the exact tab list, add `'Budget'` there.

- [ ] **Step 7: Commit**

```bash
git add spa/src/components/budgets spa/src/routes/reports.budget.tsx spa/src/components/reports/TabNav.tsx spa/src/routeTree.gen.ts spa/src/test/reports.budget.test.tsx spa/src/test/reports.tab-nav.test.tsx
git commit -m "feat(spa): add budget vs actual report page"
```

---

### Task 10: SPA budgets settings page

**Files:**
- Create: `spa/src/components/budgets/BudgetForm.tsx`
- Create: `spa/src/routes/budgets.tsx`
- Modify: `spa/src/components/Sidebar.tsx` (nav item "Budgets" after "Reports")
- Modify: `spa/src/routeTree.gen.ts` (regenerated)
- Test: `spa/src/test/budgets.page.test.tsx`; update `spa/src/test/sidebar.reconcile.test.tsx` only if it asserts the full nav list

**Interfaces:**
- Consumes: Task 8 API + helpers, Task 9 `MonthPicker`, `AccountCombobox` (`allowedTypes={['E']}`), `ApiError` (`.field`), `toast` from `sonner`, `useMutation`/`useQueryClient`.
- Produces:
  ```tsx
  export function BudgetForm(props: {
    initial?: { account_name: string; amount: number; effective_month: string };
    lockAccount?: boolean;
    defaultMonth: string;
    onDone: () => void;
  }): JSX.Element
  ```

- [ ] **Step 1: Write the failing test** (`spa/src/test/budgets.page.test.tsx`)

```tsx
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import type { Budget } from '../lib/types';
import { makeTestApp } from './test-app';

const ok = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

let items: Budget[];
let calls: { url: string; method: string; body?: unknown }[];
let putResponse: Response | null;

beforeEach(() => {
  calls = [];
  putResponse = null;
  items = [
    { id: 1, account_id: 1, account_name: 'Expenses:Food', effective_month: '2020-01', amount: 800000, stopped: false },
    { id: 2, account_id: 1, account_name: 'Expenses:Food', effective_month: '2020-06', amount: 900000, stopped: false },
    { id: 3, account_id: 2, account_name: 'Expenses:Gym', effective_month: '2020-01', amount: 5000, stopped: false },
    { id: 4, account_id: 2, account_name: 'Expenses:Gym', effective_month: '2020-03', amount: 0, stopped: true },
  ];
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      const method = init?.method ?? 'GET';
      calls.push({ url, method, body: init?.body ? JSON.parse(String(init.body)) : undefined });
      if (url === '/api/config')
        return Promise.resolve(ok({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }));
      if (url === '/api/ledgers')
        return Promise.resolve(ok({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }));
      if (url === '/api/budgets' && method === 'GET') return Promise.resolve(ok({ items }));
      if (url === '/api/budgets' && method === 'PUT')
        return Promise.resolve(putResponse ?? ok({ ...items[1], id: 9 }));
      if (url === '/api/budgets/stop') return Promise.resolve(ok({ ...items[1], stopped: true }));
      if (url.startsWith('/api/budgets/') && method === 'DELETE') return Promise.resolve(ok({ deleted: true, id: 1 }));
      throw new Error(`unexpected fetch: ${method} ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('lists budgets effective in the selected month', async () => {
  render(makeTestApp('/budgets?month=2020-07'));
  const rows = await screen.findAllByTestId('budget-setting-row');
  expect(rows).toHaveLength(1); // Gym stopped in 2020-03
  expect(within(rows[0]).getByText('Expenses:Food')).toBeInTheDocument();
  expect(within(rows[0]).getByText('2020-06')).toBeInTheDocument();
});

test('edit creates a new version from the selected month', async () => {
  render(makeTestApp('/budgets?month=2020-07'));
  const row = (await screen.findAllByTestId('budget-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /edit/i }));
  const amount = screen.getByLabelText(/amount/i);
  await userEvent.clear(amount);
  await userEvent.type(amount, '9500');
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true));
  const put = calls.find((c) => c.method === 'PUT');
  expect(put?.body).toEqual({ account_name: 'Expenses:Food', effective_month: '2020-07', amount: 950000 });
});

test('field errors from the API are shown next to the field', async () => {
  putResponse = ok({ error: 'validation_failed', message: 'budget amount must not be negative', field: 'amount' }, 400);
  render(makeTestApp('/budgets?month=2020-07'));
  const row = (await screen.findAllByTestId('budget-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /edit/i }));
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  expect(await screen.findByText('budget amount must not be negative')).toBeInTheDocument();
});

test('stop posts the selected month', async () => {
  render(makeTestApp('/budgets?month=2020-07'));
  const row = (await screen.findAllByTestId('budget-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /stop/i }));
  await userEvent.click(screen.getByRole('button', { name: /confirm stop/i }));
  await waitFor(() => expect(calls.some((c) => c.url === '/api/budgets/stop')).toBe(true));
  expect(calls.find((c) => c.url === '/api/budgets/stop')?.body).toEqual({
    account_name: 'Expenses:Food',
    effective_month: '2020-07',
  });
});

test('history shows all versions and deletes one after confirmation', async () => {
  render(makeTestApp('/budgets?month=2020-07'));
  const row = (await screen.findAllByTestId('budget-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /history/i }));
  const versions = screen.getAllByTestId('budget-version');
  expect(versions).toHaveLength(2);
  await userEvent.click(within(versions[0]).getByRole('button', { name: /delete/i }));
  await userEvent.click(screen.getByRole('button', { name: /confirm delete/i }));
  await waitFor(() => expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/budgets/1')).toBe(true));
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npx vitest run src/test/budgets.page.test.tsx`
Expected: FAIL (route not found).

- [ ] **Step 3: Implement `BudgetForm`** (`spa/src/components/budgets/BudgetForm.tsx`)

```tsx
import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ApiError } from '@/lib/api';
import { setBudget } from '@/lib/api/budgets';
import { parseCents } from '@/lib/budgets';
import type { SetBudgetInput } from '@/lib/types';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useId, useState } from 'react';
import { toast } from 'sonner';

interface Props {
  initial?: { account_name: string; amount: number; effective_month: string };
  lockAccount?: boolean;
  defaultMonth: string;
  onDone: () => void;
}

type FieldErrors = Partial<Record<'account_name' | 'effective_month' | 'amount', string>>;

export function BudgetForm({ initial, lockAccount, defaultMonth, onDone }: Props) {
  const id = useId();
  const queryClient = useQueryClient();
  const [account, setAccount] = useState(initial?.account_name ?? '');
  const [amount, setAmount] = useState(initial ? String(initial.amount / 100) : '');
  const [month, setMonth] = useState(defaultMonth);
  const [errors, setErrors] = useState<FieldErrors>({});

  const mutation = useMutation({
    mutationFn: (input: SetBudgetInput) => setBudget(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['budgets'] });
      toast.success('Budget saved');
      onDone();
    },
    onError: (err) => {
      if (err instanceof ApiError && err.field && err.field in { account_name: 1, effective_month: 1, amount: 1 }) {
        setErrors({ [err.field]: err.message });
      } else {
        toast.error(err instanceof Error ? err.message : 'Failed to save budget');
      }
    },
  });

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const cents = parseCents(amount);
    const next: FieldErrors = {};
    if (!account) next.account_name = 'Choose an Expense account';
    if (Number.isNaN(cents)) next.amount = 'Enter a number';
    setErrors(next);
    if (Object.keys(next).length > 0) return;
    mutation.mutate({ account_name: account, effective_month: month, amount: cents });
  }

  return (
    <form onSubmit={submit} className="grid gap-3 rounded border p-3 sm:grid-cols-4 sm:items-end">
      <div className="sm:col-span-2">
        <Label htmlFor={`${id}-account`}>Account</Label>
        {lockAccount ? (
          <p id={`${id}-account`} className="py-2 text-sm font-medium">
            {account}
          </p>
        ) : (
          <AccountCombobox
            id={`${id}-account`}
            value={account}
            onChange={(name) => setAccount(name)}
            allowedTypes={['E']}
            placeholder="Expense account…"
            aria-invalid={!!errors.account_name}
          />
        )}
        {errors.account_name && <p className="mt-1 text-xs text-destructive">{errors.account_name}</p>}
      </div>
      <div>
        <Label htmlFor={`${id}-amount`}>Amount</Label>
        <Input
          id={`${id}-amount`}
          inputMode="decimal"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          aria-invalid={!!errors.amount}
        />
        {errors.amount && <p className="mt-1 text-xs text-destructive">{errors.amount}</p>}
      </div>
      <div>
        <Label htmlFor={`${id}-month`}>Effective from</Label>
        <Input
          id={`${id}-month`}
          type="month"
          value={month}
          onChange={(e) => setMonth(e.target.value)}
          aria-invalid={!!errors.effective_month}
        />
        {errors.effective_month && <p className="mt-1 text-xs text-destructive">{errors.effective_month}</p>}
      </div>
      <div className="flex gap-2 sm:col-span-4">
        <Button type="submit" size="sm" disabled={mutation.isPending}>
          Save
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={onDone}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
```

- [ ] **Step 4: Implement the page** (`spa/src/routes/budgets.tsx`)

```tsx
import { BudgetForm } from '@/components/budgets/BudgetForm';
import { MonthPicker } from '@/components/budgets/MonthPicker';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { deleteBudget, stopBudget } from '@/lib/api/budgets';
import { activeBudgets, currentMonth, depthIn } from '@/lib/budgets';
import { useBudgets } from '@/lib/hooks/useBudgets';
import { type MonthSearchParams, parseMonthSearch } from '@/lib/reports-search-params';
import { useAmountFormat } from '@/lib/server-config';
import type { Budget } from '@/lib/types';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { toast } from 'sonner';

export const Route = createFileRoute('/budgets')({
  validateSearch: (s): MonthSearchParams => parseMonthSearch(s),
  component: BudgetsPage,
});

type Panel = { kind: 'add' } | { kind: 'edit'; budget: Budget } | null;

function BudgetsPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/budgets' });
  const month = search.month ?? currentMonth();
  const query = useBudgets();
  const queryClient = useQueryClient();
  const { formatAmount } = useAmountFormat();
  const [panel, setPanel] = useState<Panel>(null);
  const [historyFor, setHistoryFor] = useState<number | null>(null);
  const [confirm, setConfirm] = useState<{ kind: 'stop'; budget: Budget } | { kind: 'delete'; budget: Budget } | null>(
    null,
  );

  const onError = (err: unknown) => toast.error(err instanceof Error ? err.message : 'Request failed');
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['budgets'] });
  const stopMutation = useMutation({
    mutationFn: (b: Budget) => stopBudget({ account_name: b.account_name, effective_month: month }),
    onSuccess: invalidate,
    onError,
  });
  const deleteMutation = useMutation({
    mutationFn: (b: Budget) => deleteBudget(b.id),
    onSuccess: invalidate,
    onError,
  });

  const setMonth = (m: string | undefined) => navigate({ search: () => (m ? { month: m } : {}) });

  if (query.isPending) return <Skeleton className="h-48" />;
  if (query.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load budgets</AlertTitle>
        <AlertDescription>{query.error instanceof Error ? query.error.message : 'Unknown error'}</AlertDescription>
      </Alert>
    );
  }

  const all = query.data.items;
  const active = activeBudgets(all, month);
  const names = active.map((b) => b.account_name);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Budgets</h1>
        <Button size="sm" onClick={() => setPanel({ kind: 'add' })}>
          Add budget
        </Button>
      </div>
      <MonthPicker value={search.month} onChange={setMonth} />

      {panel?.kind === 'add' && <BudgetForm defaultMonth={month} onDone={() => setPanel(null)} />}

      {confirm && (
        <div className="flex items-center gap-3 rounded border border-amber-500 p-3 text-sm">
          <span>
            {confirm.kind === 'stop'
              ? `Stop the budget for ${confirm.budget.account_name} from ${month}?`
              : `Delete the ${confirm.budget.effective_month} version of ${confirm.budget.account_name}?`}
          </span>
          <Button
            size="sm"
            variant="destructive"
            onClick={() => {
              if (confirm.kind === 'stop') stopMutation.mutate(confirm.budget);
              else deleteMutation.mutate(confirm.budget);
              setConfirm(null);
            }}
          >
            {confirm.kind === 'stop' ? 'Confirm stop' : 'Confirm delete'}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setConfirm(null)}>
            Cancel
          </Button>
        </div>
      )}

      {active.length === 0 ? (
        <p className="text-sm text-muted-foreground">No budgets in {month}.</p>
      ) : (
        <ul className="divide-y rounded border">
          {active.map((b) => (
            <li key={b.account_id} data-testid="budget-setting-row" className="space-y-2 p-3">
              <div
                className="flex flex-wrap items-center justify-between gap-2"
                style={{ paddingLeft: `${depthIn(b.account_name, names) * 1.25}rem` }}
              >
                <div className="text-sm">
                  <div className="font-medium">{b.account_name}</div>
                  <div className="text-xs text-muted-foreground">
                    from <span>{b.effective_month}</span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <span className="tabular-nums text-sm">{formatAmount(b.amount)}</span>
                  <Button size="sm" variant="outline" onClick={() => setPanel({ kind: 'edit', budget: b })}>
                    Edit
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => setConfirm({ kind: 'stop', budget: b })}>
                    Stop
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setHistoryFor(historyFor === b.account_id ? null : b.account_id)}
                  >
                    History
                  </Button>
                </div>
              </div>
              {panel?.kind === 'edit' && panel.budget.account_id === b.account_id && (
                <BudgetForm
                  initial={b}
                  lockAccount
                  defaultMonth={month}
                  onDone={() => setPanel(null)}
                />
              )}
              {historyFor === b.account_id && (
                <ul className="ml-4 space-y-1 text-xs">
                  {all
                    .filter((v) => v.account_id === b.account_id)
                    .map((v) => (
                      <li key={v.id} data-testid="budget-version" className="flex items-center gap-3">
                        <span className="tabular-nums">{v.effective_month}</span>
                        <span className="tabular-nums">{v.stopped ? 'stopped' : formatAmount(v.amount)}</span>
                        <Button size="sm" variant="ghost" onClick={() => setConfirm({ kind: 'delete', budget: v })}>
                          Delete
                        </Button>
                      </li>
                    ))}
                </ul>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
```

Notes for the implementer:
- The history list only shows accounts that are active in the selected month. A fully stopped account is reached by moving the month back; that is acceptable for v1.
- If `Button` has no `destructive` variant in `spa/src/components/ui/button.tsx`, use the closest existing variant.
- In the edit test, the history `<span>` for `2020-06` and the row's "from 2020-06" both render that text; the test scopes queries with `within(row)` before opening history, so this is fine.

Add to `spa/src/components/Sidebar.tsx` `NAV` after Reports:

```ts
  { label: 'Budgets', to: '/budgets' },
```

- [ ] **Step 5: Regenerate the route tree, run tests**

Run: `cd spa && npm run build && npm test && npm run check`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/budgets/BudgetForm.tsx spa/src/routes/budgets.tsx spa/src/components/Sidebar.tsx spa/src/routeTree.gen.ts spa/src/test/budgets.page.test.tsx
git commit -m "feat(spa): add budgets settings page"
```

(Add `spa/src/test/sidebar.reconcile.test.tsx` if you had to update it.)

---

### Task 11: SPA dashboard card

**Files:**
- Create: `spa/src/components/dashboard/widgets/BudgetProgress.tsx`
- Modify: `spa/src/lib/dashboard/types.ts` (`WidgetId` gains `'budget-progress'`)
- Modify: `spa/src/lib/dashboard/registry.tsx`
- Modify: `spa/src/lib/dashboard/defaults.ts`
- Test: `spa/src/test/dashboard.budget-progress.test.tsx`; update `spa/src/lib/dashboard/storage.test.ts` / `spa/src/components/dashboard/Dashboard.test.tsx` if they assert the widget count

**Interfaces:**
- Consumes: `useBudgetReport()` (no month: server's current month), `BudgetProgressBar`, `elapsedFraction`, `usedPct`, `withServerConfig`.
- Produces:
  ```tsx
  export type BudgetProgressConfig = { limit: 5 | 10; onlyWarnings: boolean };
  export const BUDGET_PROGRESS_DEFAULT: BudgetProgressConfig;
  export function BudgetProgress(props: { config: BudgetProgressConfig }): JSX.Element
  export function BudgetProgressConfigForm(props: { config: BudgetProgressConfig; onChange: (c: BudgetProgressConfig) => void }): JSX.Element
  ```

- [ ] **Step 1: Write the failing test** (`spa/src/test/dashboard.budget-progress.test.tsx`)

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createMemoryHistory, createRootRoute, createRouter } from '@tanstack/react-router';
import { render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import {
  BUDGET_PROGRESS_DEFAULT,
  BudgetProgress,
} from '../components/dashboard/widgets/BudgetProgress';
import { ALL_WIDGET_IDS } from '../lib/dashboard/registry';
import { currentMonth } from '../lib/budgets';
import { withServerConfig } from './test-app';

const ok = (body: unknown) =>
  new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } });

const row = (name: string, budget: number, actual: number) => ({
  account_id: name.length,
  account_name: name,
  currency: 'USD',
  effective_month: '2026-01',
  budget,
  actual,
  actual_regular: 0,
  actual_irregular: actual,
  remaining: budget - actual,
  excluded_accounts: [],
});

let urls: string[];
let rows: ReturnType<typeof row>[];

beforeEach(() => {
  urls = [];
  rows = [
    row('Expenses:A', 1000, 100), // 10%
    row('Expenses:B', 1000, 900), // 90%
    row('Expenses:C', 1000, 1200), // 120%
    row('Expenses:D', 1000, 500), // 50%
    row('Expenses:E', 1000, 0),
    row('Expenses:F', 1000, 850), // 85%
  ];
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      urls.push(url);
      return Promise.resolve(ok({ month: currentMonth(), rows, total_budget: {}, total_actual: {} }));
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

// The widget renders <Link>, so it needs a router context.
function renderWidget(node: ReactNode) {
  const rootRoute = createRootRoute({ component: () => <>{node}</> });
  const router = createRouter({ routeTree: rootRoute, history: createMemoryHistory({ initialEntries: ['/'] }) });
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return render(
    <QueryClientProvider client={qc}>{withServerConfig(<RouterProvider router={router} />)}</QueryClientProvider>,
  );
}

test('lets the server choose the month (no UTC month on the client)', async () => {
  renderWidget(<BudgetProgress config={BUDGET_PROGRESS_DEFAULT} />);
  await waitFor(() => expect(urls).toContain('/api/reports/budget'));
});

test('sorts by percent used and limits to N', async () => {
  renderWidget(<BudgetProgress config={{ limit: 5, onlyWarnings: false }} />);
  const items = await screen.findAllByTestId('budget-progress-item');
  expect(items).toHaveLength(5);
  expect(items.map((i) => i.getAttribute('data-account'))).toEqual([
    'Expenses:C',
    'Expenses:B',
    'Expenses:F',
    'Expenses:D',
    'Expenses:A',
  ]);
});

test('onlyWarnings keeps rows at or above 80%', async () => {
  renderWidget(<BudgetProgress config={{ limit: 10, onlyWarnings: true }} />);
  const items = await screen.findAllByTestId('budget-progress-item');
  expect(items.map((i) => i.getAttribute('data-account'))).toEqual(['Expenses:C', 'Expenses:B', 'Expenses:F']);
});

test('empty state links to /budgets', async () => {
  rows = [];
  renderWidget(<BudgetProgress config={BUDGET_PROGRESS_DEFAULT} />);
  const link = await screen.findByRole('link', { name: /set up budgets/i });
  expect(link.getAttribute('href')).toBe('/budgets');
});

test('widget is registered', () => {
  expect(ALL_WIDGET_IDS).toContain('budget-progress');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npx vitest run src/test/dashboard.budget-progress.test.tsx`
Expected: FAIL (module not found).

- [ ] **Step 3: Implement the widget** (`spa/src/components/dashboard/widgets/BudgetProgress.tsx`)

```tsx
import { Link } from '@tanstack/react-router';
import { BudgetProgressBar } from '../../budgets/BudgetProgressBar';
import { WARN_PCT, elapsedFraction } from '../../../lib/budgets';
import { useBudgetReport } from '../../../lib/hooks/useBudgets';
import type { BudgetReportRow } from '../../../lib/types';

export type BudgetProgressConfig = { limit: 5 | 10; onlyWarnings: boolean };
export const BUDGET_PROGRESS_DEFAULT: BudgetProgressConfig = { limit: 5, onlyWarnings: false };

// Sort key: zero budgets with spending sort first (infinitely over).
function ratio(r: BudgetReportRow): number {
  if (r.budget === 0) return r.actual > 0 ? Number.POSITIVE_INFINITY : 0;
  return r.actual / r.budget;
}

function pctLabel(r: BudgetReportRow): string {
  if (r.budget === 0) return r.actual > 0 ? 'over' : '';
  return `${Math.round((r.actual / r.budget) * 100)}%`;
}

export function BudgetProgress({ config }: { config: BudgetProgressConfig }) {
  // No month: the server resolves the current month in its local time.
  const q = useBudgetReport();
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const report = q.data;
  if (!report || report.rows.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        <span>
          No budgets yet.{' '}
          <Link to="/budgets" className="underline">
            Set up budgets
          </Link>
        </span>
      </div>
    );
  }

  const elapsed = elapsedFraction(report.month);
  const items = [...report.rows]
    .filter((r) => !config.onlyWarnings || ratio(r) * 100 >= WARN_PCT)
    .sort((a, b) => ratio(b) - ratio(a))
    .slice(0, config.limit);

  return (
    <Link to="/reports/budget" className="flex h-full flex-col">
      <ul className="flex flex-col gap-2 overflow-auto">
        {items.map((r) => (
          <li key={r.account_id} data-testid="budget-progress-item" data-account={r.account_name} className="text-xs">
            <div className="flex justify-between">
              <span className="truncate">{r.account_name}</span>
              <span className="tabular-nums">{pctLabel(r)}</span>
            </div>
            <div className="mt-0.5">
              <BudgetProgressBar budget={r.budget} actual={r.actual} elapsed={elapsed} compact />
            </div>
          </li>
        ))}
        {items.length === 0 && <li className="text-xs text-muted-foreground">All budgets under {WARN_PCT}%</li>}
      </ul>
    </Link>
  );
}

export function BudgetProgressConfigForm({
  config,
  onChange,
}: { config: BudgetProgressConfig; onChange: (c: BudgetProgressConfig) => void }) {
  return (
    <div className="flex flex-col gap-2 text-sm">
      <label className="flex flex-col gap-1">
        <span className="text-xs font-medium">Number of budgets</span>
        <select
          value={config.limit}
          onChange={(e) => onChange({ ...config, limit: Number(e.target.value) as BudgetProgressConfig['limit'] })}
          className="rounded border px-2 py-1 text-sm"
        >
          <option value={5}>5</option>
          <option value={10}>10</option>
        </select>
      </label>
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={config.onlyWarnings}
          onChange={(e) => onChange({ ...config, onlyWarnings: e.target.checked })}
        />
        <span className="text-xs">Only show budgets at {WARN_PCT}% or more</span>
      </label>
    </div>
  );
}
```

Note: when the empty-state renders, the outer element is not a `Link`, so the test's `findByRole('link', { name: /set up budgets/i })` finds exactly one link. In the non-empty state the whole card is one link to the report; do not nest the "Set up budgets" link inside it.

- [ ] **Step 4: Register and add to the default layout**

`spa/src/lib/dashboard/types.ts`: add `| 'budget-progress'` to `WidgetId`.

`spa/src/lib/dashboard/registry.tsx`: import and add

```tsx
import {
  BUDGET_PROGRESS_DEFAULT,
  BudgetProgress,
  BudgetProgressConfigForm,
} from '../../components/dashboard/widgets/BudgetProgress';
```

```tsx
  'budget-progress': {
    id: 'budget-progress',
    title: 'Budgets This Month',
    defaultConfig: BUDGET_PROGRESS_DEFAULT,
    component: BudgetProgress,
    ConfigForm: BudgetProgressConfigForm,
  } as WidgetMeta,
```

`spa/src/lib/dashboard/defaults.ts`: append `{ i: 'budget-progress', x: 0, y: 5, w: 1, h: 2 }` to `DEFAULT_LAYOUT` and add `//   row 5: [budget-progress  ]` to the layout comment.

Existing saved layouts pick the widget up automatically: `reconcileWithRegistry` (`spa/src/lib/dashboard/storage.ts`) appends known widgets that are missing from a saved layout as visible.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd spa && npm test && npx tsc -b && npm run check`
Expected: PASS. Update widget-count assertions in `storage.test.ts` / `Dashboard.test.tsx` if any fail only because of the new widget.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/dashboard/widgets/BudgetProgress.tsx spa/src/lib/dashboard spa/src/test/dashboard.budget-progress.test.tsx spa/src/components/dashboard/Dashboard.test.tsx
git commit -m "feat(spa): add budget progress dashboard card"
```

---

### Task 12: Domain docs, decisions, final verification

**Files:**
- Modify: `docs/domain.md` (new `## Budgets` section before `## Errors you will meet`)
- Modify: `docs/decisions.md` (two entries under `## Domain`)
- Modify: `spa/README.md` (routes list)
- Modify: `docs/README.md` only if it indexes domain sections

- [ ] **Step 1: Write `docs/domain.md` `## Budgets`**

Cover, in the file's existing style (short paragraphs, bullet rules, a "Code lives in:" line):
- A budget is a monthly limit on an Expense (`E`) account, leaf or parent, stored in `budgets` (`migrations/0012_create_budgets.up.sql`).
- Versioning: a row applies from `effective_month` until a later row for the same account; `stopped=1` ends it; same (account, month) is replaced (upsert). `amount=0` is a real budget.
- Currency is the account's currency (empty = `config.Defaults.Currency`); descendant splits in other currencies are excluded and listed in `ExcludedAccounts`.
- Actual = signed sum of Expense splits of `Expense`-typed transactions on the account and descendants (`isSelfOrDescendant`: exact name or `name + ":"` prefix); refunds reduce it; split into `ActualRegular`/`ActualIrregular`. This differs from the expense report, which sums absolute values.
- Totals per currency over top-level budgeted rows only.
- No rollover.
- Code lives in: `internal/service/budget_service.go`, `internal/service/budget_report.go`, `internal/model/budget.go`, `internal/store/sqlite_budget.go`.

- [ ] **Step 2: Add decisions** (`docs/decisions.md`, under `## Domain`, same four-line format)

```markdown
### Budgets are versioned by effective month
- **Decision:** A budget row applies from its `effective_month` until a later row for the same account replaces or stops it; editing adds a version instead of rewriting the amount.
- **Why:** Past months must keep showing the budget that applied then; one table covers "amount changes from July" without per-month rows.
- **Where:** `migrations/0012_create_budgets.up.sql`, `internal/service/budget_service.go` (`activeBudgets`)
- **Source:** [2026-10-05-monthly-budget-design.md](history/superpowers/specs/2026-10-05-monthly-budget-design.md)

### Budget actuals use signed sums
- **Decision:** Budget actual spending sums Expense split amounts with their sign, so refunds reduce it; the transaction scope (Expense-typed only) matches the expense report.
- **Why:** A refund must give budget back; the existing reports' absolute-value sums would count it as more spending.
- **Where:** `internal/service/budget_report.go` (`GenerateBudgetReport`)
- **Source:** [2026-10-05-monthly-budget-design.md](history/superpowers/specs/2026-10-05-monthly-budget-design.md)
```

- [ ] **Step 3: Update `spa/README.md`**

Add `/budgets` (budget settings) and `/reports/budget` (budget vs actual) to the routes list, and `budget-progress` to the dashboard widget list if the README has one.

- [ ] **Step 4: Full verification**

Run each and confirm the expected output:

```bash
go build ./... && go vet ./...
go test ./...
scripts/check-docs.sh
cd spa && npm test && npm run check && npm run build
```

Expected: all Go tests PASS (except the documented Docker-root-only `TestSwap_FailedSwapKeepsOldConnection` if run inside the `app` container), `check-docs: all referenced paths exist`, SPA tests PASS, Biome clean, build succeeds.

- [ ] **Step 5: Commit**

```bash
git add docs/domain.md docs/decisions.md spa/README.md
git commit -m "docs: document monthly budgets"
```

(Add `docs/README.md` and `spa/src/routeTree.gen.ts` if they changed.)
