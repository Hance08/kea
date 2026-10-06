# Savings Target Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Per-ledger monthly savings targets on Asset accounts, with target vs saved (month and year to date) in the CLI, HTTP API, a SPA report tab, a SPA settings page and a dashboard card.

**Architecture:** A new `savings_targets` table (migration 0013) stores versioned rows `(account, effective_month, amount, stopped)`, exactly like `budgets`. A new `SavingsService` (facade `svc.Savings()`) owns validation, version selection and the report. "Saved" is the signed sum of all splits on the target account and its descendants, excluding `Opening`-typed transactions, bucketed by local month. Version selection and the "same-currency ancestor" total rule move from `BudgetService` into generic helpers in `internal/service/versions.go`, shared by both services. API, CLI (`cmd/savings`) and SPA sit on top and mirror the budget feature.

**Tech Stack:** Go 1.25 (generics), SQLite (mattn/go-sqlite3, golang-migrate), chi, cobra, huh, pterm, tablewriter, testify; React, TanStack Router/Query, Vitest, Testing Library, Biome.

**Spec:** [docs/history/superpowers/specs/2026-10-06-savings-target-design.md](../specs/2026-10-06-savings-target-design.md)

## Global Constraints

- All code, comments, test names, commit messages and docs in English.
- Amounts are `int64` cents; convert only with `utils.FormatAmount` / `utils.ParseAmount` (Go). HTTP API amounts are integer cents; CLI `--json` amounts are decimal units via `views.CentsToUnit`.
- Never sum amounts across currencies; every total is a `map[string]int64` keyed by currency.
- Dependency direction: `cmd`/`ui`/`internal/api` -> `internal/service` -> `internal/repository` <- `internal/store`. `internal/model` imports no kea package. Service never imports store.
- Every repository/store method takes `context.Context` first and uses `*Context` methods of `database/sql`.
- Reads that guard writes go inside `TransactionManager.ExecTx`, using the `repo` argument (no nesting).
- Wrap errors with `%w`; services return service errors (`ErrNotFound`, `*ValidationError`), never raw repository errors. A new service sentinel needs a `mapError` case in `internal/api/errors.go` (this plan adds none).
- Every Go source file starts with `// SPDX-License-Identifier: GPL-3.0-or-later` / `// Copyright (C) 2026  Hance Chin`.
- CLI flag patterns (`docs/recipes/add-cli-command.md`): Pattern B = 1-2 flags (copy fields into the runner), Pattern C = interactive-vs-flag mode (pass flags to `Run`). Follow `cmd/budget/` exactly.
- Month strings are `YYYY-MM`, interpreted in **local time** (`parseMonth` in `internal/service/report_service.go`). The SPA never derives "current month" from UTC; the dashboard card omits `month` so the server decides.
- `Opening`-typed transactions never count as saved. Every other transaction type counts.
- `Remaining = Target - Saved`, signed (negative = exceeded). No rollover.
- Work on branch `feat/savings-target` (already created; the spec is committed there). Commit after each task with explicit paths (never `git add -A`). End every commit message with:
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`
- Never commit `internal/web/dist/index.html`; if a build changes it, run `git restore internal/web/dist/index.html`.
- SPA baseline: 6 tests in `spa/src/test/balances.test.tsx` and `spa/src/test/balances.link.test.tsx` already fail on master (verified 2026-10-06). Treat exactly those 6 as the baseline; any other failure is yours.

## Review Focus

1. **`Opening` balance on the savings account** — an account created with an opening balance must report `Saved = 0` for that month, not the opening amount. Pinned in Task 5 (service) and Task 6 (API, via `seedAccount` with a balance).
2. **Local-time month boundary** — a transfer at 23:30 local time on Jan 31 belongs to January; at 00:15 on Feb 1 it belongs to February. Pinned in Task 5. The dashboard card must call `/api/reports/savings` with no `month`. Pinned in Task 12.
3. **Name-prefix collision** — a target on `Assets:Savings` must not count `Assets:SavingsBox`. Pinned in Task 5.
4. **Stop then restart within the year** — YTD must skip the stopped months (target and saved), and re-setting a stopped month must clear `stopped`. Pinned in Task 2 (store) and Task 5 (report).
5. **Negative saved (net withdrawal)** — `Saved < 0` must give `Remaining > Target`, show yellow in the CLI, and the SPA bar status `negative` with "… to go" wording. A zero target with a withdrawal is not "met". Pinned in Task 7 and Task 9/10.

---

## File Structure

| File | Responsibility |
|---|---|
| `migrations/0013_create_savings_targets.up.sql` / `.down.sql` | Schema |
| `internal/model/savings.go` | `SavingsTarget`, inputs, `SavingsReport`, `SavingsReportRow`, `SavingsMonth` |
| `internal/repository/interfaces.go` | `SavingsTargetRepository`; added to `Repository` |
| `internal/store/sqlite_savings_target.go` | SQLite CRUD |
| `internal/service/versions.go` | Generic version selection + same-currency-ancestor helper (shared by budgets and savings) |
| `internal/service/budget_service.go`, `budget_report.go` | Switch to the generic helpers (behavior unchanged) |
| `internal/service/savings_service.go` | `SavingsService` CRUD + validation |
| `internal/service/savings_report.go` | `GenerateSavingsReport` |
| `internal/service/service.go` | facade `Savings()`; `NewService` gains `savingsRepo` |
| `internal/app/app.go`, `internal/api/testhelper_test.go` | Pass the store as the new repository |
| `internal/api/savings.go`, `internal/api/router.go` | Handlers and routes |
| `ui/views/savings.go`, `ui/views/json_types.go` | CLI tables and JSON DTOs |
| `cmd/savings/*.go`, `cmd/root.go` | `kea savings set|stop|list|delete|report` |
| `spa/src/lib/types.ts`, `spa/src/lib/api/savings.ts`, `spa/src/lib/hooks/useSavings.ts`, `spa/src/lib/savings.ts`, `spa/src/lib/budgets.ts` | SPA data + pure helpers |
| `spa/src/components/savings/SavingsProgressBar.tsx`, `SavingsForm.tsx` | Components |
| `spa/src/routes/reports.savings.tsx`, `spa/src/routes/savings.tsx` | Pages |
| `spa/src/components/dashboard/widgets/SavingsProgress.tsx` | Dashboard card |
| `docs/*.md`, `SKILL.md`, `spa/README.md` | Docs |

---

### Task 1: Migration 0013

**Files:**
- Create: `migrations/0013_create_savings_targets.up.sql`
- Create: `migrations/0013_create_savings_targets.down.sql`
- Test: `internal/store/migration_0013_test.go`

**Interfaces:**
- Produces: table `savings_targets(id, account_id, effective_month, amount, stopped)` with `UNIQUE(account_id, effective_month)`, index `idx_savings_targets_account_month`.

- [ ] **Step 1: Write the failing test**

`internal/store/migration_0013_test.go`:

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

func TestMigration0013_SavingsTargetsConstraints(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	db := s.DB()

	savingsID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	insert := func(accountID int64, month string, amount int64, stopped int) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO savings_targets (account_id, effective_month, amount, stopped) VALUES (?, ?, ?, ?)`,
			accountID, month, amount, stopped)
		return err
	}

	require.NoError(t, insert(savingsID, "2026-01", 1500000, 0))
	assert.Error(t, insert(savingsID, "2026-01", 1, 0), "duplicate (account, month) must be rejected")
	assert.Error(t, insert(savingsID, "2026-1", 1, 0), "month shape must be YYYY-MM")
	assert.Error(t, insert(savingsID, "2026-02", -1, 0), "negative amount must be rejected")
	assert.Error(t, insert(savingsID, "2026-03", 5, 1), "stopped row must have amount 0")
	assert.Error(t, insert(savingsID, "2026-04", 0, 2), "stopped must be 0 or 1")
	assert.Error(t, insert(999, "2026-05", 0, 0), "unknown account must be rejected")
	require.NoError(t, insert(savingsID, "2026-06", 0, 1))
	require.NoError(t, insert(savingsID, "2026-07", 0, 0), "zero target is valid")
}

func TestMigration0013_CascadeOnAccountDelete(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	savingsID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO savings_targets (account_id, effective_month, amount) VALUES (?, '2026-01', 100)`, savingsID)
	require.NoError(t, err)

	require.NoError(t, s.DeleteAccount(ctx, savingsID))

	var n int
	require.NoError(t, s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM savings_targets`).Scan(&n))
	assert.Equal(t, 0, n)
}

func TestMigration0013_Down(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	down, err := fs.ReadFile(migrations.FS, "0013_create_savings_targets.down.sql")
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx, string(down))
	require.NoError(t, err)

	var n int
	require.NoError(t, s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('savings_targets', 'idx_savings_targets_account_month')`).Scan(&n))
	assert.Equal(t, 0, n)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestMigration0013 -v`
Expected: FAIL (`no such table: savings_targets` / file not found).

- [ ] **Step 3: Write the migration**

`migrations/0013_create_savings_targets.up.sql`:

```sql
-- Monthly savings targets on Asset accounts. Each row is a version that
-- applies from effective_month (YYYY-MM) until a later row for the same account.
CREATE TABLE IF NOT EXISTS savings_targets (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id      INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    effective_month TEXT    NOT NULL CHECK (effective_month GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'),
    amount          INTEGER NOT NULL CHECK (amount >= 0),
    stopped         INTEGER NOT NULL DEFAULT 0 CHECK (stopped IN (0, 1)),
    CHECK (stopped = 0 OR amount = 0),
    UNIQUE (account_id, effective_month)
);

CREATE INDEX IF NOT EXISTS idx_savings_targets_account_month ON savings_targets (account_id, effective_month);
```

`migrations/0013_create_savings_targets.down.sql`:

```sql
DROP INDEX IF EXISTS idx_savings_targets_account_month;
DROP TABLE IF EXISTS savings_targets;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestMigration0013 -v`
Expected: PASS. Then `go test ./internal/store/` — all PASS.

- [ ] **Step 5: Commit**

```bash
git add migrations/0013_create_savings_targets.up.sql migrations/0013_create_savings_targets.down.sql internal/store/migration_0013_test.go
git commit -m "feat(store): add savings_targets migration

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Model, repository interface, store, service mock

**Files:**
- Create: `internal/model/savings.go`
- Modify: `internal/repository/interfaces.go` (after `BudgetRepository`, and the `Repository` interface)
- Create: `internal/store/sqlite_savings_target.go`
- Test: `internal/store/sqlite_savings_target_test.go`
- Modify: `internal/service/testhelper_test.go` (new `mockSavingsRepo`; `mockCombinedRepo`; `mockTransactionManager`)

**Interfaces:**
- Consumes: table from Task 1.
- Produces:
  - `model.SavingsTarget{ID, AccountID int64; AccountName, EffectiveMonth string; Amount int64; Stopped bool}`
  - `model.SetSavingsTargetInput{AccountName, EffectiveMonth string; Amount int64}`
  - `model.StopSavingsTargetInput{AccountName, EffectiveMonth string}`
  - `model.SavingsReport`, `model.SavingsReportRow`, `model.SavingsMonth` (fields below)
  - `repository.SavingsTargetRepository` with `UpsertSavingsTarget(ctx, accountID int64, month string, amount int64, stopped bool) (int64, error)`, `ListSavingsTargets(ctx) ([]model.SavingsTarget, error)`, `DeleteSavingsTarget(ctx, id int64) error`
  - `*store.Store` implements it.
  - Service test mock: `newMockSavingsRepo(accRepo *mockAccountRepo) *mockSavingsRepo`; `mockTransactionManager.savingsRepo *mockSavingsRepo`.

- [ ] **Step 1: Write the model**

`internal/model/savings.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// SavingsTarget is one version of a monthly savings target on an Asset
// account. It applies from EffectiveMonth (YYYY-MM) until a later version for
// the same account. A Stopped version ends the target; its Amount is always 0.
type SavingsTarget struct {
	ID             int64  `json:"id"`
	AccountID      int64  `json:"account_id"`
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
	Amount         int64  `json:"amount"` // cents
	Stopped        bool   `json:"stopped"`
}

// SetSavingsTargetInput sets (or replaces) the target version starting at EffectiveMonth.
type SetSavingsTargetInput struct {
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
	Amount         int64  `json:"amount"` // cents, >= 0
}

// StopSavingsTargetInput ends an account's savings target from EffectiveMonth on.
type StopSavingsTargetInput struct {
	AccountName    string `json:"account_name"`
	EffectiveMonth string `json:"effective_month"`
}

// SavingsReport compares each target active in Month with what was saved,
// for the month and year to date. Totals are per currency and only include
// rows with no targeted ancestor in the same currency.
type SavingsReport struct {
	Month          string             `json:"month"`
	Rows           []SavingsReportRow `json:"rows"`
	TotalTarget    map[string]int64   `json:"total_target"`
	TotalSaved     map[string]int64   `json:"total_saved"`
	TotalYTDTarget map[string]int64   `json:"total_ytd_target"`
	TotalYTDSaved  map[string]int64   `json:"total_ytd_saved"`
}

// SavingsReportRow is one targeted account. Saved is the signed sum of the
// splits on the account and its descendants in Currency, excluding Opening
// transactions; it may be negative. Remaining is Target - Saved (negative
// means the target was exceeded). YTD fields cover January of Month's year
// through Month, counting only months in which the target was active.
type SavingsReportRow struct {
	AccountID        int64          `json:"account_id"`
	AccountName      string         `json:"account_name"`
	Currency         string         `json:"currency"`
	EffectiveMonth   string         `json:"effective_month"`
	Target           int64          `json:"target"`
	Saved            int64          `json:"saved"`
	Remaining        int64          `json:"remaining"`
	YTDTarget        int64          `json:"ytd_target"`
	YTDSaved         int64          `json:"ytd_saved"`
	YTDRemaining     int64          `json:"ytd_remaining"`
	Months           []SavingsMonth `json:"months"`            // active months, ascending
	ExcludedAccounts []string       `json:"excluded_accounts"` // descendants skipped for a different currency in Month
}

// SavingsMonth is one active month in a row's year-to-date breakdown.
type SavingsMonth struct {
	Month  string `json:"month"`
	Target int64  `json:"target"`
	Saved  int64  `json:"saved"`
}
```

- [ ] **Step 2: Add the repository interface**

In `internal/repository/interfaces.go`, after `BudgetRepository`:

```go
// SavingsTargetRepository stores savings target versions. Selecting the
// version that applies to a month is a service concern.
type SavingsTargetRepository interface {
	// UpsertSavingsTarget inserts the (accountID, month) version or replaces
	// its amount and stopped flag, returning the row ID.
	UpsertSavingsTarget(ctx context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error)
	// ListSavingsTargets returns every version joined with its account name,
	// ordered by account name, then effective month.
	ListSavingsTargets(ctx context.Context) ([]model.SavingsTarget, error)
	// DeleteSavingsTarget removes one version. A missing id wraps ErrNotFound.
	DeleteSavingsTarget(ctx context.Context, id int64) error
}
```

and extend `Repository`:

```go
type Repository interface {
	AccountRepository
	TransactionRepository
	BudgetRepository
	SavingsTargetRepository
}
```

- [ ] **Step 3: Write the failing store tests**

`internal/store/sqlite_savings_target_test.go`:

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

func TestUpsertSavingsTarget_InsertThenReplace(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	accID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	id1, err := s.UpsertSavingsTarget(ctx, accID, "2026-01", 1500000, false)
	require.NoError(t, err)
	id2, err := s.UpsertSavingsTarget(ctx, accID, "2026-01", 2000000, false)
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "same (account, month) must update the same row")

	list, err := s.ListSavingsTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int64(2000000), list[0].Amount)
}

func TestUpsertSavingsTarget_SetAfterStopClearsStopped(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	accID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.UpsertSavingsTarget(ctx, accID, "2026-03", 0, true)
	require.NoError(t, err)
	_, err = s.UpsertSavingsTarget(ctx, accID, "2026-03", 5000, false)
	require.NoError(t, err)

	list, err := s.ListSavingsTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.False(t, list[0].Stopped)
	assert.Equal(t, int64(5000), list[0].Amount)
}

func TestListSavingsTargets_JoinsNameAndOrders(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	tripID, err := s.CreateAccount(ctx, "Assets:Trip", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	savID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.UpsertSavingsTarget(ctx, tripID, "2026-01", 1, false)
	require.NoError(t, err)
	_, err = s.UpsertSavingsTarget(ctx, savID, "2026-05", 2, false)
	require.NoError(t, err)
	_, err = s.UpsertSavingsTarget(ctx, savID, "2026-02", 3, false)
	require.NoError(t, err)

	list, err := s.ListSavingsTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "Assets:Savings", list[0].AccountName)
	assert.Equal(t, "2026-02", list[0].EffectiveMonth)
	assert.Equal(t, "2026-05", list[1].EffectiveMonth)
	assert.Equal(t, "Assets:Trip", list[2].AccountName)
}

func TestListSavingsTargets_EmptyIsNonNil(t *testing.T) {
	s := setupTestDB(t)
	list, err := s.ListSavingsTargets(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestListSavingsTargets_ReflectsAccountRename(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	accID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.UpsertSavingsTarget(ctx, accID, "2026-01", 1, false)
	require.NoError(t, err)

	require.NoError(t, s.RenameAccount(ctx, "Assets:Savings", "Assets:Banks:Cube_Saving"))

	list, err := s.ListSavingsTargets(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Assets:Banks:Cube_Saving", list[0].AccountName)
}

func TestDeleteSavingsTarget(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	accID, err := s.CreateAccount(ctx, "Assets:Savings", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	id, err := s.UpsertSavingsTarget(ctx, accID, "2026-01", 1, false)
	require.NoError(t, err)

	require.NoError(t, s.DeleteSavingsTarget(ctx, id))
	err = s.DeleteSavingsTarget(ctx, id)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/store/ -run SavingsTarget -v`
Expected: FAIL to compile (`s.UpsertSavingsTarget undefined`).

- [ ] **Step 5: Implement the store**

`internal/store/sqlite_savings_target.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"context"
	"fmt"

	"github.com/hance08/kea/internal/model"
)

// UpsertSavingsTarget inserts or replaces the savings target version for (accountID, month).
func (s *Store) UpsertSavingsTarget(ctx context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO savings_targets (account_id, effective_month, amount, stopped)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id, effective_month) DO UPDATE
		SET amount = excluded.amount, stopped = excluded.stopped
		RETURNING id`, accountID, month, amount, stopped).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert savings target: %w", err)
	}
	return id, nil
}

// ListSavingsTargets returns all savings target versions with their account names.
func (s *Store) ListSavingsTargets(ctx context.Context) ([]model.SavingsTarget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.account_id, a.name, t.effective_month, t.amount, t.stopped
		FROM savings_targets t
		JOIN accounts a ON a.id = t.account_id
		ORDER BY a.name, t.effective_month`)
	if err != nil {
		return nil, fmt.Errorf("failed to list savings targets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []model.SavingsTarget{}
	for rows.Next() {
		var t model.SavingsTarget
		if err := rows.Scan(&t.ID, &t.AccountID, &t.AccountName, &t.EffectiveMonth, &t.Amount, &t.Stopped); err != nil {
			return nil, fmt.Errorf("failed to scan savings target: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate savings targets: %w", err)
	}
	return out, nil
}

// DeleteSavingsTarget removes one savings target version.
func (s *Store) DeleteSavingsTarget(ctx context.Context, id int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM savings_targets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete savings target: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("savings target with ID %d not found: %w", id, ErrRecordNotFound)
	}
	return nil
}
```

- [ ] **Step 6: Add the service test mock**

In `internal/service/testhelper_test.go`, after the `mockBudgetRepo` section (before `mockCombinedRepo`):

```go
// ──────────────────────────────────────────────
// mockSavingsRepo
// ──────────────────────────────────────────────

type mockSavingsRepo struct {
	accRepo *mockAccountRepo // resolves AccountName on list, like the SQL JOIN
	rows    map[int64]*model.SavingsTarget
	nextID  int64

	upsertErr error
	listErr   error
	deleteErr error
}

func newMockSavingsRepo(accRepo *mockAccountRepo) *mockSavingsRepo {
	return &mockSavingsRepo{accRepo: accRepo, rows: make(map[int64]*model.SavingsTarget), nextID: 1}
}

func (m *mockSavingsRepo) UpsertSavingsTarget(_ context.Context, accountID int64, month string, amount int64, stopped bool) (int64, error) {
	if m.upsertErr != nil {
		return 0, m.upsertErr
	}
	for _, t := range m.rows {
		if t.AccountID == accountID && t.EffectiveMonth == month {
			t.Amount, t.Stopped = amount, stopped
			return t.ID, nil
		}
	}
	id := m.nextID
	m.nextID++
	m.rows[id] = &model.SavingsTarget{ID: id, AccountID: accountID, EffectiveMonth: month, Amount: amount, Stopped: stopped}
	return id, nil
}

func (m *mockSavingsRepo) ListSavingsTargets(_ context.Context) ([]model.SavingsTarget, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	out := make([]model.SavingsTarget, 0, len(m.rows))
	for _, t := range m.rows {
		cp := *t
		if acc, ok := m.accRepo.accountsByID[t.AccountID]; ok {
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

func (m *mockSavingsRepo) DeleteSavingsTarget(_ context.Context, id int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.rows[id]; !ok {
		return fmt.Errorf("savings target %d not found: %w", id, repository.ErrNotFound)
	}
	delete(m.rows, id)
	return nil
}
```

Then add `*mockSavingsRepo` to `mockCombinedRepo`:

```go
type mockCombinedRepo struct {
	*mockAccountRepo
	*mockTransactionRepo
	*mockBudgetRepo
	*mockSavingsRepo
}
```

Add a field to `mockTransactionManager` and pass it into the combined repo in `ExecTx`:

```go
type mockTransactionManager struct {
	accRepo     *mockAccountRepo
	txRepo      *mockTransactionRepo
	budgetRepo  *mockBudgetRepo
	savingsRepo *mockSavingsRepo
	failTx      bool
}
```

```go
	combined := &mockCombinedRepo{
		mockAccountRepo:     m.accRepo,
		mockTransactionRepo: m.txRepo,
		mockBudgetRepo:      m.budgetRepo,
		mockSavingsRepo:     m.savingsRepo,
	}
```

- [ ] **Step 7: Run tests to verify they pass and everything compiles**

Run: `go test ./internal/store/ -run SavingsTarget -v` — PASS.
Run: `go build ./... && go vet ./... && go test ./internal/service/ ./internal/store/` — all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/model/savings.go internal/repository/interfaces.go internal/store/sqlite_savings_target.go internal/store/sqlite_savings_target_test.go internal/service/testhelper_test.go
git commit -m "feat(store): add savings target model and repository

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Shared version helpers

Move version selection and the same-currency-ancestor check out of `BudgetService` into generic helpers. Budget behavior must not change: **do not edit `budget_service_test.go` or `budget_report_test.go`**; they must pass as they are.

**Files:**
- Create: `internal/service/versions.go`
- Test: `internal/service/versions_test.go`
- Modify: `internal/service/budget_service.go` (remove `latestVersions`; `activeBudgets` / `hasActiveBudget` become wrappers)
- Modify: `internal/service/budget_report.go` (`hasBudgetedAncestorInCurrency` becomes a wrapper)

**Interfaces:**
- Produces (package `service`, unexported):
  - `type versionKey struct { ID, AccountID int64; Month string; Stopped bool }`
  - `func latestVersions[T any](items []T, month string, key func(T) versionKey) map[int64]T`
  - `func activeVersions[T any](items []T, month string, key func(T) versionKey) []T` — never nil, input order
  - `func activeVersionFor[T any](items []T, accountID int64, month string, key func(T) versionKey) (T, bool)`
  - `func hasAncestorInCurrency[T any](row T, rows []T, nameCcy func(T) (name, currency string)) bool`
  - `func budgetVersionKey(b model.Budget) versionKey`, `func savingsVersionKey(t model.SavingsTarget) versionKey`
  - Unchanged names kept for budgets: `activeBudgets`, `hasActiveBudget`, `hasBudgetedAncestorInCurrency`, `isSelfOrDescendant`.

- [ ] **Step 1: Write the failing test**

`internal/service/versions_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
)

func targets() []model.SavingsTarget {
	return []model.SavingsTarget{
		{ID: 1, AccountID: 10, EffectiveMonth: "2026-01", Amount: 100},
		{ID: 2, AccountID: 10, EffectiveMonth: "2026-03", Stopped: true},
		{ID: 3, AccountID: 10, EffectiveMonth: "2026-05", Amount: 300},
		{ID: 4, AccountID: 20, EffectiveMonth: "2026-04", Amount: 50},
	}
}

func ids(ts []model.SavingsTarget) []int64 {
	out := make([]int64, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

func TestActiveVersions(t *testing.T) {
	all := targets()
	assert.Equal(t, []int64{}, ids(activeVersions([]model.SavingsTarget{}, "2026-01", savingsVersionKey)))
	assert.NotNil(t, activeVersions([]model.SavingsTarget{}, "2026-01", savingsVersionKey))
	assert.Equal(t, []int64{}, ids(activeVersions(all, "2025-12", savingsVersionKey)), "before first version")
	assert.Equal(t, []int64{1}, ids(activeVersions(all, "2026-02", savingsVersionKey)))
	assert.Equal(t, []int64{}, ids(activeVersions(all, "2026-03", savingsVersionKey)), "stopped")
	assert.Equal(t, []int64{4}, ids(activeVersions(all, "2026-04", savingsVersionKey)), "account 10 still stopped")
	assert.Equal(t, []int64{3, 4}, ids(activeVersions(all, "2026-09", savingsVersionKey)), "restarted")
}

func TestActiveVersionFor(t *testing.T) {
	all := targets()
	v, ok := activeVersionFor(all, 10, "2026-02", savingsVersionKey)
	assert.True(t, ok)
	assert.Equal(t, int64(100), v.Amount)
	_, ok = activeVersionFor(all, 10, "2026-04", savingsVersionKey)
	assert.False(t, ok, "stopped month")
	_, ok = activeVersionFor(all, 20, "2026-03", savingsVersionKey)
	assert.False(t, ok, "before first version")
	_, ok = activeVersionFor(all, 99, "2026-09", savingsVersionKey)
	assert.False(t, ok, "unknown account")
}

func TestHasAncestorInCurrency(t *testing.T) {
	rows := []model.SavingsReportRow{
		{AccountName: "Assets:Savings", Currency: "USD"},
		{AccountName: "Assets:Savings:Travel", Currency: "USD"},
		{AccountName: "Assets:Savings:Japan", Currency: "JPY"},
		{AccountName: "Assets:SavingsBox", Currency: "USD"},
	}
	nc := func(r model.SavingsReportRow) (string, string) { return r.AccountName, r.Currency }
	assert.False(t, hasAncestorInCurrency(rows[0], rows, nc))
	assert.True(t, hasAncestorInCurrency(rows[1], rows, nc))
	assert.False(t, hasAncestorInCurrency(rows[2], rows, nc), "ancestor in another currency")
	assert.False(t, hasAncestorInCurrency(rows[3], rows, nc), "name prefix is not an ancestor")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run 'TestActiveVersions|TestActiveVersionFor|TestHasAncestorInCurrency' -v`
Expected: FAIL to compile (`undefined: activeVersions`).

- [ ] **Step 3: Implement the helpers**

`internal/service/versions.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import "github.com/hance08/kea/internal/model"

// versionKey is what version selection needs from a per-account row that
// applies from Month until a later row for the same account (budgets,
// savings targets).
type versionKey struct {
	ID        int64
	AccountID int64
	Month     string
	Stopped   bool
}

func budgetVersionKey(b model.Budget) versionKey {
	return versionKey{ID: b.ID, AccountID: b.AccountID, Month: b.EffectiveMonth, Stopped: b.Stopped}
}

func savingsVersionKey(t model.SavingsTarget) versionKey {
	return versionKey{ID: t.ID, AccountID: t.AccountID, Month: t.EffectiveMonth, Stopped: t.Stopped}
}

// latestVersions returns, per account, the version with the greatest
// Month <= month (including stopped versions).
func latestVersions[T any](items []T, month string, key func(T) versionKey) map[int64]T {
	latest := map[int64]T{}
	latestMonth := map[int64]string{}
	for _, it := range items {
		k := key(it)
		if k.Month > month {
			continue
		}
		if cur, ok := latestMonth[k.AccountID]; !ok || k.Month > cur {
			latest[k.AccountID] = it
			latestMonth[k.AccountID] = k.Month
		}
	}
	return latest
}

// activeVersions returns the non-stopped version that applies to month for
// each account, in the input order. The result is never nil.
func activeVersions[T any](items []T, month string, key func(T) versionKey) []T {
	latest := latestVersions(items, month, key)
	out := []T{}
	for _, it := range items {
		k := key(it)
		if v, ok := latest[k.AccountID]; ok && key(v).ID == k.ID && !k.Stopped {
			out = append(out, it)
		}
	}
	return out
}

// activeVersionFor returns the non-stopped version that applies to month for
// accountID, if any.
func activeVersionFor[T any](items []T, accountID int64, month string, key func(T) versionKey) (T, bool) {
	v, ok := latestVersions(items, month, key)[accountID]
	if !ok || key(v).Stopped {
		var zero T
		return zero, false
	}
	return v, true
}

// hasAncestorInCurrency reports whether rows holds a strict ancestor of row's
// account in the same currency. Only then is row's amount already inside that
// ancestor's; an ancestor in a different currency excludes row's account, so
// row must still count in its own currency total.
func hasAncestorInCurrency[T any](row T, rows []T, nameCcy func(T) (name, currency string)) bool {
	name, ccy := nameCcy(row)
	for _, r := range rows {
		n, c := nameCcy(r)
		if c == ccy && n != name && isSelfOrDescendant(name, n) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Switch budgets to the helpers**

In `internal/service/budget_service.go`, delete the old `latestVersions` function and replace `activeBudgets` / `hasActiveBudget` with:

```go
// activeBudgets returns the non-stopped version that applies to month for
// each account, in the input order.
func activeBudgets(budgets []model.Budget, month string) []model.Budget {
	return activeVersions(budgets, month, budgetVersionKey)
}

func hasActiveBudget(budgets []model.Budget, accountID int64, month string) bool {
	_, ok := activeVersionFor(budgets, accountID, month, budgetVersionKey)
	return ok
}
```

In `internal/service/budget_report.go`, replace the body of `hasBudgetedAncestorInCurrency` (keep its doc comment):

```go
func hasBudgetedAncestorInCurrency(row model.BudgetReportRow, rows []model.BudgetReportRow) bool {
	return hasAncestorInCurrency(row, rows, func(r model.BudgetReportRow) (string, string) {
		return r.AccountName, r.Currency
	})
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/service/ -v -run 'Version|Ancestor|Budget'` — PASS.
Run: `git diff --stat internal/service/budget_service_test.go internal/service/budget_report_test.go` — must print nothing.
Run: `go vet ./internal/service/ && go test ./internal/service/` — PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/service/versions.go internal/service/versions_test.go internal/service/budget_service.go internal/service/budget_report.go
git commit -m "refactor(service): extract generic version selection helpers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: SavingsService CRUD and facade wiring

**Files:**
- Create: `internal/service/savings_service.go`
- Test: `internal/service/savings_service_test.go`
- Modify: `internal/service/service.go`
- Modify: `internal/app/app.go:76`
- Modify: `internal/api/testhelper_test.go` (three `service.NewService(st, st, st, st, cfg)` calls)
- Modify: `internal/service/testhelper_test.go` (add `newTestSavingsService`)

**Interfaces:**
- Consumes: Task 2 repository and mock; Task 3 `activeVersionFor`, `savingsVersionKey`; existing `ValidateBudgetMonth`, `validationErrorf`, `ErrNotFound`.
- Produces:
  - `func NewSavingsService(savingsRepo repository.SavingsTargetRepository, accRepo repository.AccountRepository, txRepo repository.TransactionRepository, tm repository.TransactionManager, cfg *config.Config) *SavingsService`
  - `(*SavingsService).SetSavingsTarget(ctx, model.SetSavingsTargetInput) (*model.SavingsTarget, error)`
  - `(*SavingsService).StopSavingsTarget(ctx, model.StopSavingsTargetInput) (*model.SavingsTarget, error)`
  - `(*SavingsService).ListSavingsTargets(ctx) ([]model.SavingsTarget, error)`
  - `(*SavingsService).DeleteSavingsTarget(ctx, id int64) error`
  - `NewService(accRepo, txRepo, budgetRepo, savingsRepo, tm, cfg)`; `(*Service).Savings() *SavingsService`
  - Test helper `newTestSavingsService(accRepo *mockAccountRepo, txRepo *mockTransactionRepo, sRepo *mockSavingsRepo) *SavingsService`

- [ ] **Step 1: Add the test factory**

In `internal/service/testhelper_test.go`, next to `newTestBudgetService`:

```go
func newTestSavingsService(accRepo *mockAccountRepo, txRepo *mockTransactionRepo, sRepo *mockSavingsRepo) *SavingsService {
	tm := &mockTransactionManager{accRepo: accRepo, txRepo: txRepo, savingsRepo: sRepo}
	return NewSavingsService(sRepo, accRepo, txRepo, tm, defaultConfig())
}
```

- [ ] **Step 2: Write the failing tests**

`internal/service/savings_service_test.go`:

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

func savingsCRUDFixture() (*mockAccountRepo, *mockSavingsRepo, *SavingsService) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Savings", Type: model.AccountTypeAsset, Currency: "USD"})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"})
	sRepo := newMockSavingsRepo(accRepo)
	return accRepo, sRepo, newTestSavingsService(accRepo, newMockTransactionRepo(), sRepo)
}

func requireField(t *testing.T, err error, field string) {
	t.Helper()
	var verr *ValidationError
	require.True(t, errors.As(err, &verr), "expected ValidationError, got %v", err)
	assert.Equal(t, field, verr.Field)
}

func TestSetSavingsTarget(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a version", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		got, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: " Assets:Savings ", EffectiveMonth: "2026-10", Amount: 1500000})
		require.NoError(t, err)
		assert.Equal(t, "Assets:Savings", got.AccountName)
		assert.Equal(t, int64(1), got.AccountID)
		assert.Equal(t, "2026-10", got.EffectiveMonth)
		assert.Equal(t, int64(1500000), got.Amount)
		assert.False(t, got.Stopped)
	})

	t.Run("zero is a valid target", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		got, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-10", Amount: 0})
		require.NoError(t, err)
		assert.Equal(t, int64(0), got.Amount)
	})

	t.Run("validation", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		_, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: " ", EffectiveMonth: "2026-10", Amount: 1})
		requireField(t, err, "account_name")
		_, err = svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-13", Amount: 1})
		requireField(t, err, "effective_month")
		_, err = svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-10", Amount: -1})
		requireField(t, err, "amount")
	})

	t.Run("non-Asset account is rejected", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		_, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-10", Amount: 1})
		requireField(t, err, "account_name")
	})

	t.Run("unknown account is not found", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		_, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Assets:Nope", EffectiveMonth: "2026-10", Amount: 1})
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		_, sRepo, svc := savingsCRUDFixture()
		sRepo.upsertErr = errors.New("boom")
		_, err := svc.SetSavingsTarget(ctx, model.SetSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-10", Amount: 1})
		assert.ErrorContains(t, err, "boom")
	})
}

func TestStopSavingsTarget(t *testing.T) {
	ctx := context.Background()

	t.Run("stops an active target", func(t *testing.T) {
		_, sRepo, svc := savingsCRUDFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		got, err := svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-04"})
		require.NoError(t, err)
		assert.True(t, got.Stopped)
		assert.Equal(t, int64(0), got.Amount)
		assert.Empty(t, activeVersions(mustListTargets(t, svc), "2026-04", savingsVersionKey))
		assert.Len(t, activeVersions(mustListTargets(t, svc), "2026-03", savingsVersionKey), 1)
	})

	t.Run("no active target is a validation error", func(t *testing.T) {
		_, sRepo, svc := savingsCRUDFixture()
		_, err := svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-04"})
		requireField(t, err, "effective_month")

		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-02", 0, true)
		_, err = svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-04"})
		requireField(t, err, "effective_month")
	})

	t.Run("validation and type", func(t *testing.T) {
		_, _, svc := savingsCRUDFixture()
		_, err := svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "", EffectiveMonth: "2026-04"})
		requireField(t, err, "account_name")
		_, err = svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "x"})
		requireField(t, err, "effective_month")
		_, err = svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: "Expenses:Food", EffectiveMonth: "2026-04"})
		requireField(t, err, "account_name")
	})
}

func mustListTargets(t *testing.T, svc *SavingsService) []model.SavingsTarget {
	t.Helper()
	list, err := svc.ListSavingsTargets(context.Background())
	require.NoError(t, err)
	return list
}

func TestListAndDeleteSavingsTargets(t *testing.T) {
	ctx := context.Background()
	_, sRepo, svc := savingsCRUDFixture()

	list := mustListTargets(t, svc)
	assert.NotNil(t, list)
	assert.Empty(t, list)

	id, _ := sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
	list = mustListTargets(t, svc)
	require.Len(t, list, 1)
	assert.Equal(t, "Assets:Savings", list[0].AccountName)

	require.NoError(t, svc.DeleteSavingsTarget(ctx, id))
	assert.ErrorIs(t, svc.DeleteSavingsTarget(ctx, id), ErrNotFound)

	sRepo.listErr = errors.New("boom")
	_, err := svc.ListSavingsTargets(ctx)
	assert.ErrorContains(t, err, "boom")
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/service/ -run 'SavingsTarget' -v`
Expected: FAIL to compile (`undefined: NewSavingsService`).

- [ ] **Step 4: Implement the service**

`internal/service/savings_service.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

// SavingsService manages monthly savings targets on Asset accounts and
// reports target vs saved.
type SavingsService struct {
	savingsRepo repository.SavingsTargetRepository
	accRepo     repository.AccountRepository
	txRepo      repository.TransactionRepository
	tm          repository.TransactionManager
	config      *config.Config
}

func NewSavingsService(
	savingsRepo repository.SavingsTargetRepository,
	accRepo repository.AccountRepository,
	txRepo repository.TransactionRepository,
	tm repository.TransactionManager,
	cfg *config.Config,
) *SavingsService {
	return &SavingsService{savingsRepo: savingsRepo, accRepo: accRepo, txRepo: txRepo, tm: tm, config: cfg}
}

// SetSavingsTarget creates or replaces the target version for an account
// starting at input.EffectiveMonth.
func (ss *SavingsService) SetSavingsTarget(ctx context.Context, input model.SetSavingsTargetInput) (*model.SavingsTarget, error) {
	name := strings.TrimSpace(input.AccountName)
	if name == "" {
		return nil, validationErrorf("account_name", "account name is required")
	}
	if err := ValidateBudgetMonth(input.EffectiveMonth); err != nil {
		return nil, err
	}
	if input.Amount < 0 {
		return nil, validationErrorf("amount", "savings target must not be negative")
	}

	var out *model.SavingsTarget
	err := ss.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, err := savingsAccount(ctx, repo, name)
		if err != nil {
			return err
		}
		id, err := repo.UpsertSavingsTarget(ctx, acc.ID, input.EffectiveMonth, input.Amount, false)
		if err != nil {
			return fmt.Errorf("failed to save savings target: %w", err)
		}
		out = &model.SavingsTarget{ID: id, AccountID: acc.ID, AccountName: acc.Name, EffectiveMonth: input.EffectiveMonth, Amount: input.Amount}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StopSavingsTarget ends an account's target from input.EffectiveMonth on.
func (ss *SavingsService) StopSavingsTarget(ctx context.Context, input model.StopSavingsTargetInput) (*model.SavingsTarget, error) {
	name := strings.TrimSpace(input.AccountName)
	if name == "" {
		return nil, validationErrorf("account_name", "account name is required")
	}
	if err := ValidateBudgetMonth(input.EffectiveMonth); err != nil {
		return nil, err
	}

	var out *model.SavingsTarget
	err := ss.tm.ExecTx(ctx, func(repo repository.Repository) error {
		acc, err := savingsAccount(ctx, repo, name)
		if err != nil {
			return err
		}
		all, err := repo.ListSavingsTargets(ctx)
		if err != nil {
			return fmt.Errorf("failed to list savings targets: %w", err)
		}
		if _, ok := activeVersionFor(all, acc.ID, input.EffectiveMonth, savingsVersionKey); !ok {
			return validationErrorf("effective_month", "account %q has no active savings target in %s", acc.Name, input.EffectiveMonth)
		}
		id, err := repo.UpsertSavingsTarget(ctx, acc.ID, input.EffectiveMonth, 0, true)
		if err != nil {
			return fmt.Errorf("failed to stop savings target: %w", err)
		}
		out = &model.SavingsTarget{ID: id, AccountID: acc.ID, AccountName: acc.Name, EffectiveMonth: input.EffectiveMonth, Stopped: true}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListSavingsTargets returns every target version, ordered by account name then month.
func (ss *SavingsService) ListSavingsTargets(ctx context.Context) ([]model.SavingsTarget, error) {
	list, err := ss.savingsRepo.ListSavingsTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list savings targets: %w", err)
	}
	if list == nil {
		list = []model.SavingsTarget{}
	}
	return list, nil
}

// DeleteSavingsTarget removes a single target version.
func (ss *SavingsService) DeleteSavingsTarget(ctx context.Context, id int64) error {
	if err := ss.savingsRepo.DeleteSavingsTarget(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("savings target %d: %w", id, ErrNotFound)
		}
		return fmt.Errorf("failed to delete savings target: %w", err)
	}
	return nil
}

func (ss *SavingsService) currencyOrDefault(ccy string) string {
	if ccy == "" {
		return ss.config.Defaults.Currency
	}
	return ccy
}

// savingsAccount resolves name to an Asset account.
func savingsAccount(ctx context.Context, repo repository.AccountRepository, name string) (*model.Account, error) {
	acc, err := repo.GetAccountByName(ctx, name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("account %q: %w", name, ErrNotFound)
		}
		return nil, fmt.Errorf("failed to look up account %q: %w", name, err)
	}
	if acc.Type != model.AccountTypeAsset {
		return nil, validationErrorf("account_name", "savings targets can only be set on Asset accounts; %q is type %s", acc.Name, acc.Type)
	}
	return acc, nil
}
```

- [ ] **Step 5: Wire the facade**

`internal/service/service.go` becomes:

```go
type Service struct {
	account     *AccountService
	transaction *TransactionService
	budget      *BudgetService
	savings     *SavingsService
	config      *config.Config
}

func NewService(
	accRepo repository.AccountRepository,
	txRepo repository.TransactionRepository,
	budgetRepo repository.BudgetRepository,
	savingsRepo repository.SavingsTargetRepository,
	tm repository.TransactionManager,
	cfg *config.Config,
) *Service {
	svc := &Service{
		account:     NewAccountService(accRepo, cfg, tm),
		transaction: NewTransactionService(txRepo, accRepo, tm, cfg),
		budget:      NewBudgetService(budgetRepo, accRepo, txRepo, tm, cfg),
		savings:     NewSavingsService(savingsRepo, accRepo, txRepo, tm, cfg),
		config:      cfg,
	}

	return svc
}

func (s *Service) Account() *AccountService         { return s.account }
func (s *Service) Transaction() *TransactionService { return s.transaction }
func (s *Service) Budget() *BudgetService           { return s.budget }
func (s *Service) Savings() *SavingsService         { return s.savings }
func (s *Service) Config() *config.Config           { return s.config }
```

`internal/app/app.go:76`: `svc := service.NewService(dbStore, dbStore, dbStore, dbStore, dbStore, cfg)`

`internal/api/testhelper_test.go`: replace each of the three `service.NewService(st, st, st, st, cfg)` with `service.NewService(st, st, st, st, st, cfg)`.

Run `grep -rn "NewService(" --include='*.go' .` and confirm no other caller remains on the old arity.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/service/ -run 'SavingsTarget' -v` — PASS.
Run: `go build ./... && go vet ./... && go test ./...` — all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/service/savings_service.go internal/service/savings_service_test.go internal/service/service.go internal/service/testhelper_test.go internal/app/app.go internal/api/testhelper_test.go
git commit -m "feat(service): add SavingsService for savings target versions

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Savings report

**Files:**
- Create: `internal/service/savings_report.go`
- Test: `internal/service/savings_report_test.go`

**Interfaces:**
- Consumes: Task 3 helpers, Task 4 service, existing `parseMonth`, `isSelfOrDescendant`, `TransactionRepository.GetSplitsWithAccountsByDateRange`, `GetTransactionsByDateRange`.
- Produces: `(*SavingsService).GenerateSavingsReport(ctx context.Context, month string) (*model.SavingsReport, error)`.

Algorithm (from the spec):
1. Empty `month` -> `time.Now().Format("2006-01")` (local). Invalid -> `ValidationError{Field: "month"}`.
2. Load all targets; rows = `activeVersions(all, month, savingsVersionKey)`. None -> empty report (non-nil slices and maps).
3. Load accounts; load splits and transactions once over `[local Jan 1 of month's year, end of month]`.
4. Map each non-`Opening` transaction to its local `YYYY-MM` (`time.Unix(ts, 0).Format("2006-01")`). Ignore buckets outside `[YYYY-01, month]` (the mock repository ignores the date range, and this keeps the rule explicit).
5. Per row account N in currency C, per month bucket: splits on N or a descendant in C add their signed amount; other currency -> add to `ExcludedAccounts` only when the bucket is `month`.
6. `Target`, `Saved`, `Remaining`. For each month `YYYY-01..month` where `activeVersionFor` finds a version, append `SavingsMonth` and add to YTD.
7. Sort rows by account name; totals per currency skip rows with a same-currency targeted ancestor.

- [ ] **Step 1: Write the failing tests**

`internal/service/savings_report_test.go`:

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

// savingsFixture: Assets:Savings (USD) with children Travel (USD) and Japan
// (JPY); Assets:SavingsBox (USD, a name-prefix trap); Assets:Bank (USD);
// Assets:Emergency (default currency); Income:Interest.
func savingsFixture() (*mockAccountRepo, *mockTransactionRepo, *mockSavingsRepo) {
	accRepo := newMockAccountRepo()
	for _, a := range []*model.Account{
		{ID: 1, Name: "Assets:Savings", Type: model.AccountTypeAsset, Currency: "USD"},
		{ID: 2, Name: "Assets:Savings:Travel", Type: model.AccountTypeAsset, Currency: "USD"},
		{ID: 3, Name: "Assets:Savings:Japan", Type: model.AccountTypeAsset, Currency: "JPY"},
		{ID: 4, Name: "Assets:SavingsBox", Type: model.AccountTypeAsset, Currency: "USD"},
		{ID: 5, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"},
		{ID: 6, Name: "Assets:Emergency", Type: model.AccountTypeAsset, Currency: ""},
		{ID: 7, Name: "Income:Interest", Type: model.AccountTypeRevenue, Currency: "USD"},
	} {
		accRepo.addAccount(a)
	}
	return accRepo, newMockTransactionRepo(), newMockSavingsRepo(accRepo)
}

// at returns noon local time on the given day.
func at(y int, m time.Month, d int) int64 {
	return time.Date(y, m, d, 12, 0, 0, 0, time.Local).Unix()
}

func addSavingsTx(txRepo *mockTransactionRepo, id, ts int64, txType model.TransactionType, splits ...model.SplitDetail) {
	addTxSplits(txRepo.splitsWithAccts, id, splits...)
	txRepo.addTransaction(&model.Transaction{ID: id, Timestamp: ts, Type: txType}, nil)
}

// transferIn moves amount from Assets:Bank into account.
func transferIn(txRepo *mockTransactionRepo, id, ts int64, account string, amount int64) {
	addSavingsTx(txRepo, id, ts, model.TxTypeTransfer,
		split(account, model.AccountTypeAsset, amount),
		split("Assets:Bank", model.AccountTypeAsset, -amount))
}

func findSavingsRow(t *testing.T, r *model.SavingsReport, name string) model.SavingsReportRow {
	t.Helper()
	for _, row := range r.Rows {
		if row.AccountName == name {
			return row
		}
	}
	t.Fatalf("row %q not found", name)
	return model.SavingsReportRow{}
}

func TestGenerateSavingsReport(t *testing.T) {
	ctx := context.Background()

	t.Run("signed sum over the account and its descendants", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-03", 10000, false)
		transferIn(txRepo, 1, at(2026, 3, 5), "Assets:Savings", 8000)
		transferIn(txRepo, 2, at(2026, 3, 10), "Assets:Savings:Travel", 3000)
		addSavingsTx(txRepo, 3, at(2026, 3, 20), model.TxTypeIncome, // interest
			split("Assets:Savings", model.AccountTypeAsset, 100),
			split("Income:Interest", model.AccountTypeRevenue, -100))
		transferIn(txRepo, 4, at(2026, 3, 25), "Assets:Savings", -2000) // withdrawal
		addSavingsTx(txRepo, 5, at(2026, 3, 26), model.TxTypeTransfer, // inside the subtree
			split("Assets:Savings", model.AccountTypeAsset, -500),
			split("Assets:Savings:Travel", model.AccountTypeAsset, 500))
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, "2026-03", r.Month)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, "USD", row.Currency)
		assert.Equal(t, "2026-03", row.EffectiveMonth)
		assert.Equal(t, int64(10000), row.Target)
		assert.Equal(t, int64(9100), row.Saved)
		assert.Equal(t, int64(900), row.Remaining)
		assert.Equal(t, []string{}, row.ExcludedAccounts)
	})

	t.Run("name prefix is not a descendant", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		transferIn(txRepo, 1, at(2026, 1, 5), "Assets:SavingsBox", 900)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, int64(0), findSavingsRow(t, r, "Assets:Savings").Saved)
	})

	t.Run("opening transactions do not count", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		addSavingsTx(txRepo, 1, at(2026, 1, 1), model.TxTypeOpening,
			split("Assets:Savings", model.AccountTypeAsset, 5000000),
			split("Equity:OpeningBalances_USD", model.AccountTypeEquity, -5000000))
		transferIn(txRepo, 2, at(2026, 1, 9), "Assets:Savings", 300)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, int64(300), row.Saved)
		assert.Equal(t, int64(300), row.YTDSaved)
	})

	t.Run("every non-opening type counts", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 0, false)
		for i, typ := range []model.TransactionType{model.TxTypeDeposit, model.TxTypeWithdrawal, model.TxTypeInvestment, model.TxTypeExpense, model.TxTypeOther} {
			addSavingsTx(txRepo, int64(i+1), at(2026, 1, 10), typ,
				split("Assets:Savings", model.AccountTypeAsset, 10),
				split("Assets:Bank", model.AccountTypeAsset, -10))
		}
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, int64(50), findSavingsRow(t, r, "Assets:Savings").Saved)
	})

	t.Run("other-currency descendants are excluded and listed for the month only", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		jpyIn := func(id, ts int64) {
			japan := split("Assets:Savings:Japan", model.AccountTypeAsset, 50000)
			japan.Currency = "JPY"
			bank := split("Assets:Bank", model.AccountTypeAsset, -50000)
			bank.Currency = "JPY"
			addSavingsTx(txRepo, id, ts, model.TxTypeTransfer, japan, bank)
		}
		jpyIn(1, at(2026, 2, 3))
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-03")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, int64(0), row.YTDSaved)
		assert.Equal(t, []string{}, row.ExcludedAccounts, "February JPY activity is not listed for March")

		jpyIn(2, at(2026, 3, 3))
		r, err = svc.GenerateSavingsReport(ctx, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, []string{"Assets:Savings:Japan"}, findSavingsRow(t, r, "Assets:Savings").ExcludedAccounts)
	})

	t.Run("ytd counts only active months, across stop and restart", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 1000, false)
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-03", 0, true)
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-05", 2000, false)
		for m := time.January; m <= time.June; m++ {
			transferIn(txRepo, int64(m), at(2026, m, 15), "Assets:Savings", 500)
		}
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-06")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, "2026-05", row.EffectiveMonth)
		assert.Equal(t, int64(2000), row.Target)
		assert.Equal(t, int64(500), row.Saved)
		assert.Equal(t, []model.SavingsMonth{
			{Month: "2026-01", Target: 1000, Saved: 500},
			{Month: "2026-02", Target: 1000, Saved: 500},
			{Month: "2026-05", Target: 2000, Saved: 500},
			{Month: "2026-06", Target: 2000, Saved: 500},
		}, row.Months)
		assert.Equal(t, int64(6000), row.YTDTarget)
		assert.Equal(t, int64(2000), row.YTDSaved)
		assert.Equal(t, int64(4000), row.YTDRemaining)
	})

	t.Run("target starting after january ignores earlier months", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-04", 1000, false)
		transferIn(txRepo, 1, at(2026, 2, 10), "Assets:Savings", 700)
		transferIn(txRepo, 2, at(2026, 4, 10), "Assets:Savings", 300)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-04")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, []model.SavingsMonth{{Month: "2026-04", Target: 1000, Saved: 300}}, row.Months)
		assert.Equal(t, int64(300), row.YTDSaved)
		assert.Equal(t, int64(1000), row.YTDTarget)
	})

	t.Run("january ytd ignores the previous year", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2025-06", 1000, false)
		transferIn(txRepo, 1, at(2025, 12, 10), "Assets:Savings", 5000)
		transferIn(txRepo, 2, at(2026, 1, 10), "Assets:Savings", 400)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, int64(400), row.Saved)
		assert.Equal(t, []model.SavingsMonth{{Month: "2026-01", Target: 1000, Saved: 400}}, row.Months)
		assert.Equal(t, int64(400), row.YTDSaved)
	})

	t.Run("months are bucketed in local time", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 1000, false)
		transferIn(txRepo, 1, time.Date(2026, 1, 31, 23, 30, 0, 0, time.Local).Unix(), "Assets:Savings", 400)
		transferIn(txRepo, 2, time.Date(2026, 2, 1, 0, 15, 0, 0, time.Local).Unix(), "Assets:Savings", 600)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, int64(400), findSavingsRow(t, r, "Assets:Savings").Saved)

		r, err = svc.GenerateSavingsReport(ctx, "2026-02")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, int64(600), row.Saved)
		assert.Equal(t, int64(1000), row.YTDSaved)
	})

	t.Run("net withdrawal gives remaining above target", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 1000, false)
		transferIn(txRepo, 1, at(2026, 1, 10), "Assets:Savings", -300)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		row := findSavingsRow(t, r, "Assets:Savings")
		assert.Equal(t, int64(-300), row.Saved)
		assert.Equal(t, int64(1300), row.Remaining)
	})

	t.Run("totals skip same-currency targeted descendants", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 10000, false) // Savings
		_, _ = sRepo.UpsertSavingsTarget(ctx, 2, "2026-01", 4000, false)  // Savings:Travel
		_, _ = sRepo.UpsertSavingsTarget(ctx, 3, "2026-01", 70000, false) // Savings:Japan (JPY)
		_, _ = sRepo.UpsertSavingsTarget(ctx, 6, "2026-01", 2000, false)  // Emergency (default USD)
		transferIn(txRepo, 1, at(2026, 1, 3), "Assets:Savings:Travel", 1500)
		transferIn(txRepo, 2, at(2026, 1, 4), "Assets:Emergency", 500)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-01")
		require.NoError(t, err)
		require.Len(t, r.Rows, 4)
		assert.Equal(t, []string{"Assets:Emergency", "Assets:Savings", "Assets:Savings:Japan", "Assets:Savings:Travel"},
			[]string{r.Rows[0].AccountName, r.Rows[1].AccountName, r.Rows[2].AccountName, r.Rows[3].AccountName})
		assert.Equal(t, "USD", findSavingsRow(t, r, "Assets:Emergency").Currency)
		assert.Equal(t, map[string]int64{"USD": 12000, "JPY": 70000}, r.TotalTarget)
		assert.Equal(t, map[string]int64{"USD": 2000, "JPY": 0}, r.TotalSaved)
		assert.Equal(t, map[string]int64{"USD": 12000, "JPY": 70000}, r.TotalYTDTarget)
		assert.Equal(t, map[string]int64{"USD": 2000, "JPY": 0}, r.TotalYTDSaved)
	})

	t.Run("no active target gives an empty, non-nil report", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-02", 0, true)
		svc := newTestSavingsService(accRepo, txRepo, sRepo)

		r, err := svc.GenerateSavingsReport(ctx, "2026-03")
		require.NoError(t, err)
		assert.NotNil(t, r.Rows)
		assert.Empty(t, r.Rows)
		for _, m := range []map[string]int64{r.TotalTarget, r.TotalSaved, r.TotalYTDTarget, r.TotalYTDSaved} {
			assert.NotNil(t, m)
		}
	})

	t.Run("default month is the current local month", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		svc := newTestSavingsService(accRepo, txRepo, sRepo)
		r, err := svc.GenerateSavingsReport(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, time.Now().Format("2006-01"), r.Month)
	})

	t.Run("invalid month", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		svc := newTestSavingsService(accRepo, txRepo, sRepo)
		_, err := svc.GenerateSavingsReport(ctx, "2026-13")
		requireField(t, err, "month")
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		accRepo, txRepo, sRepo := savingsFixture()
		_, _ = sRepo.UpsertSavingsTarget(ctx, 1, "2026-01", 100, false)
		txRepo.splitsRangeErr = errors.New("boom")
		svc := newTestSavingsService(accRepo, txRepo, sRepo)
		_, err := svc.GenerateSavingsReport(ctx, "2026-01")
		assert.ErrorContains(t, err, "boom")
	})
}
```

Note: `split()` (in `transaction_classifier_test.go`) sets `Currency: "USD"`; `addTxSplits` is in `report_service_test.go`; `requireField` is in `savings_service_test.go` (Task 4).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestGenerateSavingsReport -v`
Expected: FAIL to compile (`svc.GenerateSavingsReport undefined`).

- [ ] **Step 3: Implement the report**

`internal/service/savings_report.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hance08/kea/internal/model"
)

// GenerateSavingsReport compares each savings target active in month (YYYY-MM,
// local time; empty means the current month) with what was saved, for the
// month and from January of the same year.
func (ss *SavingsService) GenerateSavingsReport(ctx context.Context, month string) (*model.SavingsReport, error) {
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	if err := ValidateBudgetMonth(month); err != nil {
		return nil, validationErrorf("month", "invalid month %q, expected YYYY-MM", month)
	}
	firstMonth := month[:4] + "-01"
	start, _, _, err := parseMonth(firstMonth)
	if err != nil {
		return nil, err
	}
	_, end, _, err := parseMonth(month)
	if err != nil {
		return nil, err
	}

	report := &model.SavingsReport{
		Month:          month,
		Rows:           []model.SavingsReportRow{},
		TotalTarget:    map[string]int64{},
		TotalSaved:     map[string]int64{},
		TotalYTDTarget: map[string]int64{},
		TotalYTDSaved:  map[string]int64{},
	}

	all, err := ss.savingsRepo.ListSavingsTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list savings targets: %w", err)
	}
	active := activeVersions(all, month, savingsVersionKey)
	if len(active) == 0 {
		return report, nil
	}

	accounts, err := ss.accRepo.GetAllAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load accounts: %w", err)
	}
	accByID := make(map[int64]*model.Account, len(accounts))
	for _, a := range accounts {
		accByID[a.ID] = a
	}

	splitsByTx, err := ss.txRepo.GetSplitsWithAccountsByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load splits: %w", err)
	}
	txs, err := ss.txRepo.GetTransactionsByDateRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load transactions: %w", err)
	}
	// Local YYYY-MM of every counted transaction. Opening balances are
	// existing money, not money saved, so they never count.
	txMonth := make(map[int64]string, len(txs))
	for _, tx := range txs {
		if tx.Type == model.TxTypeOpening {
			continue
		}
		m := time.Unix(tx.Timestamp, 0).Format("2006-01")
		if m < firstMonth || m > month {
			continue
		}
		txMonth[tx.ID] = m
	}

	months := monthsFromJanuary(month)
	for _, t := range active {
		acc, ok := accByID[t.AccountID]
		if !ok {
			continue
		}
		row := model.SavingsReportRow{
			AccountID:        acc.ID,
			AccountName:      acc.Name,
			Currency:         ss.currencyOrDefault(acc.Currency),
			EffectiveMonth:   t.EffectiveMonth,
			Target:           t.Amount,
			Months:           []model.SavingsMonth{},
			ExcludedAccounts: []string{},
		}
		saved := map[string]int64{}
		excluded := map[string]struct{}{}
		for txID, details := range splitsByTx {
			m, ok := txMonth[txID]
			if !ok {
				continue
			}
			for _, d := range details {
				if !isSelfOrDescendant(d.AccountName, acc.Name) {
					continue
				}
				if ss.currencyOrDefault(d.Currency) != row.Currency {
					if m == month {
						excluded[d.AccountName] = struct{}{}
					}
					continue
				}
				saved[m] += d.Amount
			}
		}
		for name := range excluded {
			row.ExcludedAccounts = append(row.ExcludedAccounts, name)
		}
		sort.Strings(row.ExcludedAccounts)

		row.Saved = saved[month]
		row.Remaining = row.Target - row.Saved
		for _, m := range months {
			v, ok := activeVersionFor(all, acc.ID, m, savingsVersionKey)
			if !ok {
				continue
			}
			row.Months = append(row.Months, model.SavingsMonth{Month: m, Target: v.Amount, Saved: saved[m]})
			row.YTDTarget += v.Amount
			row.YTDSaved += saved[m]
		}
		row.YTDRemaining = row.YTDTarget - row.YTDSaved
		report.Rows = append(report.Rows, row)
	}

	sort.Slice(report.Rows, func(i, j int) bool { return report.Rows[i].AccountName < report.Rows[j].AccountName })

	nameCcy := func(r model.SavingsReportRow) (string, string) { return r.AccountName, r.Currency }
	for _, row := range report.Rows {
		if hasAncestorInCurrency(row, report.Rows, nameCcy) {
			continue
		}
		report.TotalTarget[row.Currency] += row.Target
		report.TotalSaved[row.Currency] += row.Saved
		report.TotalYTDTarget[row.Currency] += row.YTDTarget
		report.TotalYTDSaved[row.Currency] += row.YTDSaved
	}
	return report, nil
}

// monthsFromJanuary returns "YYYY-01" through month (a validated YYYY-MM).
func monthsFromJanuary(month string) []string {
	var n int
	_, _ = fmt.Sscanf(month[5:], "%d", &n)
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, fmt.Sprintf("%s-%02d", month[:4], i))
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestGenerateSavingsReport -v` — PASS.
Run: `go vet ./internal/service/ && go test ./internal/service/` — PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/savings_report.go internal/service/savings_report_test.go
git commit -m "feat(service): add savings report with year-to-date progress

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: HTTP API

**Files:**
- Create: `internal/api/savings.go`
- Modify: `internal/api/router.go` (next to the budget routes)
- Test: `internal/api/savings_test.go`
- Modify: `docs/http-api.md`

**Interfaces:**
- Consumes: `svc.Savings()` methods from Tasks 4–5; existing `decodeJSON`, `writeJSON`, `parseInt64Path`, `apiHandler`.
- Produces: `GET/PUT /api/savings-targets`, `POST /api/savings-targets/stop`, `DELETE /api/savings-targets/{id}`, `GET /api/reports/savings`.

- [ ] **Step 1: Write the failing tests**

`internal/api/savings_test.go` (`putJSON`, `decodeErrBody` live in `budgets_test.go`; `getJSON`, `postJSON`, `deleteURL` in `ledgers_test.go`; `itoa` in `accounts_test.go`; `seedAccount`, `seedTransaction` in `testhelper_test.go`):

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
)

func TestHandleSetSavingsTarget_OK(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Savings", model.AccountTypeAsset, 0)

	status, body := putJSON(t, ts.URL+"/api/savings-targets",
		`{"account_name":"Assets:Savings","effective_month":"2026-01","amount":1500000}`)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var got model.SavingsTarget
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.AccountName != "Assets:Savings" || got.Amount != 1500000 || got.EffectiveMonth != "2026-01" {
		t.Errorf("unexpected target: %+v", got)
	}
}

func TestHandleSetSavingsTarget_ValidationAndNotFound(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Savings", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Expenses:Food", model.AccountTypeExpense, 0)

	cases := []struct {
		name   string
		body   string
		status int
		field  string
	}{
		{"negative amount", `{"account_name":"Assets:Savings","effective_month":"2026-01","amount":-1}`, 400, "amount"},
		{"bad month", `{"account_name":"Assets:Savings","effective_month":"2026-13","amount":1}`, 400, "effective_month"},
		{"missing month", `{"account_name":"Assets:Savings","amount":1}`, 400, "effective_month"},
		{"non-asset", `{"account_name":"Expenses:Food","effective_month":"2026-01","amount":1}`, 400, "account_name"},
		{"unknown field", `{"account_name":"Assets:Savings","effective_month":"2026-01","amount":1,"x":1}`, 400, "body"},
		{"unknown account", `{"account_name":"Assets:Nope","effective_month":"2026-01","amount":1}`, 404, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := putJSON(t, ts.URL+"/api/savings-targets", tc.body)
			if status != tc.status {
				t.Fatalf("status %d, want %d: %s", status, tc.status, body)
			}
			if tc.field != "" && decodeErrBody(t, body).Field != tc.field {
				t.Errorf("field = %q, want %q", decodeErrBody(t, body).Field, tc.field)
			}
		})
	}
}

func TestHandleListStopDeleteSavingsTargets(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Savings", model.AccountTypeAsset, 0)

	status, body := getJSON(t, ts.URL+"/api/savings-targets")
	if status != http.StatusOK || string(bytes.TrimSpace(body)) != `{"items":[]}` {
		t.Fatalf("empty list: %d %s", status, body)
	}

	if status, body := putJSON(t, ts.URL+"/api/savings-targets",
		`{"account_name":"Assets:Savings","effective_month":"2026-01","amount":100}`); status != 200 {
		t.Fatalf("set: %d %s", status, body)
	}

	status, body = postJSON(t, ts.URL+"/api/savings-targets/stop",
		map[string]string{"account_name": "Assets:Savings", "effective_month": "2026-03"})
	if status != http.StatusOK {
		t.Fatalf("stop: %d %s", status, body)
	}
	var stopped model.SavingsTarget
	_ = json.Unmarshal(body, &stopped)
	if !stopped.Stopped {
		t.Errorf("expected stopped target, got %+v", stopped)
	}

	status, body = postJSON(t, ts.URL+"/api/savings-targets/stop",
		map[string]string{"account_name": "Assets:Savings", "effective_month": "2026-05"})
	if status != http.StatusBadRequest || decodeErrBody(t, body).Field != "effective_month" {
		t.Fatalf("stop twice: %d %s", status, body)
	}

	status, body = getJSON(t, ts.URL+"/api/savings-targets")
	var list struct{ Items []model.SavingsTarget }
	_ = json.Unmarshal(body, &list)
	if status != 200 || len(list.Items) != 2 {
		t.Fatalf("list: %d %s", status, body)
	}

	id := list.Items[0].ID
	status, body = deleteURL(t, ts.URL+"/api/savings-targets/"+itoa(id))
	if status != http.StatusOK {
		t.Fatalf("delete: %d %s", status, body)
	}
	status, _ = deleteURL(t, ts.URL+"/api/savings-targets/"+itoa(id))
	if status != http.StatusNotFound {
		t.Errorf("delete again: %d, want 404", status)
	}
	status, _ = deleteURL(t, ts.URL+"/api/savings-targets/abc")
	if status != http.StatusBadRequest {
		t.Errorf("delete bad id: %d, want 400", status)
	}
}

func TestHandleSavingsReport(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 100000)
	// Created with an opening balance: the Opening transaction must not count as saved.
	seedAccount(t, svc, "Assets:Savings", model.AccountTypeAsset, 500000)
	if _, err := svc.Savings().SetSavingsTarget(t.Context(), model.SetSavingsTargetInput{
		AccountName: "Assets:Savings", EffectiveMonth: "2026-05", Amount: 5000,
	}); err != nil {
		t.Fatal(err)
	}
	ts15 := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local).Unix()
	seedTransaction(t, svc, "Assets:Cash", "Assets:Savings", 1200, ts15, "save", model.TxTypeTransfer, model.StatusCleared)

	status, body := getJSON(t, ts.URL+"/api/reports/savings?month=2026-05")
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var got model.SavingsReport
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || got.Rows[0].Saved != 1200 || got.Rows[0].Remaining != 3800 {
		t.Fatalf("unexpected report: %s", body)
	}
	if got.Rows[0].ExcludedAccounts == nil || got.Rows[0].Months == nil {
		t.Errorf("excluded_accounts and months must serialize as []")
	}

	status, body = getJSON(t, ts.URL+"/api/reports/savings")
	if status != http.StatusOK {
		t.Fatalf("default month: %d %s", status, body)
	}
	_ = json.Unmarshal(body, &got)
	if got.Month != time.Now().Format("2006-01") {
		t.Errorf("default month = %q", got.Month)
	}
	// The opening balance of Assets:Savings is dated now, so it falls in the
	// current month; it must not count as saved.
	if len(got.Rows) != 1 || got.Rows[0].Saved != 0 {
		t.Errorf("opening balance counted as saved: %s", body)
	}

	status, body = getJSON(t, ts.URL+"/api/reports/savings?month=2026-13")
	if status != http.StatusBadRequest || decodeErrBody(t, body).Field != "month" {
		t.Errorf("bad month: %d %s", status, body)
	}
}
```

`seedAccount` with a non-zero balance calls `CreateAccountWithBalance`, which records an `Opening` transaction with `Timestamp: time.Now()` (`internal/service/account_ops.go`, `createOpeningBalanceInRepo`). That is why the default-month check expects `Saved == 0`: the target (from 2026-05) is active today and the only split on the account this month is the opening balance.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -run 'Savings' -v`
Expected: FAIL (404 for the new paths).

- [ ] **Step 3: Implement the handlers and routes**

`internal/api/savings.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

type savingsTargetListResponse struct {
	Items []model.SavingsTarget `json:"items"`
}

func (s *Server) handleListSavingsTargets(w http.ResponseWriter, r *http.Request) error {
	items, err := s.svc.Savings().ListSavingsTargets(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, savingsTargetListResponse{Items: items})
}

func (s *Server) handleSetSavingsTarget(w http.ResponseWriter, r *http.Request) error {
	var input model.SetSavingsTargetInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	t, err := s.svc.Savings().SetSavingsTarget(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleStopSavingsTarget(w http.ResponseWriter, r *http.Request) error {
	var input model.StopSavingsTargetInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	t, err := s.svc.Savings().StopSavingsTarget(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteSavingsTarget(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	if err := s.svc.Savings().DeleteSavingsTarget(r.Context(), id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

func (s *Server) handleSavingsReport(w http.ResponseWriter, r *http.Request) error {
	report, err := s.svc.Savings().GenerateSavingsReport(r.Context(), r.URL.Query().Get("month"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, report)
}
```

In `internal/api/router.go`, after the `/budgets/{id}` route:

```go
		r.Method(http.MethodGet, "/reports/savings", apiHandler(s.handleSavingsReport))
		r.Method(http.MethodGet, "/savings-targets", apiHandler(s.handleListSavingsTargets))
		r.Method(http.MethodPut, "/savings-targets", apiHandler(s.handleSetSavingsTarget))
		r.Method(http.MethodPost, "/savings-targets/stop", apiHandler(s.handleStopSavingsTarget))
		r.Method(http.MethodDelete, "/savings-targets/{id}", apiHandler(s.handleDeleteSavingsTarget))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/api/ -run 'Savings' -v` — PASS. Then `go test ./internal/api/` — PASS.

- [ ] **Step 5: Document the endpoints**

In `docs/http-api.md`, after the `### Budgets` section (after its `Errors:` line), add:

```markdown
### Savings targets

| Method | Path | Handler | Service call | Notes |
|---|---|---|---|---|
| GET | `/api/savings-targets` | `savings.go` `handleListSavingsTargets` | `ListSavingsTargets` | `{"items":[SavingsTarget]}`; items never null |
| PUT | `/api/savings-targets` | `savings.go` `handleSetSavingsTarget` | `SetSavingsTarget` | Body `{account_name, effective_month, amount}` (`YYYY-MM`, cents); Asset accounts only; idempotent upsert; returns `SavingsTarget` |
| POST | `/api/savings-targets/stop` | `savings.go` `handleStopSavingsTarget` | `StopSavingsTarget` | Body `{account_name, effective_month}`; returns the stop `SavingsTarget` |
| DELETE | `/api/savings-targets/{id}` | `savings.go` `handleDeleteSavingsTarget` | `DeleteSavingsTarget` | Returns `{"deleted":true,"id":n}` |

Errors: 400 `validation_failed` with `field` one of `account_name`, `effective_month`, `amount`, `body` (unknown JSON field or bad JSON); 404 `not_found` for an unknown account or target id.
```

In the `### Reports` table, after the `/api/reports/budget` row, add:

```markdown
| GET | `/api/reports/savings` | `savings.go` `handleSavingsReport` | `GenerateSavingsReport` | `month` `YYYY-MM`, default current local month; returns `SavingsReport` (fields in `internal/model/savings.go`): per target saved (signed balance change of the account and its descendants, `Opening` transactions excluded), remaining, and year-to-date over active months; totals per currency over rows with no targeted ancestor in the same currency; bad month is 400 with `field` `month` |
```

Run: `scripts/check-docs.sh` — passes.

- [ ] **Step 6: Commit**

```bash
git add internal/api/savings.go internal/api/savings_test.go internal/api/router.go docs/http-api.md
git commit -m "feat(api): add savings target endpoints and report

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: CLI views and JSON DTOs

**Files:**
- Create: `ui/views/savings.go`
- Modify: `ui/views/json_types.go` (append after the budget DTOs)
- Test: `ui/views/savings_test.go`

**Interfaces:**
- Consumes: `model.SavingsTarget`, `model.SavingsReport`; existing `utils.FormatAmount`, `CentsToUnit`, `centsMapToUnitMap`, `sortedKeys`.
- Produces:
  - `NewSavingsReportView(w io.Writer) *SavingsReportView` with `Render(*model.SavingsReport)`
  - `NewSavingsListView(w io.Writer) *SavingsListView` with `Render([]model.SavingsTarget)`
  - `ToJSONSavingsTarget(model.SavingsTarget) JSONSavingsTarget`, `ToJSONSavingsTargets([]model.SavingsTarget) []JSONSavingsTarget`, `ToJSONSavingsReport(*model.SavingsReport) JSONSavingsReport`

- [ ] **Step 1: Write the failing tests**

`ui/views/savings_test.go`:

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

func sampleSavingsReport() *model.SavingsReport {
	return &model.SavingsReport{
		Month: "2026-10",
		Rows: []model.SavingsReportRow{
			{AccountName: "Assets:Savings", Currency: "TWD", Target: 1500000, Saved: 1200000, Remaining: 300000,
				YTDTarget: 15000000, YTDSaved: 15650000, YTDRemaining: -650000,
				Months:           []model.SavingsMonth{{Month: "2026-10", Target: 1500000, Saved: 1200000}},
				ExcludedAccounts: []string{"Assets:Savings:USD"}},
			{AccountName: "Assets:Savings:Travel", Currency: "TWD", Target: 200000, Saved: -50000, Remaining: 250000,
				Months: []model.SavingsMonth{}, ExcludedAccounts: []string{}},
			{AccountName: "Assets:Broker", Currency: "USD", Target: 0, Saved: 0, Remaining: 0,
				Months: []model.SavingsMonth{}, ExcludedAccounts: []string{}},
		},
		TotalTarget:    map[string]int64{"TWD": 1500000, "USD": 0},
		TotalSaved:     map[string]int64{"TWD": 1200000, "USD": 0},
		TotalYTDTarget: map[string]int64{"TWD": 15000000, "USD": 0},
		TotalYTDSaved:  map[string]int64{"TWD": 15650000, "USD": 0},
	}
}

func TestSavingsReportView_Render(t *testing.T) {
	pterm.DisableColor()
	defer pterm.EnableColor()

	var buf bytes.Buffer
	NewSavingsReportView(&buf).Render(sampleSavingsReport())
	out := buf.String()

	assert.Contains(t, out, "Savings report  2026-10")
	var parentLine, childLine string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		switch {
		case strings.HasPrefix(trimmed, "Assets:Savings:Travel"):
			childLine = line
		case strings.HasPrefix(trimmed, "Assets:Savings "):
			parentLine = line
		}
	}
	assert.NotEmpty(t, parentLine)
	assert.NotEmpty(t, childLine)
	indent := func(l string) int { return len(l) - len(strings.TrimLeft(l, " ")) }
	assert.Greater(t, indent(childLine), indent(parentLine), "child row is indented under its targeted parent")
	assert.Contains(t, parentLine, "3,000")
	assert.Contains(t, parentLine, "-6,500")
	assert.Contains(t, childLine, "-500")
	assert.Contains(t, out, "Total (TWD)")
	assert.Contains(t, out, "Total (USD)")
	assert.Contains(t, out, "Assets:Savings: 1 sub-account excluded (different currency): Assets:Savings:USD")
}

func TestSavingsColor(t *testing.T) {
	// Compare against pterm's own functions so the result does not depend on
	// whether another test left color enabled or disabled.
	assert.Equal(t, pterm.Yellow("x"), savingsColor(1)("x"), "still missing")
	assert.Equal(t, pterm.Green("x"), savingsColor(0)("x"), "met")
	assert.Equal(t, pterm.Green("x"), savingsColor(-1)("x"), "exceeded")
}

func TestSavingsReportView_Empty(t *testing.T) {
	var buf bytes.Buffer
	NewSavingsReportView(&buf).Render(&model.SavingsReport{Month: "2026-10", Rows: []model.SavingsReportRow{}})
	assert.Contains(t, buf.String(), "No savings targets in 2026-10")
}

func TestSavingsListView_Render(t *testing.T) {
	pterm.DisableColor()
	defer pterm.EnableColor()

	var buf bytes.Buffer
	NewSavingsListView(&buf).Render([]model.SavingsTarget{
		{ID: 7, AccountName: "Assets:Savings", EffectiveMonth: "2026-01", Amount: 1500000},
		{ID: 9, AccountName: "Assets:Savings", EffectiveMonth: "2026-06", Stopped: true},
	})
	out := buf.String()
	assert.Contains(t, out, "7")
	assert.Contains(t, out, "15,000")
	assert.Contains(t, out, "stopped")

	buf.Reset()
	NewSavingsListView(&buf).Render([]model.SavingsTarget{})
	assert.Contains(t, buf.String(), "No savings targets")
}

func TestToJSONSavingsReport(t *testing.T) {
	j := ToJSONSavingsReport(sampleSavingsReport())
	assert.Equal(t, "2026-10", j.Month)
	assert.InDelta(t, 12000.0, j.Rows[0].Saved, 0.001)
	assert.InDelta(t, -6500.0, j.Rows[0].YTDRemaining, 0.001)
	assert.Equal(t, "2026-10", j.Rows[0].Months[0].Month)
	assert.InDelta(t, 15000.0, j.Rows[0].Months[0].Target, 0.001)
	assert.NotNil(t, j.Rows[1].Months)
	assert.InDelta(t, 156500.0, j.TotalYTDSaved["TWD"], 0.001)
	assert.Equal(t, []string{"Assets:Savings:USD"}, j.Rows[0].ExcludedAccounts)

	empty := ToJSONSavingsReport(&model.SavingsReport{Month: "2026-10"})
	assert.NotNil(t, empty.Rows)
	assert.NotNil(t, empty.TotalTarget)
	assert.NotNil(t, empty.TotalYTDSaved)
}

func TestToJSONSavingsTarget(t *testing.T) {
	j := ToJSONSavingsTarget(model.SavingsTarget{ID: 1, AccountName: "Assets:Savings", EffectiveMonth: "2026-01", Amount: 12345})
	assert.InDelta(t, 123.45, j.Amount, 0.001)
	assert.Equal(t, "Assets:Savings", j.AccountName)
	assert.Len(t, ToJSONSavingsTargets([]model.SavingsTarget{{ID: 1}, {ID: 2}}), 2)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ui/views/ -run 'Savings' -v`
Expected: FAIL to compile.

- [ ] **Step 3: Implement the views**

`ui/views/savings.go`:

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

// savingsColor is yellow while part of the target is still missing and green
// once it is met or exceeded.
func savingsColor(remaining int64) func(...any) string {
	if remaining > 0 {
		return pterm.Yellow
	}
	return pterm.Green
}

// SavingsReportView renders a target vs saved table.
type SavingsReportView struct{ w io.Writer }

func NewSavingsReportView(w io.Writer) *SavingsReportView { return &SavingsReportView{w: w} }

func (v *SavingsReportView) Render(r *model.SavingsReport) {
	if len(r.Rows) == 0 {
		fmt.Fprintf(v.w, "No savings targets in %s. Set one with: kea savings set <account> <amount>\n", r.Month)
		return
	}
	fmt.Fprintf(v.w, "Savings report  %s\n\n", r.Month)

	t := tablewriter.NewWriter(v.w)
	t.SetHeader([]string{"Account", "Target", "Saved", "Remaining", "YTD Target", "YTD Saved", "YTD Remaining", "Currency"})
	t.SetBorder(false)
	t.SetHeaderLine(true)
	t.SetAutoWrapText(false)
	t.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT,
	})

	names := make([]string, len(r.Rows))
	for i, row := range r.Rows {
		names[i] = row.AccountName
	}
	for _, row := range r.Rows {
		t.Append([]string{
			strings.Repeat("  ", ancestorDepth(row.AccountName, names)) + row.AccountName,
			utils.FormatAmount(row.Target),
			utils.FormatAmount(row.Saved),
			savingsColor(row.Remaining)(utils.FormatAmount(row.Remaining)),
			utils.FormatAmount(row.YTDTarget),
			utils.FormatAmount(row.YTDSaved),
			savingsColor(row.YTDRemaining)(utils.FormatAmount(row.YTDRemaining)),
			row.Currency,
		})
	}
	for _, ccy := range sortedKeys(r.TotalTarget) {
		target, saved := r.TotalTarget[ccy], r.TotalSaved[ccy]
		ytdTarget, ytdSaved := r.TotalYTDTarget[ccy], r.TotalYTDSaved[ccy]
		t.Append([]string{
			fmt.Sprintf("Total (%s)", ccy),
			utils.FormatAmount(target),
			utils.FormatAmount(saved),
			savingsColor(target - saved)(utils.FormatAmount(target - saved)),
			utils.FormatAmount(ytdTarget),
			utils.FormatAmount(ytdSaved),
			savingsColor(ytdTarget - ytdSaved)(utils.FormatAmount(ytdTarget - ytdSaved)),
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

// ancestorDepth counts the other names that are ancestors of name.
func ancestorDepth(name string, names []string) int {
	depth := 0
	for _, n := range names {
		if n != name && strings.HasPrefix(name, n+":") {
			depth++
		}
	}
	return depth
}

// SavingsListView renders every savings target version.
type SavingsListView struct{ w io.Writer }

func NewSavingsListView(w io.Writer) *SavingsListView { return &SavingsListView{w: w} }

func (v *SavingsListView) Render(items []model.SavingsTarget) {
	if len(items) == 0 {
		fmt.Fprintln(v.w, "No savings targets. Set one with: kea savings set <account> <amount>")
		return
	}
	t := tablewriter.NewWriter(v.w)
	t.SetHeader([]string{"ID", "Account", "From", "Amount"})
	t.SetBorder(false)
	t.SetHeaderLine(true)
	t.SetAutoWrapText(false)
	t.SetColumnAlignment([]int{tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT})
	for _, s := range items {
		amount := utils.FormatAmount(s.Amount)
		if s.Stopped {
			amount = "stopped"
		}
		t.Append([]string{fmt.Sprintf("%d", s.ID), s.AccountName, s.EffectiveMonth, amount})
	}
	t.Render()
}
```

Append to `ui/views/json_types.go`:

```go
// JSONSavingsTarget is the CLI JSON shape of a savings target version (amount in currency units).
type JSONSavingsTarget struct {
	ID             int64   `json:"id"`
	AccountName    string  `json:"account_name"`
	EffectiveMonth string  `json:"effective_month"`
	Amount         float64 `json:"amount"`
	Stopped        bool    `json:"stopped"`
}

func ToJSONSavingsTarget(s model.SavingsTarget) JSONSavingsTarget {
	return JSONSavingsTarget{
		ID:             s.ID,
		AccountName:    s.AccountName,
		EffectiveMonth: s.EffectiveMonth,
		Amount:         CentsToUnit(s.Amount),
		Stopped:        s.Stopped,
	}
}

func ToJSONSavingsTargets(ss []model.SavingsTarget) []JSONSavingsTarget {
	out := make([]JSONSavingsTarget, len(ss))
	for i, s := range ss {
		out[i] = ToJSONSavingsTarget(s)
	}
	return out
}

// JSONSavingsMonth mirrors model.SavingsMonth in currency units.
type JSONSavingsMonth struct {
	Month  string  `json:"month"`
	Target float64 `json:"target"`
	Saved  float64 `json:"saved"`
}

// JSONSavingsReportRow mirrors model.SavingsReportRow in currency units.
type JSONSavingsReportRow struct {
	AccountName      string             `json:"account_name"`
	Currency         string             `json:"currency"`
	EffectiveMonth   string             `json:"effective_month"`
	Target           float64            `json:"target"`
	Saved            float64            `json:"saved"`
	Remaining        float64            `json:"remaining"`
	YTDTarget        float64            `json:"ytd_target"`
	YTDSaved         float64            `json:"ytd_saved"`
	YTDRemaining     float64            `json:"ytd_remaining"`
	Months           []JSONSavingsMonth `json:"months"`
	ExcludedAccounts []string           `json:"excluded_accounts"`
}

// JSONSavingsReport mirrors model.SavingsReport in currency units.
type JSONSavingsReport struct {
	Month          string                 `json:"month"`
	Rows           []JSONSavingsReportRow `json:"rows"`
	TotalTarget    map[string]float64     `json:"total_target"`
	TotalSaved     map[string]float64     `json:"total_saved"`
	TotalYTDTarget map[string]float64     `json:"total_ytd_target"`
	TotalYTDSaved  map[string]float64     `json:"total_ytd_saved"`
}

func ToJSONSavingsReport(r *model.SavingsReport) JSONSavingsReport {
	rows := make([]JSONSavingsReportRow, len(r.Rows))
	for i, row := range r.Rows {
		months := make([]JSONSavingsMonth, len(row.Months))
		for j, m := range row.Months {
			months[j] = JSONSavingsMonth{Month: m.Month, Target: CentsToUnit(m.Target), Saved: CentsToUnit(m.Saved)}
		}
		excluded := row.ExcludedAccounts
		if excluded == nil {
			excluded = []string{}
		}
		rows[i] = JSONSavingsReportRow{
			AccountName:      row.AccountName,
			Currency:         row.Currency,
			EffectiveMonth:   row.EffectiveMonth,
			Target:           CentsToUnit(row.Target),
			Saved:            CentsToUnit(row.Saved),
			Remaining:        CentsToUnit(row.Remaining),
			YTDTarget:        CentsToUnit(row.YTDTarget),
			YTDSaved:         CentsToUnit(row.YTDSaved),
			YTDRemaining:     CentsToUnit(row.YTDRemaining),
			Months:           months,
			ExcludedAccounts: excluded,
		}
	}
	unitMap := func(m map[string]int64) map[string]float64 {
		if out := centsMapToUnitMap(m); out != nil {
			return out
		}
		return map[string]float64{}
	}
	return JSONSavingsReport{
		Month:          r.Month,
		Rows:           rows,
		TotalTarget:    unitMap(r.TotalTarget),
		TotalSaved:     unitMap(r.TotalSaved),
		TotalYTDTarget: unitMap(r.TotalYTDTarget),
		TotalYTDSaved:  unitMap(r.TotalYTDSaved),
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./ui/views/ -v -run 'Savings'` — PASS. Then `go vet ./ui/... && go test ./ui/...` — PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/views/savings.go ui/views/savings_test.go ui/views/json_types.go
git commit -m "feat(ui): add savings report and list views

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: CLI commands (`kea savings`)

**Files:**
- Create: `cmd/savings/doc.go`, `savings.go`, `set.go`, `stop.go`, `list.go`, `delete.go`, `report.go`
- Test: `cmd/savings/savings_test.go`
- Modify: `cmd/root.go` (import + `AddCommand` after budget)
- Modify: `SKILL.md`, `docs/architecture.md` (package map row)

**Interfaces:**
- Consumes: `svc.Savings()` (Tasks 4–5), `svc.Account().GetAccountsByType`, views from Task 7, `prompts.PromptSelect/PromptAmount/PromptInput/PromptConfirm`, `service.ValidateBudgetMonth`.
- Produces: `savings.NewSavingsCmd(svc *service.Service) *cobra.Command`.

- [ ] **Step 1: Write the failing tests**

`cmd/savings/savings_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package savings

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

type mockSavingsSvc struct {
	lastSet   model.SetSavingsTargetInput
	lastStop  model.StopSavingsTargetInput
	lastMonth string
	deletedID int64
	list      []model.SavingsTarget
	report    *model.SavingsReport
	err       error
}

func (m *mockSavingsSvc) SetSavingsTarget(_ context.Context, in model.SetSavingsTargetInput) (*model.SavingsTarget, error) {
	m.lastSet = in
	if m.err != nil {
		return nil, m.err
	}
	return &model.SavingsTarget{ID: 1, AccountName: in.AccountName, EffectiveMonth: in.EffectiveMonth, Amount: in.Amount}, nil
}

func (m *mockSavingsSvc) StopSavingsTarget(_ context.Context, in model.StopSavingsTargetInput) (*model.SavingsTarget, error) {
	m.lastStop = in
	if m.err != nil {
		return nil, m.err
	}
	return &model.SavingsTarget{ID: 2, AccountName: in.AccountName, EffectiveMonth: in.EffectiveMonth, Stopped: true}, nil
}

func (m *mockSavingsSvc) ListSavingsTargets(_ context.Context) ([]model.SavingsTarget, error) {
	return m.list, m.err
}

func (m *mockSavingsSvc) DeleteSavingsTarget(_ context.Context, id int64) error {
	m.deletedID = id
	return m.err
}

func (m *mockSavingsSvc) GenerateSavingsReport(_ context.Context, month string) (*model.SavingsReport, error) {
	m.lastMonth = month
	if m.err != nil {
		return nil, m.err
	}
	return m.report, nil
}

func TestSetRunner_FlagMode(t *testing.T) {
	m := &mockSavingsSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), []string{"Assets:Savings", "15000.50"}, &setFlags{Month: "2026-01"}))
	assert.Equal(t, model.SetSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-01", Amount: 1500050}, m.lastSet)
}

func TestSetRunner_DefaultsToCurrentMonth(t *testing.T) {
	m := &mockSavingsSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), []string{"Assets:Savings", "0"}, &setFlags{}))
	assert.Equal(t, time.Now().Format("2006-01"), m.lastSet.EffectiveMonth)
	assert.Equal(t, int64(0), m.lastSet.Amount)
}

func TestSetRunner_BadAmountAndMonth(t *testing.T) {
	m := &mockSavingsSvc{}
	r := &setRunner{svc: m, out: &bytes.Buffer{}}
	assert.ErrorContains(t, r.Run(context.Background(), []string{"Assets:Savings", "abc"}, &setFlags{}), "invalid amount")
	assert.Error(t, r.Run(context.Background(), []string{"Assets:Savings", "10"}, &setFlags{Month: "2026-13"}))
	assert.Empty(t, m.lastSet.AccountName, "service must not be called with bad input")
}

func TestSetRunner_PropagatesError(t *testing.T) {
	r := &setRunner{svc: &mockSavingsSvc{err: errors.New("boom")}, out: &bytes.Buffer{}}
	assert.ErrorContains(t, r.Run(context.Background(), []string{"Assets:Savings", "10"}, &setFlags{}), "boom")
}

func TestSetRunner_JSON(t *testing.T) {
	var out bytes.Buffer
	r := &setRunner{svc: &mockSavingsSvc{}, out: &out}
	require.NoError(t, r.Run(context.Background(), []string{"Assets:Savings", "12.5"}, &setFlags{Month: "2026-02", JSON: true}))
	assert.Contains(t, out.String(), `"amount": 12.5`)
	assert.Contains(t, out.String(), `"effective_month": "2026-02"`)
}

func TestStopRunner(t *testing.T) {
	m := &mockSavingsSvc{}
	r := &stopRunner{svc: m, month: "2026-04", out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), "Assets:Savings"))
	assert.Equal(t, model.StopSavingsTargetInput{AccountName: "Assets:Savings", EffectiveMonth: "2026-04"}, m.lastStop)
}

func TestListRunner_FiltersByAccount(t *testing.T) {
	m := &mockSavingsSvc{list: []model.SavingsTarget{
		{ID: 1, AccountName: "Assets:Savings", EffectiveMonth: "2026-01", Amount: 100},
		{ID: 2, AccountName: "Assets:Trip", EffectiveMonth: "2026-01", Amount: 200},
	}}
	var out bytes.Buffer
	r := &listRunner{svc: m, account: "Assets:Trip", json: true, out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Contains(t, out.String(), "Assets:Trip")
	assert.NotContains(t, out.String(), "Assets:Savings")
}

func TestDeleteRunner(t *testing.T) {
	m := &mockSavingsSvc{}
	r := &deleteRunner{svc: m, yes: true, out: &bytes.Buffer{}}
	require.NoError(t, r.Run(context.Background(), "42"))
	assert.Equal(t, int64(42), m.deletedID)

	err := (&deleteRunner{svc: m, yes: true, out: &bytes.Buffer{}}).Run(context.Background(), "x")
	assert.ErrorContains(t, err, "invalid savings target id")
}

func TestReportRunner(t *testing.T) {
	m := &mockSavingsSvc{report: &model.SavingsReport{Month: "2026-03", Rows: []model.SavingsReportRow{}}}
	var out bytes.Buffer
	r := &reportRunner{svc: m, month: "2026-03", out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Equal(t, "2026-03", m.lastMonth)
	assert.Contains(t, out.String(), "No savings targets in 2026-03")
}

func TestReportRunner_JSON(t *testing.T) {
	m := &mockSavingsSvc{report: &model.SavingsReport{
		Month: "2026-03",
		Rows: []model.SavingsReportRow{{AccountName: "Assets:Savings", Currency: "USD", Target: 1000, Saved: 250, Remaining: 750,
			Months: []model.SavingsMonth{{Month: "2026-03", Target: 1000, Saved: 250}}}},
		TotalTarget: map[string]int64{"USD": 1000},
		TotalSaved:  map[string]int64{"USD": 250},
	}}
	var out bytes.Buffer
	r := &reportRunner{svc: m, month: "2026-03", json: true, out: &out}
	require.NoError(t, r.Run(context.Background()))
	assert.Contains(t, out.String(), `"saved": 2.5`)
	assert.Contains(t, out.String(), `"ytd_remaining"`)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/savings/ -v`
Expected: FAIL (package does not exist).

- [ ] **Step 3: Implement the commands**

`cmd/savings/doc.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

// Package savings implements the `kea savings` subcommands: set, stop, list,
// delete and report.
package savings
```

`cmd/savings/savings.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package savings

import (
	"encoding/json"
	"io"
	"time"

	"github.com/hance08/kea/internal/service"
	"github.com/spf13/cobra"
)

func NewSavingsCmd(svc *service.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "savings",
		Aliases: []string{"sv"},
		Short:   "Manage monthly savings targets on Asset accounts",
		Long: `Set, stop, list and delete monthly savings targets on Asset accounts, and compare them with what was saved.
Saved is the account's balance change in the month (descendants included, opening balances excluded).
A target applies from its month until a later version replaces or stops it.`,
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

`cmd/savings/set.go` (Pattern C):

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package savings

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

type SavingsSetProvider interface {
	SetSavingsTarget(ctx context.Context, input model.SetSavingsTargetInput) (*model.SavingsTarget, error)
}

type AssetAccountLister interface {
	GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error)
}

type setFlags struct {
	Month string
	JSON  bool
}

type setRunner struct {
	svc      SavingsSetProvider
	accounts AssetAccountLister
	out      io.Writer
}

func NewSetCmd(svc *service.Service) *cobra.Command {
	flags := &setFlags{}
	cmd := &cobra.Command{
		Use:   "set [<account> <amount>]",
		Short: "Set a monthly savings target on an Asset account",
		Long: `Set the monthly savings target of an Asset account from --month (default: current month) on.
Setting the same account and month again replaces that version. Run without arguments for an interactive prompt.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 && len(args) != 2 {
				return fmt.Errorf("expected <account> <amount>, or no arguments for interactive mode")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &setRunner{svc: svc.Savings(), accounts: svc.Account(), out: os.Stdout}
			return r.Run(cmd.Context(), args, flags)
		},
	}
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "first month the target applies to (YYYY-MM, default: current month)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *setRunner) Run(ctx context.Context, args []string, flags *setFlags) error {
	var input model.SetSavingsTargetInput
	var err error
	if len(args) == 0 {
		input, err = r.promptInput(ctx, flags)
	} else {
		input, err = r.inputFromArgs(args, flags)
	}
	if err != nil {
		return err
	}

	t, err := r.svc.SetSavingsTarget(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to set savings target: %w", err)
	}
	if flags.JSON {
		return writeJSON(r.out, views.ToJSONSavingsTarget(*t))
	}
	pterm.Success.Printf("Savings target for %s set to %s from %s\n", t.AccountName, utils.FormatAmount(t.Amount), t.EffectiveMonth)
	return nil
}

func (r *setRunner) inputFromArgs(args []string, flags *setFlags) (model.SetSavingsTargetInput, error) {
	amount, err := utils.ParseAmount(args[1])
	if err != nil {
		return model.SetSavingsTargetInput{}, fmt.Errorf("invalid amount %q: %w", args[1], err)
	}
	month := monthOrCurrent(flags.Month)
	if err := service.ValidateBudgetMonth(month); err != nil {
		return model.SetSavingsTargetInput{}, err
	}
	return model.SetSavingsTargetInput{AccountName: args[0], EffectiveMonth: month, Amount: amount}, nil
}

func (r *setRunner) promptInput(ctx context.Context, flags *setFlags) (model.SetSavingsTargetInput, error) {
	accs, err := r.accounts.GetAccountsByType(ctx, model.AccountTypeAsset)
	if err != nil {
		return model.SetSavingsTargetInput{}, fmt.Errorf("failed to load Asset accounts: %w", err)
	}
	names := make([]string, 0, len(accs))
	for _, a := range accs {
		if !a.IsHidden {
			names = append(names, a.Name)
		}
	}
	if len(names) == 0 {
		return model.SetSavingsTargetInput{}, fmt.Errorf("no Asset accounts; create one with `kea account create`")
	}
	sort.Strings(names)

	account, err := prompts.PromptSelect("Savings account", names, names[0])
	if err != nil {
		return model.SetSavingsTargetInput{}, err
	}
	amountStr, err := prompts.PromptAmount("Monthly savings target", "e.g. 15000 or 15000.50", func(s string) error {
		_, err := utils.ParseAmount(s)
		return err
	})
	if err != nil {
		return model.SetSavingsTargetInput{}, err
	}
	month, err := prompts.PromptInput("Effective from (YYYY-MM)", monthOrCurrent(flags.Month), service.ValidateBudgetMonth)
	if err != nil {
		return model.SetSavingsTargetInput{}, err
	}
	amount, err := utils.ParseAmount(amountStr)
	if err != nil {
		return model.SetSavingsTargetInput{}, fmt.Errorf("invalid amount %q: %w", amountStr, err)
	}
	return model.SetSavingsTargetInput{AccountName: account, EffectiveMonth: month, Amount: amount}, nil
}
```

`cmd/savings/stop.go` (Pattern B):

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package savings

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

type SavingsStopProvider interface {
	StopSavingsTarget(ctx context.Context, input model.StopSavingsTargetInput) (*model.SavingsTarget, error)
}

type stopFlags struct {
	Month string
	JSON  bool
}

type stopRunner struct {
	svc   SavingsStopProvider
	month string
	json  bool
	out   io.Writer
}

func NewStopCmd(svc *service.Service) *cobra.Command {
	flags := &stopFlags{}
	cmd := &cobra.Command{
		Use:   "stop <account>",
		Short: "Stop an account's savings target from a month on",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &stopRunner{svc: svc.Savings(), month: monthOrCurrent(flags.Month), json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context(), args[0])
		},
	}
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "first month without a target (YYYY-MM, default: current month)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *stopRunner) Run(ctx context.Context, account string) error {
	t, err := r.svc.StopSavingsTarget(ctx, model.StopSavingsTargetInput{AccountName: account, EffectiveMonth: r.month})
	if err != nil {
		return fmt.Errorf("failed to stop savings target: %w", err)
	}
	if r.json {
		return writeJSON(r.out, views.ToJSONSavingsTarget(*t))
	}
	pterm.Success.Printf("Savings target for %s stopped from %s\n", t.AccountName, t.EffectiveMonth)
	return nil
}
```

`cmd/savings/list.go` (Pattern B):

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package savings

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

type SavingsListProvider interface {
	ListSavingsTargets(ctx context.Context) ([]model.SavingsTarget, error)
}

type listFlags struct {
	Account string
	JSON    bool
}

type listRunner struct {
	svc     SavingsListProvider
	account string
	json    bool
	out     io.Writer
}

func NewListCmd(svc *service.Service) *cobra.Command {
	flags := &listFlags{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List every savings target version",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &listRunner{svc: svc.Savings(), account: flags.Account, json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&flags.Account, "account", "", "only show versions of this account")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *listRunner) Run(ctx context.Context) error {
	items, err := r.svc.ListSavingsTargets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list savings targets: %w", err)
	}
	if r.account != "" {
		filtered := make([]model.SavingsTarget, 0, len(items))
		for _, t := range items {
			if t.AccountName == r.account {
				filtered = append(filtered, t)
			}
		}
		items = filtered
	}
	if r.json {
		return writeJSON(r.out, views.ToJSONSavingsTargets(items))
	}
	views.NewSavingsListView(r.out).Render(items)
	return nil
}
```

`cmd/savings/delete.go` (Pattern B):

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package savings

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

type SavingsDeleteProvider interface {
	DeleteSavingsTarget(ctx context.Context, id int64) error
}

type deleteFlags struct {
	Yes  bool
	JSON bool
}

type deleteRunner struct {
	svc  SavingsDeleteProvider
	yes  bool
	json bool
	out  io.Writer
}

func NewDeleteCmd(svc *service.Service) *cobra.Command {
	flags := &deleteFlags{}
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"del"},
		Short:   "Delete one savings target version (see `kea savings list` for IDs)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &deleteRunner{svc: svc.Savings(), yes: flags.Yes || flags.JSON, json: flags.JSON, out: os.Stdout}
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
		return fmt.Errorf("invalid savings target id %q", rawID)
	}
	if !r.yes {
		ok, err := prompts.PromptConfirm(fmt.Sprintf("Delete savings target version #%d?", id), false)
		if err != nil {
			return err
		}
		if !ok {
			pterm.Info.Println("Deletion cancelled")
			return nil
		}
	}
	if err := r.svc.DeleteSavingsTarget(ctx, id); err != nil {
		return fmt.Errorf("failed to delete savings target: %w", err)
	}
	if r.json {
		return writeJSON(r.out, map[string]any{"id": id, "deleted": true})
	}
	pterm.Success.Printf("Savings target version #%d deleted\n", id)
	return nil
}
```

`cmd/savings/report.go` (Pattern B):

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package savings

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

type SavingsReportProvider interface {
	GenerateSavingsReport(ctx context.Context, month string) (*model.SavingsReport, error)
}

type reportFlags struct {
	Month string
	JSON  bool
}

type reportRunner struct {
	svc   SavingsReportProvider
	month string
	json  bool
	out   io.Writer
}

func NewReportCmd(svc *service.Service) *cobra.Command {
	flags := &reportFlags{}
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show savings target vs saved for a month and year to date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &reportRunner{svc: svc.Savings(), month: flags.Month, json: flags.JSON, out: os.Stdout}
			return r.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "month to report on (YYYY-MM, default: current month)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output report as JSON")
	return cmd
}

func (r *reportRunner) Run(ctx context.Context) error {
	report, err := r.svc.GenerateSavingsReport(ctx, r.month)
	if err != nil {
		return fmt.Errorf("failed to generate savings report: %w", err)
	}
	if r.json {
		return writeJSON(r.out, views.ToJSONSavingsReport(report))
	}
	views.NewSavingsReportView(r.out).Render(report)
	return nil
}
```

In `cmd/root.go`, add the import `savingscmd "github.com/hance08/kea/cmd/savings"` next to `budgetcmd`, and after `rootCmd.AddCommand(budgetcmd.NewBudgetCmd(application.Service))`:

```go
		rootCmd.AddCommand(savingscmd.NewSavingsCmd(application.Service))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/... -v -run 'Runner'` — PASS. Then `go build ./... && go vet ./... && go test ./...` — PASS.

Smoke test against a throwaway ledger (never the user's real ledger). `cmd/root.go` opens the ledger before registering subcommands, so even `--help` needs a ledger. Check how `kea` picks its config/ledger (`docs/development.md`) and point it at a temp directory; if startup still needs an initialized ledger there, create one in that temp directory first. For example, if `HOME` decides the config location:

```bash
make build
HOME=$(mktemp -d) ./kea savings --help
```

Expected: help lists `set`, `stop`, `list`, `delete`, `report`.

- [ ] **Step 5: Update docs**

In `SKILL.md`, after the `### Budgets` section's `Rules:` list and its `---` separator, add:

````markdown
### Savings targets

```bash
kea savings set <account> <amount> [--month YYYY-MM] [--json]
kea savings stop <account> [--month YYYY-MM] [--json]
kea savings list [--account <name>] [--json]
kea savings delete <id> [--yes] [--json]
kea savings report [--month YYYY-MM] [--json]
```

Examples:
```bash
# Save 15000 per month into the savings account, starting October 2026
kea savings set Assets:Banks:Cube_Saving 15000 --month 2026-10

# Stop that target from March 2027 on
kea savings stop Assets:Banks:Cube_Saving --month 2027-03

# List all target versions (IDs are used by delete)
kea savings list --account Assets:Banks:Cube_Saving

# Delete one target version
kea savings delete 3 --yes

# Target vs saved for October 2026 and year to date, as JSON
kea savings report --month 2026-10 --json
```

Rules:
- Savings targets can only be set on Asset accounts.
- Saved is the account's balance change in the month, descendants in the same currency included; opening balances do not count. Withdrawals make it smaller, possibly negative.
- A target version applies from its month on, until a later version replaces or stops it.
- Each month stands alone (no rollover); the report also shows year-to-date totals over the months the target was active.

---
````

In the SKILL.md recipe block that ends with `kea budget report --json`, add:

```bash
# Savings target vs saved this month
kea savings report --json
```

In `docs/architecture.md` package map, after the `cmd/budget` row:

```markdown
| `cmd/savings` | `kea savings` subcommands (set, stop, list, delete, report) | `internal/app`, `internal/store`, `internal/api` |
```

Run: `scripts/check-docs.sh` — passes.

- [ ] **Step 6: Commit**

```bash
git add cmd/savings cmd/root.go SKILL.md docs/architecture.md
git commit -m "feat(cli): add kea savings commands

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: SPA data layer

**Files:**
- Modify: `spa/src/lib/types.ts` (after the budget types)
- Create: `spa/src/lib/api/savings.ts`
- Create: `spa/src/lib/hooks/useSavings.ts`
- Create: `spa/src/lib/savings.ts`
- Modify: `spa/src/lib/budgets.ts` (generic `activeVersions`)
- Test: `spa/src/test/api.savings.test.ts`, `spa/src/test/lib.savings.test.ts`

**Interfaces:**
- Produces:
  - Types `SavingsTarget`, `SavingsTargetListResponse`, `SetSavingsTargetInput`, `StopSavingsTargetInput`, `SavingsMonth`, `SavingsReportRow`, `SavingsReport`
  - `fetchSavingsTargets()`, `setSavingsTarget(input)`, `stopSavingsTarget(input)`, `deleteSavingsTarget(id)`, `fetchSavingsReport(month?)`
  - `useSavingsTargets()` (query key `['savings','list']`), `useSavingsReport(month?)` (key `['savings','report', month ?? 'current']`)
  - `type SavingsStatus = 'achieved' | 'on-track' | 'behind' | 'negative'`; `savingsStatus(target, saved, elapsed: number | null)`; `remainingLabel(remaining, format: (cents: number) => string)`
  - In `budgets.ts`: `interface VersionedRow`, `activeVersions<T extends VersionedRow>(items: T[], month: string): T[]`; `activeBudgets` keeps its signature.

- [ ] **Step 1: Write the failing tests**

`spa/src/test/api.savings.test.ts`:

```ts
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import {
  deleteSavingsTarget,
  fetchSavingsReport,
  fetchSavingsTargets,
  setSavingsTarget,
  stopSavingsTarget,
} from '../lib/api/savings';

let fetchSpy: ReturnType<typeof vi.fn>;
const ok = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  fetchSpy = vi.fn(() => Promise.resolve(ok({})));
  vi.stubGlobal('fetch', fetchSpy);
});
afterEach(() => vi.unstubAllGlobals());

test('fetchSavingsTargets GETs the list', async () => {
  await fetchSavingsTargets();
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/savings-targets');
});

test('setSavingsTarget PUTs JSON', async () => {
  await setSavingsTarget({ account_name: 'Assets:Savings', effective_month: '2026-01', amount: 100 });
  const [url, init] = fetchSpy.mock.calls[0];
  expect(url).toBe('/api/savings-targets');
  expect(init.method).toBe('PUT');
  expect(JSON.parse(init.body)).toEqual({
    account_name: 'Assets:Savings',
    effective_month: '2026-01',
    amount: 100,
  });
});

test('stopSavingsTarget POSTs to /stop', async () => {
  await stopSavingsTarget({ account_name: 'Assets:Savings', effective_month: '2026-02' });
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/savings-targets/stop');
  expect(fetchSpy.mock.calls[0][1].method).toBe('POST');
});

test('deleteSavingsTarget DELETEs by id', async () => {
  await deleteSavingsTarget(7);
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/savings-targets/7');
  expect(fetchSpy.mock.calls[0][1].method).toBe('DELETE');
});

test('fetchSavingsReport omits month when undefined', async () => {
  await fetchSavingsReport();
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/reports/savings');
  await fetchSavingsReport('2026-03');
  expect(fetchSpy.mock.calls[1][0]).toBe('/api/reports/savings?month=2026-03');
});
```

`spa/src/test/lib.savings.test.ts`:

```ts
import { expect, test } from 'vitest';
import { activeBudgets, activeVersions } from '../lib/budgets';
import { remainingLabel, savingsStatus } from '../lib/savings';
import type { Budget, SavingsTarget } from '../lib/types';

test('savingsStatus', () => {
  expect(savingsStatus(1000, 1000, null)).toBe('achieved');
  expect(savingsStatus(1000, 1500, 0.2)).toBe('achieved');
  expect(savingsStatus(1000, -1, 0.9)).toBe('negative');
  expect(savingsStatus(0, 0, null)).toBe('achieved');
  expect(savingsStatus(0, -500, null)).toBe('negative');
  // Current month: compare with the elapsed fraction.
  expect(savingsStatus(1000, 600, 0.5)).toBe('on-track');
  expect(savingsStatus(1000, 400, 0.5)).toBe('behind');
  expect(savingsStatus(1000, 0, 0)).toBe('on-track');
  // A month that is not the current one is complete: anything short is behind.
  expect(savingsStatus(1000, 999, null)).toBe('behind');
});

test('remainingLabel', () => {
  const fmt = (c: number) => (c / 100).toFixed(2);
  expect(remainingLabel(300000, fmt)).toBe('3000.00 to go');
  expect(remainingLabel(-650000, fmt)).toBe('6500.00 over target');
  expect(remainingLabel(0, fmt)).toBe('Target met');
});

test('activeVersions works for savings targets and keeps activeBudgets behavior', () => {
  const targets: SavingsTarget[] = [
    { id: 1, account_id: 1, account_name: 'Assets:Savings', effective_month: '2026-01', amount: 100, stopped: false },
    { id: 2, account_id: 1, account_name: 'Assets:Savings', effective_month: '2026-03', amount: 0, stopped: true },
    { id: 3, account_id: 2, account_name: 'Assets:Emergency', effective_month: '2026-02', amount: 50, stopped: false },
  ];
  expect(activeVersions(targets, '2026-02').map((t) => t.id)).toEqual([3, 1]);
  expect(activeVersions(targets, '2026-04').map((t) => t.id)).toEqual([3]);

  const budgets: Budget[] = [
    { id: 9, account_id: 5, account_name: 'Expenses:Food', effective_month: '2026-01', amount: 1, stopped: false },
  ];
  expect(activeBudgets(budgets, '2026-01').map((b) => b.id)).toEqual([9]);
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd spa && npx vitest run src/test/api.savings.test.ts src/test/lib.savings.test.ts`
Expected: FAIL (modules not found).

- [ ] **Step 3: Implement**

Append to `spa/src/lib/types.ts` (after `BudgetReport`):

```ts
export interface SavingsTarget {
  id: number;
  account_id: number;
  account_name: string;
  effective_month: string;
  amount: number;
  stopped: boolean;
}

export interface SavingsTargetListResponse {
  items: SavingsTarget[];
}

export interface SetSavingsTargetInput {
  account_name: string;
  effective_month: string;
  amount: number;
}

export interface StopSavingsTargetInput {
  account_name: string;
  effective_month: string;
}

export interface SavingsMonth {
  month: string;
  target: number;
  saved: number;
}

export interface SavingsReportRow {
  account_id: number;
  account_name: string;
  currency: string;
  effective_month: string;
  target: number;
  saved: number;
  remaining: number;
  ytd_target: number;
  ytd_saved: number;
  ytd_remaining: number;
  months: SavingsMonth[];
  excluded_accounts: string[];
}

export interface SavingsReport {
  month: string;
  rows: SavingsReportRow[];
  total_target: Record<string, number>;
  total_saved: Record<string, number>;
  total_ytd_target: Record<string, number>;
  total_ytd_saved: Record<string, number>;
}
```

`spa/src/lib/api/savings.ts`:

```ts
import { apiFetch } from '../api';
import type {
  SavingsReport,
  SavingsTarget,
  SavingsTargetListResponse,
  SetSavingsTargetInput,
  StopSavingsTargetInput,
} from '../types';

const jsonInit = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
});

export function fetchSavingsTargets(): Promise<SavingsTargetListResponse> {
  return apiFetch<SavingsTargetListResponse>('/api/savings-targets');
}

export function setSavingsTarget(input: SetSavingsTargetInput): Promise<SavingsTarget> {
  return apiFetch<SavingsTarget>('/api/savings-targets', jsonInit('PUT', input));
}

export function stopSavingsTarget(input: StopSavingsTargetInput): Promise<SavingsTarget> {
  return apiFetch<SavingsTarget>('/api/savings-targets/stop', jsonInit('POST', input));
}

export function deleteSavingsTarget(id: number): Promise<{ deleted: boolean; id: number }> {
  return apiFetch<{ deleted: boolean; id: number }>(`/api/savings-targets/${id}`, {
    method: 'DELETE',
  });
}

// Omitting month lets the server use its current local month.
export function fetchSavingsReport(month?: string): Promise<SavingsReport> {
  const q = month ? `?month=${encodeURIComponent(month)}` : '';
  return apiFetch<SavingsReport>(`/api/reports/savings${q}`);
}
```

`spa/src/lib/hooks/useSavings.ts`:

```ts
import { useQuery } from '@tanstack/react-query';
import { fetchSavingsReport, fetchSavingsTargets } from '../api/savings';

export function useSavingsTargets() {
  return useQuery({ queryKey: ['savings', 'list'], queryFn: fetchSavingsTargets });
}

export function useSavingsReport(month?: string) {
  return useQuery({
    queryKey: ['savings', 'report', month ?? 'current'],
    queryFn: () => fetchSavingsReport(month),
  });
}
```

`spa/src/lib/savings.ts`:

```ts
export type SavingsStatus = 'achieved' | 'on-track' | 'behind' | 'negative';

/**
 * Saving is on track when the saved share of the target keeps up with the
 * elapsed share of the month. `elapsed` is null for any month other than the
 * current one, which is treated as complete.
 */
export function savingsStatus(
  target: number,
  saved: number,
  elapsed: number | null,
): SavingsStatus {
  if (saved < 0) return 'negative';
  if (saved >= target) return 'achieved';
  const expected = elapsed ?? 1;
  return saved / target < expected ? 'behind' : 'on-track';
}

/** "X to go" while short, "X over target" when exceeded, "Target met" at exactly zero. */
export function remainingLabel(remaining: number, format: (cents: number) => string): string {
  if (remaining > 0) return `${format(remaining)} to go`;
  if (remaining < 0) return `${format(-remaining)} over target`;
  return 'Target met';
}
```

In `spa/src/lib/budgets.ts`, replace `activeBudgets` with:

```ts
export interface VersionedRow {
  account_id: number;
  account_name: string;
  effective_month: string;
  stopped: boolean;
}

/** Mirrors the server: latest version with effective_month <= month per account, minus stopped ones. */
export function activeVersions<T extends VersionedRow>(items: T[], month: string): T[] {
  const latest = new Map<number, T>();
  for (const v of items) {
    if (v.effective_month > month) continue;
    const cur = latest.get(v.account_id);
    if (!cur || v.effective_month > cur.effective_month) latest.set(v.account_id, v);
  }
  return [...latest.values()]
    .filter((v) => !v.stopped)
    .sort((a, b) => a.account_name.localeCompare(b.account_name));
}

export function activeBudgets(items: Budget[], month: string): Budget[] {
  return activeVersions(items, month);
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npx vitest run src/test/api.savings.test.ts src/test/lib.savings.test.ts src/test/budgets.page.test.tsx` — PASS.
Run: `cd spa && npx tsc -b && npx biome check src` — clean (fix formatting with `npx biome check --write src` if needed).

- [ ] **Step 5: Commit**

```bash
git add spa/src/lib/types.ts spa/src/lib/api/savings.ts spa/src/lib/hooks/useSavings.ts spa/src/lib/savings.ts spa/src/lib/budgets.ts spa/src/test/api.savings.test.ts spa/src/test/lib.savings.test.ts
git commit -m "feat(spa): add savings data layer

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: SPA progress bar and savings report tab

**Files:**
- Create: `spa/src/components/savings/SavingsProgressBar.tsx`
- Create: `spa/src/routes/reports.savings.tsx`
- Modify: `spa/src/components/reports/TabNav.tsx` (add tab after Budget)
- Modify: `spa/src/lib/filter-memory.ts` (`PageId` gains `'reports/savings'`)
- Regenerate: `spa/src/routeTree.gen.ts`
- Test: `spa/src/test/reports.savings.test.tsx`

**Interfaces:**
- Consumes: Task 9 (`useSavingsReport`, `savingsStatus`, `remainingLabel`), `elapsedFraction`, `depthIn` from `lib/budgets`, `MonthPicker`, `makeFilterMemoryLoader`, `parseMonthSearch`, `useAmountFormat`.
- Produces: `SavingsProgressBar({ target, saved, elapsed?, compact? })` rendering `data-testid="savings-bar"` with `data-status`; route `/reports/savings`.

- [ ] **Step 1: Write the failing test**

`spa/src/test/reports.savings.test.tsx`:

```tsx
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { currentMonth } from '../lib/budgets';
import type { SavingsReport, SavingsReportRow } from '../lib/types';
import { makeTestApp } from './test-app';

const ok = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

function row(name: string, target: number, saved: number, extra: Partial<SavingsReportRow> = {}) {
  return {
    account_id: name.length,
    account_name: name,
    currency: 'USD',
    effective_month: '2026-01',
    target,
    saved,
    remaining: target - saved,
    ytd_target: target * 3,
    ytd_saved: saved * 3,
    ytd_remaining: (target - saved) * 3,
    months: [],
    excluded_accounts: [],
    ...extra,
  };
}

let report: SavingsReport;
let reportUrls: string[];

beforeEach(() => {
  localStorage.clear();
  reportUrls = [];
  report = {
    month: '2020-03',
    rows: [
      row('Assets:Savings', 1500000, 1200000, {
        excluded_accounts: ['Assets:Savings:Japan'],
        months: [
          { month: '2020-01', target: 1500000, saved: 1600000 },
          { month: '2020-02', target: 1500000, saved: 1000000 },
          { month: '2020-03', target: 1500000, saved: 1200000 },
        ],
      }),
      row('Assets:Savings:Travel', 200000, 250000),
      row('Assets:Emergency', 100000, -30000),
    ],
    total_target: { USD: 1600000 },
    total_saved: { USD: 1170000 },
    total_ytd_target: { USD: 4800000 },
    total_ytd_saved: { USD: 3510000 },
  };
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config')
        return Promise.resolve(
          ok({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      if (url === '/api/ledgers')
        return Promise.resolve(
          ok({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      if (url.startsWith('/api/reports/savings')) {
        reportUrls.push(url);
        return Promise.resolve(ok(report));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('renders statuses, remaining wording, ytd and month breakdown', async () => {
  render(makeTestApp('/reports/savings?month=2020-03'));
  await waitFor(() => expect(screen.getAllByTestId('savings-bar').length).toBe(3));
  expect(reportUrls).toContain('/api/reports/savings?month=2020-03');

  // Sorted as the server returns them; a past month counts as complete.
  const bars = screen.getAllByTestId('savings-bar');
  expect(bars.map((b) => b.getAttribute('data-status'))).toEqual(['behind', 'achieved', 'negative']);

  // "Savings" also appears in the report tabs and the sidebar, so address rows by position.
  const [savings, travel, emergency] = screen.getAllByTestId('savings-row');
  expect(within(savings).getByText('Savings')).toBeInTheDocument();
  expect(within(savings).getByTestId('savings-remaining')).toHaveTextContent(/3,000\.00 to go/);
  expect(within(savings).getByTestId('savings-ytd')).toHaveTextContent(/9,000\.00 to go/);
  const months = within(savings).getAllByTestId('savings-month');
  expect(months.map((m) => m.getAttribute('data-met'))).toEqual(['true', 'false', 'false']);

  expect(within(travel).getByText('Savings:Travel')).toBeInTheDocument();
  expect(travel.getAttribute('data-depth')).toBe('1');
  expect(within(travel).getByTestId('savings-remaining')).toHaveTextContent(/500\.00 over target/);

  expect(within(emergency).getByTestId('savings-remaining')).toHaveTextContent(/1,300\.00 to go/);

  expect(screen.getByText('Total (USD)')).toBeInTheDocument();
});

test('excluded accounts produce a warning with their names', async () => {
  render(makeTestApp('/reports/savings?month=2020-03'));
  const warn = await screen.findByTestId('excluded-warning');
  expect(warn.getAttribute('title')).toContain('Assets:Savings:Japan');
});

test('current month omits the month param and shows the time marker', async () => {
  report = { ...report, month: currentMonth() };
  render(makeTestApp('/reports/savings'));
  await waitFor(() => expect(screen.getAllByTestId('savings-bar').length).toBe(3));
  expect(reportUrls[0]).toBe('/api/reports/savings');
  expect(screen.getAllByTestId('time-marker').length).toBe(3);
});

test('empty month links to the savings page', async () => {
  report = {
    month: currentMonth(),
    rows: [],
    total_target: {},
    total_saved: {},
    total_ytd_target: {},
    total_ytd_saved: {},
  };
  render(makeTestApp('/reports/savings'));
  const link = await screen.findByRole('link', { name: /set up savings targets/i });
  expect(link.getAttribute('href')).toBe('/savings');
});

test('previous-month button navigates with month param', async () => {
  report = { ...report, month: currentMonth() };
  render(makeTestApp('/reports/savings'));
  await waitFor(() => expect(screen.getAllByTestId('savings-bar').length).toBe(3));
  await userEvent.click(screen.getByRole('button', { name: /previous month/i }));
  await waitFor(() => expect(reportUrls.some((u) => u.includes('month='))).toBe(true));
});

test('report tabs include Savings', async () => {
  render(makeTestApp('/reports/savings?month=2020-03'));
  const tabs = await screen.findByRole('navigation', { name: 'Report types' });
  expect(within(tabs).getByRole('link', { name: 'Savings' }).getAttribute('href')).toBe(
    '/reports/savings',
  );
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npx vitest run src/test/reports.savings.test.tsx`
Expected: FAIL (route not found / no bars).

- [ ] **Step 3: Implement**

`spa/src/components/savings/SavingsProgressBar.tsx`:

```tsx
import { cn } from '@/lib/cn';
import { savingsStatus } from '@/lib/savings';

interface Props {
  target: number;
  saved: number;
  elapsed?: number | null; // 0..1 for the current month; null/undefined = month complete
  compact?: boolean;
}

const FILL = {
  achieved: 'bg-emerald-600',
  'on-track': 'bg-primary',
  behind: 'bg-amber-500',
  negative: 'bg-red-600',
} as const;

export function SavingsProgressBar({ target, saved, elapsed, compact }: Props) {
  const status = savingsStatus(target, saved, elapsed ?? null);
  const ratio = target === 0 ? (saved >= 0 ? 1 : 0) : Math.max(0, saved / target);
  return (
    <div
      data-testid="savings-bar"
      data-status={status}
      className={cn('relative w-full rounded bg-muted', compact ? 'h-1.5' : 'h-2.5')}
    >
      <div
        className={cn('h-full rounded', FILL[status])}
        style={{ width: `${Math.min(ratio, 1) * 100}%` }}
      />
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

`spa/src/routes/reports.savings.tsx`:

```tsx
import { MonthPicker } from '@/components/budgets/MonthPicker';
import { SavingsProgressBar } from '@/components/savings/SavingsProgressBar';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { depthIn, elapsedFraction } from '@/lib/budgets';
import { makeFilterMemoryLoader } from '@/lib/filter-memory';
import { useSavingsReport } from '@/lib/hooks/useSavings';
import { type MonthSearchParams, parseMonthSearch } from '@/lib/reports-search-params';
import { remainingLabel } from '@/lib/savings';
import { useAmountFormat } from '@/lib/server-config';
import { cn } from '@/lib/cn';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/reports/savings')({
  validateSearch: (s): MonthSearchParams => parseMonthSearch(s),
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<MonthSearchParams>({
    pageId: 'reports/savings',
    defaults: {},
    redirectTo: '/reports/savings',
  }),
  component: SavingsReportPage,
});

const shortName = (name: string) => name.split(':').slice(1).join(':') || name;

function SavingsReportPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/reports/savings' });
  const query = useSavingsReport(search.month);
  const { formatCents } = useAmountFormat();
  const setMonth = (month: string | undefined) =>
    navigate({ search: () => (month ? { month } : {}) });

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
          <AlertTitle>Failed to load savings report</AlertTitle>
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
    <div className="space-y-4" data-testid="savings-report">
      {picker}
      {report.rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No savings targets in {report.month}.{' '}
          <Link to="/savings" className="underline">
            Set up savings targets
          </Link>
        </p>
      ) : (
        <div className="space-y-4">
          {report.rows.map((r) => {
            const depth = depthIn(r.account_name, names);
            const fmt = (c: number) => formatCents(c, r.currency);
            return (
              <div
                key={r.account_id}
                data-testid="savings-row"
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
                    {fmt(r.saved)} / {fmt(r.target)}
                  </span>
                </div>
                <SavingsProgressBar target={r.target} saved={r.saved} elapsed={elapsed} />
                <div className="flex flex-wrap justify-between gap-2 text-xs text-muted-foreground">
                  <span data-testid="savings-remaining">{remainingLabel(r.remaining, fmt)}</span>
                  <span data-testid="savings-ytd">
                    YTD {fmt(r.ytd_saved)} / {fmt(r.ytd_target)} · {remainingLabel(r.ytd_remaining, fmt)}
                  </span>
                </div>
                {r.months.length > 0 && (
                  <ul className="grid grid-cols-3 gap-x-4 gap-y-0.5 text-xs sm:grid-cols-6">
                    {r.months.map((m) => {
                      const met = m.saved >= m.target;
                      return (
                        <li
                          key={m.month}
                          data-testid="savings-month"
                          data-met={String(met)}
                          className="flex justify-between gap-2 tabular-nums"
                        >
                          <span className="text-muted-foreground">{m.month.slice(5)}</span>
                          <span className={cn(met ? 'text-emerald-600' : 'text-amber-600')}>
                            {fmt(m.saved)}
                          </span>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </div>
            );
          })}
          <div className="border-t pt-3 text-sm">
            {Object.keys(report.total_target)
              .sort()
              .map((ccy) => {
                const fmt = (c: number) => formatCents(c, ccy);
                const target = report.total_target[ccy];
                const saved = report.total_saved[ccy] ?? 0;
                const ytdTarget = report.total_ytd_target[ccy] ?? 0;
                const ytdSaved = report.total_ytd_saved[ccy] ?? 0;
                return (
                  <div key={ccy} className="flex flex-wrap justify-between gap-2">
                    <span className="font-medium">Total ({ccy})</span>
                    <span className="tabular-nums">
                      {fmt(saved)} / {fmt(target)} · {remainingLabel(target - saved, fmt)} · YTD{' '}
                      {fmt(ytdSaved)} / {fmt(ytdTarget)}
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

`spa/src/components/reports/TabNav.tsx`: after the Budget tab add

```ts
  { to: '/reports/savings', label: 'Savings' },
```

`spa/src/lib/filter-memory.ts`: extend `PageId` with `| 'reports/savings'`.

- [ ] **Step 4: Regenerate the route tree**

`npm run build` runs `tsc` before Vite, and `tsc` fails until the new route is in `routeTree.gen.ts`, so generate it with Vite first:

```bash
cd spa && npx vite build
git -C .. restore internal/web/dist/index.html
```

Confirm `spa/src/routeTree.gen.ts` now contains `/reports/savings`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd spa && npx vitest run src/test/reports.savings.test.tsx src/test/reports.budget.test.tsx` — PASS.
Run: `cd spa && npx tsc -b && npx biome check src` — clean.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/savings/SavingsProgressBar.tsx spa/src/routes/reports.savings.tsx spa/src/components/reports/TabNav.tsx spa/src/lib/filter-memory.ts spa/src/routeTree.gen.ts spa/src/test/reports.savings.test.tsx
git commit -m "feat(spa): add savings report tab

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: SPA savings settings page

**Files:**
- Create: `spa/src/components/savings/SavingsForm.tsx`
- Create: `spa/src/routes/savings.tsx`
- Modify: `spa/src/components/Sidebar.tsx` (nav item after Budgets)
- Regenerate: `spa/src/routeTree.gen.ts`
- Test: `spa/src/test/savings.page.test.tsx`

**Interfaces:**
- Consumes: Task 9 (`useSavingsTargets`, `setSavingsTarget`, `stopSavingsTarget`, `deleteSavingsTarget`, `activeVersions`), `parseCents`, `depthIn`, `currentMonth`, `MonthPicker`, `AccountCombobox`, `listAccounts`.
- Produces: route `/savings`; `SavingsForm({ initial?, lockAccount?, defaultMonth, onDone })`.

- [ ] **Step 1: Write the failing test**

`spa/src/test/savings.page.test.tsx`:

```tsx
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import type { SavingsTarget } from '../lib/types';
import { makeTestApp } from './test-app';

const ok = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

let items: SavingsTarget[];
let calls: { url: string; method: string; body?: unknown }[];
let putResponse: Response | null;

beforeEach(() => {
  calls = [];
  putResponse = null;
  items = [
    { id: 1, account_id: 1, account_name: 'Assets:Savings', effective_month: '2020-01', amount: 1500000, stopped: false },
    { id: 2, account_id: 1, account_name: 'Assets:Savings', effective_month: '2020-06', amount: 2000000, stopped: false },
    { id: 3, account_id: 2, account_name: 'Assets:Trip', effective_month: '2020-01', amount: 5000, stopped: false },
    { id: 4, account_id: 2, account_name: 'Assets:Trip', effective_month: '2020-03', amount: 0, stopped: true },
  ];
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      const method = init?.method ?? 'GET';
      calls.push({ url, method, body: init?.body ? JSON.parse(String(init.body)) : undefined });
      if (url === '/api/config')
        return Promise.resolve(
          ok({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      if (url === '/api/ledgers')
        return Promise.resolve(
          ok({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      if (url === '/api/accounts?include_hidden=true')
        return Promise.resolve(
          ok({
            items: [
              { id: 1, name: 'Assets:Savings', type: 'A', currency: 'TWD', description: '', is_hidden: true },
              { id: 2, name: 'Assets:Trip', type: 'A', currency: '', description: '', is_hidden: false },
            ],
            total: 2,
          }),
        );
      if (url.startsWith('/api/accounts?q='))
        return Promise.resolve(
          ok({
            items: [
              { id: 5, name: 'Assets:Emergency', type: 'A', currency: 'USD', description: '', is_hidden: false },
              { id: 6, name: 'Expenses:Food', type: 'E', currency: 'USD', description: '', is_hidden: false },
            ],
            total: 2,
          }),
        );
      if (url === '/api/savings-targets' && method === 'GET') return Promise.resolve(ok({ items }));
      if (url === '/api/savings-targets' && method === 'PUT')
        return Promise.resolve(putResponse ?? ok({ ...items[1], id: 9 }));
      if (url === '/api/savings-targets/stop')
        return Promise.resolve(ok({ ...items[1], stopped: true }));
      if (url.startsWith('/api/savings-targets/') && method === 'DELETE')
        return Promise.resolve(ok({ deleted: true, id: 1 }));
      throw new Error(`unexpected fetch: ${method} ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('lists targets effective in the selected month', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  const rows = await screen.findAllByTestId('savings-setting-row');
  expect(rows).toHaveLength(1); // Trip stopped in 2020-03
  expect(within(rows[0]).getByText('Assets:Savings')).toBeInTheDocument();
  expect(within(rows[0]).getByText('2020-06')).toBeInTheDocument();
  expect(within(rows[0]).getByTestId('savings-currency')).toHaveTextContent('TWD');
});

test('empty currency falls back to the default', async () => {
  render(makeTestApp('/savings?month=2020-02'));
  const rows = await screen.findAllByTestId('savings-setting-row');
  expect(within(rows[1]).getByTestId('savings-currency')).toHaveTextContent('USD');
});

test('edit creates a new version from the selected month', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  const row = (await screen.findAllByTestId('savings-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /edit/i }));
  const amount = screen.getByLabelText(/amount/i);
  await userEvent.clear(amount);
  await userEvent.type(amount, '25000');
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true));
  expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({
    account_name: 'Assets:Savings',
    effective_month: '2020-07',
    amount: 2500000,
  });
});

test('field errors from the API are shown next to the field', async () => {
  putResponse = ok(
    { error: 'validation_failed', message: 'savings target must not be negative', field: 'amount' },
    400,
  );
  render(makeTestApp('/savings?month=2020-07'));
  const row = (await screen.findAllByTestId('savings-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /edit/i }));
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  expect(await screen.findByText('savings target must not be negative')).toBeInTheDocument();
});

test('stop posts the selected month', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  const row = (await screen.findAllByTestId('savings-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /stop/i }));
  await userEvent.click(screen.getByRole('button', { name: /confirm stop/i }));
  await waitFor(() => expect(calls.some((c) => c.url === '/api/savings-targets/stop')).toBe(true));
  expect(calls.find((c) => c.url === '/api/savings-targets/stop')?.body).toEqual({
    account_name: 'Assets:Savings',
    effective_month: '2020-07',
  });
});

test('history shows all versions and deletes one after confirmation', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  const row = (await screen.findAllByTestId('savings-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /history/i }));
  const versions = screen.getAllByTestId('savings-version');
  expect(versions).toHaveLength(2);
  await userEvent.click(within(versions[0]).getByRole('button', { name: /delete/i }));
  await userEvent.click(screen.getByRole('button', { name: /confirm delete/i }));
  await waitFor(() =>
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/savings-targets/1')).toBe(true),
  );
});

test('add only offers Asset accounts and creates a target from the selected month', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  await screen.findAllByTestId('savings-setting-row');
  await userEvent.click(screen.getByRole('button', { name: /add target/i }));
  await userEvent.type(screen.getByPlaceholderText(/asset account/i), 'Emer');
  await userEvent.click(await screen.findByText('Assets:Emergency'));
  expect(screen.queryByText('Expenses:Food')).not.toBeInTheDocument();
  await userEvent.type(screen.getByLabelText(/amount/i), '0');
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true));
  expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({
    account_name: 'Assets:Emergency',
    effective_month: '2020-07',
    amount: 0,
  });
});

test('sidebar links to the savings page', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
  expect(within(nav).getByRole('link', { name: 'Savings' }).getAttribute('href')).toBe('/savings');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npx vitest run src/test/savings.page.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

`spa/src/components/savings/SavingsForm.tsx`:

```tsx
import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ApiError } from '@/lib/api';
import { setSavingsTarget } from '@/lib/api/savings';
import { parseCents } from '@/lib/budgets';
import type { SetSavingsTargetInput } from '@/lib/types';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { FormEvent } from 'react';
import { useId, useState } from 'react';
import { toast } from 'sonner';

interface Props {
  initial?: { account_name: string; amount: number; effective_month: string };
  lockAccount?: boolean;
  defaultMonth: string;
  onDone: () => void;
}

type FieldErrors = Partial<Record<'account_name' | 'effective_month' | 'amount', string>>;

export function SavingsForm({ initial, lockAccount, defaultMonth, onDone }: Props) {
  const id = useId();
  const queryClient = useQueryClient();
  const [account, setAccount] = useState(initial?.account_name ?? '');
  const [amount, setAmount] = useState(initial ? String(initial.amount / 100) : '');
  const [month, setMonth] = useState(defaultMonth);
  const [errors, setErrors] = useState<FieldErrors>({});

  const mutation = useMutation({
    mutationFn: (input: SetSavingsTargetInput) => setSavingsTarget(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['savings'] });
      toast.success('Savings target saved');
      onDone();
    },
    onError: (err) => {
      if (
        err instanceof ApiError &&
        err.field &&
        err.field in { account_name: 1, effective_month: 1, amount: 1 }
      ) {
        setErrors({ [err.field]: err.message });
      } else {
        toast.error(err instanceof Error ? err.message : 'Failed to save savings target');
      }
    },
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    const cents = parseCents(amount);
    const next: FieldErrors = {};
    if (!account) next.account_name = 'Choose an Asset account';
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
            allowedTypes={['A']}
            placeholder="Asset account…"
            aria-invalid={!!errors.account_name}
          />
        )}
        {errors.account_name && (
          <p className="mt-1 text-xs text-destructive">{errors.account_name}</p>
        )}
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
        {errors.effective_month && (
          <p className="mt-1 text-xs text-destructive">{errors.effective_month}</p>
        )}
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

`spa/src/routes/savings.tsx`:

```tsx
import { MonthPicker } from '@/components/budgets/MonthPicker';
import { SavingsForm } from '@/components/savings/SavingsForm';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { listAccounts } from '@/lib/accounts';
import { deleteSavingsTarget, stopSavingsTarget } from '@/lib/api/savings';
import { activeVersions, currentMonth, depthIn } from '@/lib/budgets';
import { useSavingsTargets } from '@/lib/hooks/useSavings';
import { type MonthSearchParams, parseMonthSearch } from '@/lib/reports-search-params';
import { useAmountFormat, useServerConfig } from '@/lib/server-config';
import type { SavingsTarget } from '@/lib/types';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { toast } from 'sonner';

export const Route = createFileRoute('/savings')({
  validateSearch: (s): MonthSearchParams => parseMonthSearch(s),
  component: SavingsPage,
});

type Panel = { kind: 'add' } | { kind: 'edit'; target: SavingsTarget } | null;

function SavingsPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/savings' });
  const month = search.month ?? currentMonth();
  const query = useSavingsTargets();
  const queryClient = useQueryClient();
  const { formatCents } = useAmountFormat();
  const defaultCurrency = useServerConfig().defaults.currency;
  const accounts = useQuery({
    queryKey: ['accounts', 'list', 'all'],
    queryFn: () => listAccounts({ include_hidden: true }),
    staleTime: 60_000,
  });
  const currencyOf = (t: SavingsTarget) => {
    const acc = accounts.data?.items.find((a) => a.id === t.account_id);
    return acc?.currency || defaultCurrency;
  };
  const [panel, setPanel] = useState<Panel>(null);
  const [historyFor, setHistoryFor] = useState<number | null>(null);
  const [confirm, setConfirm] = useState<
    { kind: 'stop'; target: SavingsTarget } | { kind: 'delete'; target: SavingsTarget } | null
  >(null);

  const onError = (err: unknown) =>
    toast.error(err instanceof Error ? err.message : 'Request failed');
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['savings'] });
  const stopMutation = useMutation({
    mutationFn: (t: SavingsTarget) =>
      stopSavingsTarget({ account_name: t.account_name, effective_month: month }),
    onSuccess: invalidate,
    onError,
  });
  const deleteMutation = useMutation({
    mutationFn: (t: SavingsTarget) => deleteSavingsTarget(t.id),
    onSuccess: invalidate,
    onError,
  });

  const setMonth = (m: string | undefined) => navigate({ search: () => (m ? { month: m } : {}) });

  if (query.isPending || accounts.isPending) return <Skeleton className="h-48" />;
  if (query.isError || accounts.isError) {
    const err = query.error ?? accounts.error;
    return (
      <Alert variant="destructive">
        <AlertTitle>
          {query.isError ? 'Failed to load savings targets' : 'Failed to load accounts'}
        </AlertTitle>
        <AlertDescription>{err instanceof Error ? err.message : 'Unknown error'}</AlertDescription>
      </Alert>
    );
  }

  const all = query.data.items;
  const active = activeVersions(all, month);
  const names = active.map((t) => t.account_name);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Savings targets</h1>
        <Button size="sm" onClick={() => setPanel({ kind: 'add' })}>
          Add target
        </Button>
      </div>
      <p className="text-sm text-muted-foreground">
        Saved is the account's balance change in the month (sub-accounts included, opening balances
        excluded).
      </p>
      <MonthPicker value={search.month} onChange={setMonth} />

      {panel?.kind === 'add' && <SavingsForm defaultMonth={month} onDone={() => setPanel(null)} />}

      {confirm && (
        <div className="flex items-center gap-3 rounded border border-amber-500 p-3 text-sm">
          <span>
            {confirm.kind === 'stop'
              ? `Stop the savings target for ${confirm.target.account_name} from ${month}?`
              : `Delete the ${confirm.target.effective_month} version of ${confirm.target.account_name}?`}
          </span>
          <Button
            size="sm"
            variant="destructive"
            onClick={() => {
              if (confirm.kind === 'stop') stopMutation.mutate(confirm.target);
              else deleteMutation.mutate(confirm.target);
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
        <p className="text-sm text-muted-foreground">No savings targets in {month}.</p>
      ) : (
        <ul className="divide-y rounded border">
          {active.map((t) => (
            <li key={t.account_id} data-testid="savings-setting-row" className="space-y-2 p-3">
              <div
                className="flex flex-wrap items-center justify-between gap-2"
                style={{ paddingLeft: `${depthIn(t.account_name, names) * 1.25}rem` }}
              >
                <div className="text-sm">
                  <div className="font-medium">{t.account_name}</div>
                  <div className="text-xs text-muted-foreground">
                    <span data-testid="savings-currency">{currencyOf(t)}</span> · from{' '}
                    <span>{t.effective_month}</span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <span className="tabular-nums text-sm">
                    {formatCents(t.amount, currencyOf(t))}
                  </span>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setPanel({ kind: 'edit', target: t })}
                  >
                    Edit
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setConfirm({ kind: 'stop', target: t })}
                  >
                    Stop
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setHistoryFor(historyFor === t.account_id ? null : t.account_id)}
                  >
                    History
                  </Button>
                </div>
              </div>
              {panel?.kind === 'edit' && panel.target.account_id === t.account_id && (
                <SavingsForm
                  initial={t}
                  lockAccount
                  defaultMonth={month}
                  onDone={() => setPanel(null)}
                />
              )}
              {historyFor === t.account_id && (
                <ul className="ml-4 space-y-1 text-xs">
                  {all
                    .filter((v) => v.account_id === t.account_id)
                    .map((v) => (
                      <li
                        key={v.id}
                        data-testid="savings-version"
                        className="flex items-center gap-3"
                      >
                        <span className="tabular-nums">{v.effective_month}</span>
                        <span className="tabular-nums">
                          {v.stopped ? 'stopped' : formatCents(v.amount, currencyOf(v))}
                        </span>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => setConfirm({ kind: 'delete', target: v })}
                        >
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

`spa/src/components/Sidebar.tsx`, in `NAV` after Budgets:

```ts
  { label: 'Savings', to: '/savings' },
```

- [ ] **Step 4: Regenerate the route tree**

```bash
cd spa && npx vite build
git -C .. restore internal/web/dist/index.html
```

Confirm `spa/src/routeTree.gen.ts` contains `/savings`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd spa && npx vitest run src/test/savings.page.test.tsx src/test/reports.savings.test.tsx src/test/sidebar.reconcile.test.tsx` — PASS.
Run: `cd spa && npx tsc -b && npx biome check src` — clean.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/savings/SavingsForm.tsx spa/src/routes/savings.tsx spa/src/components/Sidebar.tsx spa/src/routeTree.gen.ts spa/src/test/savings.page.test.tsx
git commit -m "feat(spa): add savings targets settings page

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: SPA dashboard card

**Files:**
- Create: `spa/src/components/dashboard/widgets/SavingsProgress.tsx`
- Modify: `spa/src/lib/dashboard/types.ts` (`WidgetId` gains `'savings-progress'`)
- Modify: `spa/src/lib/dashboard/registry.tsx` (import + entry)
- Modify: `spa/src/lib/dashboard/defaults.ts` (layout comment + item)
- Test: `spa/src/test/dashboard.savings-progress.test.tsx`

**Interfaces:**
- Consumes: `useSavingsReport()` (no month), `SavingsProgressBar`, `remainingLabel`, `elapsedFraction`, `useAmountFormat`.
- Produces: `SavingsProgress()` widget, registered as `'savings-progress'`.

- [ ] **Step 1: Write the failing test**

`spa/src/test/dashboard.savings-progress.test.tsx`:

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRouter,
} from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { SavingsProgress } from '../components/dashboard/widgets/SavingsProgress';
import { currentMonth } from '../lib/budgets';
import { DEFAULT_STATE } from '../lib/dashboard/defaults';
import { ALL_WIDGET_IDS } from '../lib/dashboard/registry';
import { withServerConfig } from './test-app';

const ok = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const row = (name: string, target: number, saved: number, ytdRemaining: number) => ({
  account_id: name.length,
  account_name: name,
  currency: 'USD',
  effective_month: '2026-01',
  target,
  saved,
  remaining: target - saved,
  ytd_target: 0,
  ytd_saved: 0,
  ytd_remaining: ytdRemaining,
  months: [],
  excluded_accounts: [],
});

let urls: string[];
let rows: ReturnType<typeof row>[];

beforeEach(() => {
  urls = [];
  rows = [row('Assets:Savings', 1500000, 1500000, -650000), row('Assets:Trip', 20000, -5000, 25000)];
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      urls.push(url);
      return Promise.resolve(
        ok({
          month: currentMonth(),
          rows,
          total_target: {},
          total_saved: {},
          total_ytd_target: {},
          total_ytd_saved: {},
        }),
      );
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

function renderWidget(node: ReactNode) {
  const rootRoute = createRootRoute({ component: () => <>{node}</> });
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return render(
    <QueryClientProvider client={qc}>
      {withServerConfig(<RouterProvider router={router} />)}
    </QueryClientProvider>,
  );
}

test('lets the server choose the month (no UTC month on the client)', async () => {
  renderWidget(<SavingsProgress />);
  await waitFor(() => expect(urls).toContain('/api/reports/savings'));
});

test('shows one line per target with status and ytd remaining', async () => {
  renderWidget(<SavingsProgress />);
  const items = await screen.findAllByTestId('savings-progress-item');
  expect(items.map((i) => i.getAttribute('data-account'))).toEqual(['Assets:Savings', 'Assets:Trip']);
  expect(within(items[0]).getByTestId('savings-bar').getAttribute('data-status')).toBe('achieved');
  expect(within(items[1]).getByTestId('savings-bar').getAttribute('data-status')).toBe('negative');
  expect(within(items[0]).getByText(/over target/)).toBeInTheDocument();
  expect(within(items[1]).getByText(/to go/)).toBeInTheDocument();
});

test('links to the savings report', async () => {
  renderWidget(<SavingsProgress />);
  await screen.findAllByTestId('savings-progress-item');
  expect(screen.getByRole('link').getAttribute('href')).toBe('/reports/savings');
});

test('empty state links to /savings', async () => {
  rows = [];
  renderWidget(<SavingsProgress />);
  const link = await screen.findByRole('link', { name: /set up savings targets/i });
  expect(link.getAttribute('href')).toBe('/savings');
});

test('widget is registered and in the default layout', () => {
  expect(ALL_WIDGET_IDS).toContain('savings-progress');
  expect(DEFAULT_STATE.layout.some((g) => g.i === 'savings-progress')).toBe(true);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npx vitest run src/test/dashboard.savings-progress.test.tsx`
Expected: FAIL (module not found).

- [ ] **Step 3: Implement**

`spa/src/components/dashboard/widgets/SavingsProgress.tsx`:

```tsx
import { Link } from '@tanstack/react-router';
import { elapsedFraction } from '../../../lib/budgets';
import { useSavingsReport } from '../../../lib/hooks/useSavings';
import { remainingLabel } from '../../../lib/savings';
import { useAmountFormat } from '../../../lib/server-config';
import { SavingsProgressBar } from '../../savings/SavingsProgressBar';

export function SavingsProgress() {
  // No month: the server resolves the current month in its local time.
  const q = useSavingsReport();
  const { formatCents } = useAmountFormat();
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const report = q.data;
  if (!report || report.rows.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        <span>
          No savings targets yet.{' '}
          <Link to="/savings" className="underline">
            Set up savings targets
          </Link>
        </span>
      </div>
    );
  }

  const elapsed = elapsedFraction(report.month);
  return (
    <Link to="/reports/savings" className="flex h-full flex-col">
      <ul className="flex flex-col gap-2 overflow-auto">
        {report.rows.map((r) => {
          const fmt = (c: number) => formatCents(c, r.currency);
          return (
            <li
              key={r.account_id}
              data-testid="savings-progress-item"
              data-account={r.account_name}
              className="text-xs"
            >
              <div className="flex justify-between gap-2">
                <span className="truncate">{r.account_name}</span>
                <span className="tabular-nums">
                  {fmt(r.saved)} / {fmt(r.target)}
                </span>
              </div>
              <div className="mt-0.5">
                <SavingsProgressBar target={r.target} saved={r.saved} elapsed={elapsed} compact />
              </div>
              <div className="mt-0.5 text-muted-foreground">
                YTD {remainingLabel(r.ytd_remaining, fmt)}
              </div>
            </li>
          );
        })}
      </ul>
    </Link>
  );
}
```

`spa/src/lib/dashboard/types.ts`:

```ts
  | 'budget-progress'
  | 'savings-progress';
```

`spa/src/lib/dashboard/registry.tsx`: import `import { SavingsProgress } from '../../components/dashboard/widgets/SavingsProgress';` and after the `'budget-progress'` entry add:

```tsx
  'savings-progress': {
    id: 'savings-progress',
    title: 'Savings This Month',
    defaultConfig: {},
    component: SavingsProgress,
  },
```

`spa/src/lib/dashboard/defaults.ts`: change the comment to `//   row 5: [budget-progress  ] [savings-progress ]` and append to `DEFAULT_LAYOUT`:

```ts
  { i: 'savings-progress', x: 1, y: 5, w: 1, h: 2 },
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npx vitest run src/test/dashboard.savings-progress.test.tsx src/test/dashboard.budget-progress.test.tsx` and `npx vitest run src/components/dashboard` — PASS.
Run: `cd spa && npx tsc -b && npx biome check src` — clean.

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/dashboard/widgets/SavingsProgress.tsx spa/src/lib/dashboard/types.ts spa/src/lib/dashboard/registry.tsx spa/src/lib/dashboard/defaults.ts spa/src/test/dashboard.savings-progress.test.tsx
git commit -m "feat(spa): add savings progress dashboard card

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Domain docs, decisions, final verification

**Files:**
- Modify: `docs/domain.md` (new section after `## Budgets`)
- Modify: `docs/decisions.md` (after "Budget actuals use signed sums")
- Modify: `docs/architecture.md` (facade paragraph and service list)
- Modify: `spa/README.md` (routes table)

- [ ] **Step 1: `docs/domain.md`**

After the `## Budgets` section (before `## Errors you will meet`):

```markdown
## Savings targets

A savings target is a monthly amount to put into an `Asset` (`A`) account, leaf or parent. Targets live in the `savings_targets` table (`migrations/0013_create_savings_targets.up.sql`) and are versioned exactly like budgets: a row applies from its `effective_month` until a later row for the same account; set upserts per (account, month); stop writes `stopped=1, amount=0`; `amount=0` is a real target ("do not draw the account down").

- **Saved** is the signed sum of every split on the account and its descendants (`isSelfOrDescendant`) in the month, whatever the transaction type: transfers in add, transfers out subtract, interest adds. Transfers between the account and its own descendants net to zero. It may be negative.
- **Opening transactions never count.** An opening balance is existing money, not money saved; in a month with an `Opening` transaction, Saved differs from the balance change by that amount.
- **Currency** is the account's currency (empty means `config.Defaults.Currency`). Descendant splits in another currency are excluded; those seen in the report month are listed in `ExcludedAccounts`.
- **Remaining** is `Target - Saved`, signed: positive is still missing, negative means the target was exceeded.
- **Year to date** covers January of the report month's year through the report month, counting only months in which the target was active; each such month is listed in `Months`. There is no rollover: a shortfall shows up in the YTD figures, not in next month's target.
- **Totals** are per currency and skip rows with a targeted ancestor in the same currency (`hasAncestorInCurrency`).
- Months are local time; the report loads splits once from January 1 to the end of the report month.

Code lives in: `internal/service/savings_service.go`, `internal/service/savings_report.go` (`GenerateSavingsReport`), `internal/service/versions.go` (version selection shared with budgets), `internal/model/savings.go`, `internal/store/sqlite_savings_target.go`. Endpoints are in [http-api.md](http-api.md); the CLI is `kea savings` (`cmd/savings/`, `ui/views/savings.go`).
```

Also in the `## Budgets` section, update the code pointer: version selection now lives in `internal/service/versions.go` (`activeBudgets` wraps `activeVersions`).

- [ ] **Step 2: `docs/decisions.md`**

After "Budget actuals use signed sums":

```markdown
### Savings are measured as the target account's balance change
- **Decision:** A savings target's "saved" is the signed sum of the splits on the target Asset account and its descendants in the month, regardless of transaction type, instead of income minus expense or net-worth change.
- **Why:** The user keeps a dedicated savings account; what matters is whether money actually arrived there. Income minus expense counts money that may stay in a spending account, and net-worth change is noisy with equity transactions.
- **Where:** `internal/service/savings_report.go` (`GenerateSavingsReport`)
- **Source:** [2026-10-06-savings-target-design.md](history/superpowers/specs/2026-10-06-savings-target-design.md)

### Opening transactions do not count as saving
- **Decision:** `Opening`-typed transactions are excluded from saved amounts; every other type counts.
- **Why:** Recording an existing balance is not saving; without the exclusion the month a savings account is set up would show its whole balance as saved.
- **Where:** `internal/service/savings_report.go`
- **Source:** [2026-10-06-savings-target-design.md](history/superpowers/specs/2026-10-06-savings-target-design.md)

### Savings targets have their own table and service
- **Decision:** Savings targets use a separate `savings_targets` table and `SavingsService`; only the version-selection logic is shared with budgets, through generic helpers.
- **Why:** Budgets cap Expense spending and targets set a floor on Asset growth; a shared table would need a kind column, a SQLite table rebuild and kind checks in every budget query.
- **Where:** `migrations/0013_create_savings_targets.up.sql`, `internal/service/savings_service.go`, `internal/service/versions.go`
- **Source:** [2026-10-06-savings-target-design.md](history/superpowers/specs/2026-10-06-savings-target-design.md)
```

- [ ] **Step 3: `docs/architecture.md`**

Update the facade paragraph to:

```markdown
`internal/service/service.go` defines `Service`, which holds unexported `*AccountService`, `*TransactionService`, `*BudgetService`, `*SavingsService` and `*config.Config` fields. Callers use `svc.Account()`, `svc.Transaction()`, `svc.Budget()`, `svc.Savings()` and `svc.Config()`. `NewService` takes an `AccountRepository`, a `TransactionRepository`, a `BudgetRepository`, a `SavingsTargetRepository` and a `TransactionManager`; `app.NewApp` passes the same `*store.Store` for all five.
```

After the `BudgetService` bullet add:

```markdown
- `SavingsService` (`internal/service/savings_service.go`, `internal/service/savings_report.go`) — savings target versions and target vs saved. Version selection shared with budgets is in `internal/service/versions.go`.
```

- [ ] **Step 4: `spa/README.md`**

After the `/budgets` row:

```markdown
| `/reports/savings` | `spa/src/routes/reports.savings.tsx` | Savings target vs saved, month and year to date |
| `/savings` | `spa/src/routes/savings.tsx` | Savings target settings (set, stop, delete; Asset accounts only) |
```

- [ ] **Step 5: Full verification**

```bash
scripts/check-docs.sh
go build ./... && go vet ./... && go test ./...
cd spa && npx tsc -b && npx biome check src && npx vitest run
cd spa && npm run build
git -C .. restore internal/web/dist/index.html
cd .. && make build && HOME=$(mktemp -d) ./kea savings --help
git status --short
```

Expected: docs check passes; all Go tests pass; SPA type check and Biome are clean; Vitest reports exactly the 6 baseline failures in `balances.test.tsx` / `balances.link.test.tsx` and nothing else; build succeeds; `git status` shows only the doc files from this task (no `internal/web/dist/index.html`).

- [ ] **Step 6: Commit**

```bash
git add docs/domain.md docs/decisions.md docs/architecture.md spa/README.md
git commit -m "docs: document savings targets

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 7: Hand-off reminder**

Tell the user: the `kea` binary was rebuilt with migration `0013`; an older `kea` binary refuses to open a ledger that has been migrated, so they must re-run the install script on every other host before using those ledgers there.
