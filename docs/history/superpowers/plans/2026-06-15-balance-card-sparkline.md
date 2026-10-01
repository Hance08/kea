# Balance Card Sparkline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a small bottom-right sparkline to each card in the Balances cards view showing the account's full balance history at monthly granularity, backed by a new batched `/api/balances/history` server endpoint.

**Architecture:** A new server endpoint reads splits joined to transactions, groups by account_id and `%Y-%m` (UTC), computes a running cumulative sum per account, applies natural-amount sign, and returns one series per Asset/Liability account in a single response. The SPA Balances page fires a second `useQuery` for the history; cards look up their series by `account_id` and render an inline-SVG sparkline (no chart library).

**Tech Stack:** Go 1.x with chi router + SQLite (existing repository/service/store pattern), React 18 + TanStack Query + TanStack Router + Tailwind CSS in the SPA, hand-rolled SVG for the sparkline.

**Spec:** [docs/superpowers/specs/2026-06-15-balance-card-sparkline-design.md](../specs/2026-06-15-balance-card-sparkline-design.md)

---

## File Structure

**Backend (new):**

- `internal/model/balance_history.go` — `MonthlyBalancePoint` and `AccountMonthlySeries` types.

**Backend (modified):**

- `internal/repository/interfaces.go` — add `MonthlySplitTotal` struct and `GetMonthlySplitTotalsForAssetsAndLiabilities` to `TransactionRepository`.
- `internal/store/sqlite_transaction.go` — implement the new repository method.
- `internal/service/transaction_service.go` — add `GetMonthlyBalanceHistory` method.
- `internal/service/transaction_service_test.go` — tests for the new service method.
- `internal/service/testhelper_test.go` — extend `mockTransactionRepo` with the new method.
- `internal/api/reports.go` — add `handleBalanceHistory` handler.
- `internal/api/reports_test.go` — handler test.
- `internal/api/router.go` — wire the new route.

**Frontend (new):**

- `spa/src/components/balances/Sparkline.tsx` — inline-SVG sparkline component.
- `spa/src/test/sparkline.test.tsx` — Vitest tests for `Sparkline`.

**Frontend (modified):**

- `spa/src/lib/types.ts` — add `BalanceHistoryPoint`, `AccountBalanceHistory`, `BalanceHistoryResponse`.
- `spa/src/lib/api.ts` — add `getBalanceHistory()`.
- `spa/src/lib/accounts.ts` — add `balanceStrokeColor` helper that mirrors `balanceColor`.
- `spa/src/components/balances/BalanceCard.tsx` — accept optional `points`, render sparkline bottom-right.
- `spa/src/components/balances/BalanceCardGrid.tsx` — accept optional `historyByAccount` map, pass points to each card.
- `spa/src/components/balances/BalanceColumn.tsx` — thread `historyByAccount` through to the grid.
- `spa/src/routes/balances.tsx` — second `useQuery`, build `Map<number, BalanceHistoryPoint[]>`, pass down.
- `spa/src/test/balances.test.tsx` — add `/api/balances/history` mock to the existing fetch stub.

---

## Phase A — Backend

### Task 1: Add model types

**Files:**
- Create: `internal/model/balance_history.go`

- [ ] **Step 1: Create the model file**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// MonthlyBalancePoint is one end-of-month balance for an account, in cents,
// expressed in the account's natural-amount direction.
type MonthlyBalancePoint struct {
	Month   string `json:"month"`   // "YYYY-MM" (UTC year-month)
	Balance int64  `json:"balance"` // cents, natural-amount
}

// AccountMonthlySeries is the full monthly balance history for one account.
// The points slice is ordered ascending by month and starts at the account's
// earliest month with activity (no front-fill of zero months).
type AccountMonthlySeries struct {
	AccountID int64                 `json:"account_id"`
	Currency  string                `json:"currency"`
	Points    []MonthlyBalancePoint `json:"points"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/model/balance_history.go
git commit -m "feat(model): add monthly balance series types"
```

---

### Task 2: Add repository interface and mock

**Files:**
- Modify: `internal/repository/interfaces.go`
- Modify: `internal/service/testhelper_test.go`

- [ ] **Step 1: Add the repository type and method**

In `internal/repository/interfaces.go`, after the existing `TransactionRepository` block (just before the closing `}` of the interface), add:

```go
	// GetMonthlySplitTotalsForAssetsAndLiabilities returns, for every
	// (account_id, month) with activity on an Asset or Liability account,
	// the sum of split amounts in that month. Months are UTC "YYYY-MM"
	// strings derived from transaction timestamps. Ordered by account_id
	// ASC, month ASC. Used to build per-account monthly balance series.
	GetMonthlySplitTotalsForAssetsAndLiabilities(ctx context.Context) ([]MonthlySplitTotal, error)
```

And, at the bottom of the file (after the `TransactionManager` interface), add:

```go
// MonthlySplitTotal is one (account, month) bucket of summed split amounts.
// Amount is stored-sign (not natural-amount); the service layer applies the
// natural-amount conversion based on AccountType.
type MonthlySplitTotal struct {
	AccountID   int64
	AccountType model.AccountType
	Currency    string
	Month       string // "YYYY-MM" UTC
	Amount      int64  // cents, stored sign, sum within the month
}
```

- [ ] **Step 2: Add a stub mock so the service tests still compile**

In `internal/service/testhelper_test.go`, add fields to `mockTransactionRepo` (inside the struct):

```go
	// monthly history support
	monthlySplitTotals    []repository.MonthlySplitTotal
	monthlySplitTotalsErr error
```

Then, anywhere in the file's method list for `mockTransactionRepo`, add:

```go
func (m *mockTransactionRepo) GetMonthlySplitTotalsForAssetsAndLiabilities(_ context.Context) ([]repository.MonthlySplitTotal, error) {
	if m.monthlySplitTotalsErr != nil {
		return nil, m.monthlySplitTotalsErr
	}
	return m.monthlySplitTotals, nil
}
```

If `repository` isn't already imported in `testhelper_test.go`, add `"github.com/hance08/kea/internal/repository"` to the imports.

- [ ] **Step 3: Verify it builds and existing tests still pass**

Run: `go build ./... && go test ./internal/service/...`
Expected: build succeeds; tests pass (no behavior changes yet).

- [ ] **Step 4: Commit**

```bash
git add internal/repository/interfaces.go internal/service/testhelper_test.go
git commit -m "feat(repository): add GetMonthlySplitTotalsForAssetsAndLiabilities"
```

---

### Task 3: Implement the SQL query in the store

**Files:**
- Modify: `internal/store/sqlite_transaction.go`

- [ ] **Step 1: Add the new method to the Store**

At the end of `internal/store/sqlite_transaction.go`, add:

```go
// GetMonthlySplitTotalsForAssetsAndLiabilities returns, for every
// (account_id, month) with activity on an Asset or Liability account,
// the sum of split amounts in that month. Months are UTC "YYYY-MM"
// strings derived from each transaction's Unix timestamp.
func (s *Store) GetMonthlySplitTotalsForAssetsAndLiabilities(ctx context.Context) ([]repository.MonthlySplitTotal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
        SELECT
            s.account_id,
            a.type,
            a.currency,
            strftime('%Y-%m', t.timestamp, 'unixepoch') AS month,
            SUM(s.amount) AS total
        FROM splits s
        JOIN transactions t ON t.id = s.transaction_id
        JOIN accounts     a ON a.id = s.account_id
        WHERE a.type IN ('A', 'L')
        GROUP BY s.account_id, month
        ORDER BY s.account_id, month
    `)
	if err != nil {
		return nil, fmt.Errorf("failed to query monthly split totals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []repository.MonthlySplitTotal
	for rows.Next() {
		var r repository.MonthlySplitTotal
		if err := rows.Scan(&r.AccountID, &r.AccountType, &r.Currency, &r.Month, &r.Amount); err != nil {
			return nil, fmt.Errorf("failed to scan monthly split total: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return result, nil
}
```

Make sure `internal/store/sqlite_transaction.go` imports `"github.com/hance08/kea/internal/repository"` — add it if not present.

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/store/sqlite_transaction.go
git commit -m "feat(store): implement GetMonthlySplitTotalsForAssetsAndLiabilities"
```

---

### Task 4: Service method with unit tests

**Files:**
- Modify: `internal/service/transaction_service.go`
- Modify: `internal/service/transaction_service_test.go`

- [ ] **Step 1: Write the failing test for an asset's running balance**

Append to `internal/service/transaction_service_test.go`:

```go
func TestGetMonthlyBalanceHistory_AssetRunningBalance(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	// Asset account id=10
	accRepo.accountsByID[10] = &model.Account{
		ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
	}
	txRepo.monthlySplitTotals = []repository.MonthlySplitTotal{
		{AccountID: 10, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-01", Amount: 100000},
		{AccountID: 10, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-02", Amount: 50000},
		{AccountID: 10, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-03", Amount: -20000},
	}

	got, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err != nil {
		t.Fatalf("GetMonthlyBalanceHistory: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("series count: got %d, want 1", len(got))
	}
	if got[0].AccountID != 10 {
		t.Errorf("AccountID: got %d, want 10", got[0].AccountID)
	}
	if got[0].Currency != "USD" {
		t.Errorf("Currency: got %q, want USD", got[0].Currency)
	}
	want := []model.MonthlyBalancePoint{
		{Month: "2024-01", Balance: 100000},
		{Month: "2024-02", Balance: 150000},
		{Month: "2024-03", Balance: 130000},
	}
	if !reflect.DeepEqual(got[0].Points, want) {
		t.Errorf("Points: got %+v, want %+v", got[0].Points, want)
	}
}
```

The existing file already imports `context`, `errors`, and `repository`. Add `"reflect"` and `"github.com/hance08/kea/internal/model"` to the import block.

- [ ] **Step 2: Verify it fails to compile**

Run: `go test ./internal/service/ -run TestGetMonthlyBalanceHistory_AssetRunningBalance`
Expected: build error referencing `GetMonthlyBalanceHistory` undefined.

- [ ] **Step 3: Implement the service method**

Append to `internal/service/transaction_service.go`:

```go
// GetMonthlyBalanceHistory returns, for every Asset and Liability account
// with activity, the full monthly end-of-month balance series in the
// account's natural-amount direction. Accounts with no activity are omitted.
// Series are ordered by AccountID ASC; points within each series ascend by month.
func (ts *TransactionService) GetMonthlyBalanceHistory(ctx context.Context) ([]model.AccountMonthlySeries, error) {
	rows, err := ts.repo.GetMonthlySplitTotalsForAssetsAndLiabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly split totals: %w", err)
	}

	// rows are already ordered by account_id, month ASC.
	var out []model.AccountMonthlySeries
	var cur *model.AccountMonthlySeries
	var running int64
	for _, r := range rows {
		if cur == nil || cur.AccountID != r.AccountID {
			out = append(out, model.AccountMonthlySeries{
				AccountID: r.AccountID,
				Currency:  r.Currency,
				Points:    nil,
			})
			cur = &out[len(out)-1]
			running = 0
		}
		running += r.Amount
		balance := running
		// Liability: stored is negative when the debt grows, so natural-amount = -stored.
		if r.AccountType == model.AccountTypeLiability {
			balance = -balance
		}
		cur.Points = append(cur.Points, model.MonthlyBalancePoint{
			Month:   r.Month,
			Balance: balance,
		})
	}
	return out, nil
}
```

If `fmt` isn't already imported in `transaction_service.go`, add it.

- [ ] **Step 4: Verify the test passes**

Run: `go test ./internal/service/ -run TestGetMonthlyBalanceHistory_AssetRunningBalance -v`
Expected: PASS.

- [ ] **Step 5: Add the remaining cases**

Append to `internal/service/transaction_service_test.go`:

```go
func TestGetMonthlyBalanceHistory_LiabilityNaturalSign(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)
	accRepo.accountsByID[20] = &model.Account{
		ID: 20, Name: "Liabilities:Card", Type: model.AccountTypeLiability, Currency: "USD",
	}
	txRepo.monthlySplitTotals = []repository.MonthlySplitTotal{
		// Liability balances are stored negative when the debt grows.
		{AccountID: 20, AccountType: model.AccountTypeLiability, Currency: "USD", Month: "2024-01", Amount: -50000},
		{AccountID: 20, AccountType: model.AccountTypeLiability, Currency: "USD", Month: "2024-02", Amount: -30000},
	}

	got, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []model.MonthlyBalancePoint{
		{Month: "2024-01", Balance: 50000},  // -(-50000)
		{Month: "2024-02", Balance: 80000},  // -(-50000 + -30000)
	}
	if !reflect.DeepEqual(got[0].Points, want) {
		t.Errorf("Points: got %+v, want %+v", got[0].Points, want)
	}
}

func TestGetMonthlyBalanceHistory_SingleMonth(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)
	accRepo.accountsByID[30] = &model.Account{
		ID: 30, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD",
	}
	txRepo.monthlySplitTotals = []repository.MonthlySplitTotal{
		{AccountID: 30, AccountType: model.AccountTypeAsset, Currency: "USD", Month: "2024-05", Amount: 1234},
	}
	got, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || len(got[0].Points) != 1 {
		t.Fatalf("expected 1 series with 1 point, got %+v", got)
	}
	if got[0].Points[0].Balance != 1234 {
		t.Errorf("Balance: got %d, want 1234", got[0].Points[0].Balance)
	}
}

func TestGetMonthlyBalanceHistory_EmptyLedger(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)
	txRepo.monthlySplitTotals = nil

	got, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty series list, got %+v", got)
	}
}

func TestGetMonthlyBalanceHistory_RepoError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)
	txRepo.monthlySplitTotalsErr = errors.New("boom")

	_, err := svc.GetMonthlyBalanceHistory(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
```

(No new imports needed for Step 5 — `errors` is already in the import block.)

- [ ] **Step 6: Run all new tests**

Run: `go test ./internal/service/ -run TestGetMonthlyBalanceHistory -v`
Expected: all four sub-tests PASS.

- [ ] **Step 7: Run the full service test suite to check for regressions**

Run: `go test ./internal/service/...`
Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/service/transaction_service.go internal/service/transaction_service_test.go
git commit -m "feat(service): add GetMonthlyBalanceHistory"
```

---

### Task 5: HTTP handler with tests

**Files:**
- Modify: `internal/api/reports.go`
- Modify: `internal/api/reports_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write the failing handler test**

Append to `internal/api/reports_test.go`:

```go
func TestHandleBalanceHistory(t *testing.T) {
	ts, svc := newServerWithStore(t)
	seedAccount(t, svc, "Assets:Cash", model.AccountTypeAsset, 0)
	seedAccount(t, svc, "Revenue:Salary", model.AccountTypeRevenue, 0)

	jan1 := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC).Unix()
	feb1 := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC).Unix()
	seedTransaction(t, svc, "Revenue:Salary", "Assets:Cash", 100000, jan1, "pay1", model.TxTypeIncome, model.StatusCleared)
	seedTransaction(t, svc, "Revenue:Salary", "Assets:Cash", 200000, feb1, "pay2", model.TxTypeIncome, model.StatusCleared)

	resp, err := http.Get(ts.URL + "/api/balances/history")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}

	var got struct {
		Items []struct {
			AccountID int64  `json:"account_id"`
			Currency  string `json:"currency"`
			Points    []struct {
				Month   string `json:"month"`
				Balance int64  `json:"balance"`
			} `json:"points"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Only the Asset account should appear; Revenue is excluded.
	if len(got.Items) != 1 {
		t.Fatalf("items: got %d, want 1: %+v", len(got.Items), got)
	}
	pts := got.Items[0].Points
	if len(pts) != 2 {
		t.Fatalf("points: got %d, want 2", len(pts))
	}
	if pts[0].Month != "2026-01" || pts[0].Balance != 100000 {
		t.Errorf("pts[0]: got %+v, want {2026-01 100000}", pts[0])
	}
	if pts[1].Month != "2026-02" || pts[1].Balance != 300000 {
		t.Errorf("pts[1]: got %+v, want {2026-02 300000}", pts[1])
	}
}
```

- [ ] **Step 2: Run it — expect a 404 or panic**

Run: `go test ./internal/api/ -run TestHandleBalanceHistory -v`
Expected: FAIL (status 404 from the SPA fallback or 405 — route is not yet wired).

- [ ] **Step 3: Add the handler**

Append to `internal/api/reports.go`:

```go
type balanceHistoryResponse struct {
	Items []model.AccountMonthlySeries `json:"items"`
}

func (s *Server) handleBalanceHistory(w http.ResponseWriter, r *http.Request) error {
	items, err := s.svc.Transaction().GetMonthlyBalanceHistory(r.Context())
	if err != nil {
		return err
	}
	if items == nil {
		items = []model.AccountMonthlySeries{}
	}
	return writeJSON(w, http.StatusOK, balanceHistoryResponse{Items: items})
}
```

If `internal/api/reports.go` doesn't already import `"github.com/hance08/kea/internal/model"`, add it.

- [ ] **Step 4: Wire the route**

In `internal/api/router.go`, inside the `r.Route("/api", func(r chi.Router) { ... })` block, add (next to the existing `/balances` route):

```go
		r.Method(http.MethodGet, "/balances/history", apiHandler(s.handleBalanceHistory))
```

- [ ] **Step 5: Run the handler test again**

Run: `go test ./internal/api/ -run TestHandleBalanceHistory -v`
Expected: PASS.

- [ ] **Step 6: Run the full backend test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/reports.go internal/api/reports_test.go internal/api/router.go
git commit -m "feat(api): add GET /api/balances/history"
```

---

## Phase B — Frontend

### Task 6: Add SPA types

**Files:**
- Modify: `spa/src/lib/types.ts`

- [ ] **Step 1: Append the new types**

Add to the end of `spa/src/lib/types.ts`:

```ts
export interface BalanceHistoryPoint {
  month: string;   // "YYYY-MM"
  balance: number; // cents, natural-amount
}

export interface AccountBalanceHistory {
  account_id: number;
  currency: string;
  points: BalanceHistoryPoint[];
}

export interface BalanceHistoryResponse {
  items: AccountBalanceHistory[];
}
```

- [ ] **Step 2: Verify the SPA still type-checks**

Run: `cd spa && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/lib/types.ts
git commit -m "feat(spa): add balance-history types"
```

---

### Task 7: Add the API client function

**Files:**
- Modify: `spa/src/lib/api.ts`

- [ ] **Step 1: Add the import for the new type**

Modify the import block at the top of `spa/src/lib/api.ts`:

```ts
import type {
  AccountBalance,
  BalanceHistoryResponse,
  LedgerInfo,
  LedgerListResponse,
  ListResult,
  ServerConfig,
} from './types';
```

- [ ] **Step 2: Add the fetch function**

Add immediately after the existing `getBalances` function in `spa/src/lib/api.ts`:

```ts
export function getBalanceHistory(): Promise<BalanceHistoryResponse> {
  return apiFetch<BalanceHistoryResponse>('/api/balances/history');
}
```

- [ ] **Step 3: Verify**

Run: `cd spa && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add spa/src/lib/api.ts
git commit -m "feat(spa): add getBalanceHistory api client"
```

---

### Task 8: Add the `balanceStrokeColor` helper

**Files:**
- Modify: `spa/src/lib/accounts.ts`

- [ ] **Step 1: Add the helper next to `balanceColor`**

Add immediately after the existing `balanceColor` function in `spa/src/lib/accounts.ts`:

```ts
// Same rule as balanceColor, but returns an SVG stroke-* Tailwind class for
// use with the Sparkline component. Zero amount stays neutral (no stroke
// override; caller may default to a muted color).
export function balanceStrokeColor(type: AccountType, amount: number): string {
  const inverted = type === 'R' || type === 'E';
  if (amount > 0) return inverted ? 'stroke-red-600' : 'stroke-green-600';
  if (amount < 0) return inverted ? 'stroke-green-600' : 'stroke-red-600';
  return 'stroke-muted-foreground';
}
```

- [ ] **Step 2: Verify**

Run: `cd spa && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/lib/accounts.ts
git commit -m "feat(spa): add balanceStrokeColor helper"
```

---

### Task 9: Sparkline component with tests

**Files:**
- Create: `spa/src/components/balances/Sparkline.tsx`
- Create: `spa/src/test/sparkline.test.tsx`

- [ ] **Step 1: Write the failing test file**

Create `spa/src/test/sparkline.test.tsx`:

```tsx
import { render } from '@testing-library/react';
import { expect, test } from 'vitest';
import { Sparkline } from '@/components/balances/Sparkline';

test('renders nothing when fewer than 2 points', () => {
  const { container } = render(
    <Sparkline points={[{ month: '2024-01', balance: 100 }]} strokeClassName="stroke-green-600" />,
  );
  expect(container.querySelector('svg')).toBeNull();
});

test('renders a polyline when given >= 2 points', () => {
  const { container } = render(
    <Sparkline
      points={[
        { month: '2024-01', balance: 100 },
        { month: '2024-02', balance: 200 },
      ]}
      strokeClassName="stroke-green-600"
    />,
  );
  const line = container.querySelector('polyline');
  expect(line).not.toBeNull();
  expect(line?.getAttribute('class')).toContain('stroke-green-600');
});

test('flat history renders a horizontal line at vertical center', () => {
  const { container } = render(
    <Sparkline
      points={[
        { month: '2024-01', balance: 500 },
        { month: '2024-02', balance: 500 },
        { month: '2024-03', balance: 500 },
      ]}
      strokeClassName="stroke-green-600"
    />,
  );
  const line = container.querySelector('polyline');
  // viewBox is 0 0 100 30 → vertical center y = 15.
  const points = line?.getAttribute('points') ?? '';
  const ys = points.split(' ').map((p) => Number(p.split(',')[1]));
  for (const y of ys) {
    expect(y).toBeCloseTo(15);
  }
});

test('scales points to the line min/max', () => {
  const { container } = render(
    <Sparkline
      points={[
        { month: '2024-01', balance: 0 },
        { month: '2024-02', balance: 100 },
      ]}
      strokeClassName="stroke-green-600"
    />,
  );
  // 2 points → x = 0 and 100. Min balance maps to y=30 (bottom), max to y=0 (top).
  const points = container.querySelector('polyline')?.getAttribute('points') ?? '';
  expect(points).toBe('0,30 100,0');
});
```

- [ ] **Step 2: Run the test — expect compile/import failure**

Run: `cd spa && npx vitest run sparkline`
Expected: FAIL — cannot resolve `@/components/balances/Sparkline`.

- [ ] **Step 3: Implement the component**

Create `spa/src/components/balances/Sparkline.tsx`:

```tsx
import { cn } from '@/lib/cn';
import type { BalanceHistoryPoint } from '@/lib/types';

interface Props {
  points: BalanceHistoryPoint[];
  strokeClassName: string;
  className?: string;
}

const VIEWBOX_W = 100;
const VIEWBOX_H = 30;

export function Sparkline({ points, strokeClassName, className }: Props) {
  if (points.length < 2) return null;

  const balances = points.map((p) => p.balance);
  const min = Math.min(...balances);
  const max = Math.max(...balances);
  const range = max - min;
  const xStep = VIEWBOX_W / (points.length - 1);

  const coords = points
    .map((p, i) => {
      const x = i * xStep;
      const y = range === 0 ? VIEWBOX_H / 2 : VIEWBOX_H - ((p.balance - min) / range) * VIEWBOX_H;
      return `${x},${y}`;
    })
    .join(' ');

  return (
    <svg
      viewBox={`0 0 ${VIEWBOX_W} ${VIEWBOX_H}`}
      preserveAspectRatio="none"
      aria-hidden="true"
      className={cn('h-6 w-20', className)}
    >
      <polyline
        points={coords}
        fill="none"
        strokeWidth={1.5}
        vectorEffect="non-scaling-stroke"
        className={strokeClassName}
      />
    </svg>
  );
}
```

- [ ] **Step 4: Run the tests — expect PASS**

Run: `cd spa && npx vitest run sparkline`
Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/balances/Sparkline.tsx spa/src/test/sparkline.test.tsx
git commit -m "feat(spa): add Sparkline component"
```

---

### Task 10: Wire sparkline into `BalanceCard`

**Files:**
- Modify: `spa/src/components/balances/BalanceCard.tsx`

- [ ] **Step 1: Update the file in place**

Replace the entire contents of `spa/src/components/balances/BalanceCard.tsx` with:

```tsx
import { Sparkline } from '@/components/balances/Sparkline';
import { balanceColor, balanceStrokeColor } from '@/lib/accounts';
import { cn } from '@/lib/cn';
import { formatBalanceAbs } from '@/lib/format';
import type { AccountBalance, BalanceHistoryPoint } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  row: AccountBalance;
  columnLabel: 'Assets' | 'Liabilities';
  share: number | null;
  points?: BalanceHistoryPoint[];
}

// Strip the canonical column-type prefix when present; leave non-canonical
// names unchanged so users with quirky ledger naming see what they typed.
function stripColumnPrefix(name: string, columnLabel: 'Assets' | 'Liabilities'): string {
  const prefix = `${columnLabel}:`;
  return name.startsWith(prefix) ? name.slice(prefix.length) : name;
}

export function BalanceCard({ row, columnLabel, share, points }: Props) {
  const displayName = stripColumnPrefix(row.name, columnLabel);
  const shareWording = columnLabel.toLowerCase(); // 'assets' | 'liabilities'

  return (
    <Link
      to="/accounts/$id"
      params={{ id: String(row.account_id) }}
      search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
      className="flex h-full flex-col justify-between rounded-md border bg-card p-3 text-sm hover:bg-muted/40"
    >
      <div className="flex items-start justify-between gap-2">
        <span className="truncate text-xs text-muted-foreground" title={row.name}>
          {displayName}
        </span>
        <span className="shrink-0 rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold text-blue-800 dark:bg-blue-950 dark:text-blue-200">
          {row.currency}
        </span>
      </div>
      <div className="mt-2 flex items-end justify-between gap-2">
        <div className="min-w-0">
          <div className={cn('text-lg font-bold tabular-nums', balanceColor(row.type, row.amount))}>
            {formatBalanceAbs(row.amount)}
          </div>
          {share !== null && (
            <div className="mt-0.5 text-[11px] text-muted-foreground">
              {share}% of {shareWording}
            </div>
          )}
        </div>
        <div className="shrink-0">
          {points && points.length >= 2 && (
            <Sparkline
              points={points}
              strokeClassName={balanceStrokeColor(row.type, row.amount)}
            />
          )}
        </div>
      </div>
    </Link>
  );
}
```

- [ ] **Step 2: Verify type-check**

Run: `cd spa && npx tsc --noEmit`
Expected: no errors. (The next task updates the grid; the missing `points` prop is optional, so existing callers still compile.)

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/balances/BalanceCard.tsx
git commit -m "feat(spa): render sparkline slot on BalanceCard"
```

---

### Task 11: Thread `historyByAccount` through `BalanceCardGrid`

**Files:**
- Modify: `spa/src/components/balances/BalanceCardGrid.tsx`

- [ ] **Step 1: Update the file**

Replace the contents of `spa/src/components/balances/BalanceCardGrid.tsx` with:

```tsx
import { BalanceCard } from '@/components/balances/BalanceCard';
import { CARDS_PAGE_SIZE } from '@/components/balances/BalanceColumn';
import type { AccountBalance, BalanceHistoryPoint } from '@/lib/types';

interface Props {
  rows: AccountBalance[]; // already sorted and sliced by the parent
  shares: (number | null)[]; // aligned with rows, same length
  columnLabel: 'Assets' | 'Liabilities';
  historyByAccount?: Map<number, BalanceHistoryPoint[]>;
}

export function BalanceCardGrid({ rows, shares, columnLabel, historyByAccount }: Props) {
  const placeholderCount = CARDS_PAGE_SIZE - rows.length;
  return (
    <div className="grid grid-cols-2 gap-3 p-3">
      {rows.map((row, i) => (
        <BalanceCard
          key={row.account_id}
          row={row}
          columnLabel={columnLabel}
          share={shares[i] ?? null}
          points={historyByAccount?.get(row.account_id)}
        />
      ))}
      {Array.from({ length: placeholderCount }).map((_, i) => (
        <div
          // biome-ignore lint/suspicious/noArrayIndexKey: placeholder cards have no identity
          key={`placeholder-${i}`}
          aria-hidden="true"
          className="h-[112px] rounded-md"
        />
      ))}
    </div>
  );
}
```

- [ ] **Step 2: Verify**

Run: `cd spa && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/balances/BalanceCardGrid.tsx
git commit -m "feat(spa): thread history map through BalanceCardGrid"
```

---

### Task 12: Pass `historyByAccount` through `BalanceColumn`

**Files:**
- Modify: `spa/src/components/balances/BalanceColumn.tsx`

- [ ] **Step 1: Update the type import**

In `spa/src/components/balances/BalanceColumn.tsx`, change:

```ts
import type { AccountBalance } from '@/lib/types';
```

to:

```ts
import type { AccountBalance, BalanceHistoryPoint } from '@/lib/types';
```

- [ ] **Step 2: Add the new prop to the `Props` interface**

In the same file, add to the `Props` interface (after `view: 'list' | 'cards';`):

```ts
  historyByAccount?: Map<number, BalanceHistoryPoint[]>;
```

- [ ] **Step 3: Destructure it in the component args**

Update the `BalanceColumn` argument destructuring to include `historyByAccount`:

```tsx
export function BalanceColumn({
  label,
  total,
  rows,
  shares,
  totalRowCount,
  sortDir,
  onToggleSort,
  offset,
  onOffsetChange,
  emptyText,
  view,
  historyByAccount,
}: Props) {
```

- [ ] **Step 4: Pass it through to `BalanceCardGrid`**

In the cards branch of the JSX, change:

```tsx
<BalanceCardGrid rows={rows} shares={shares} columnLabel={label} />
```

to:

```tsx
<BalanceCardGrid
  rows={rows}
  shares={shares}
  columnLabel={label}
  historyByAccount={historyByAccount}
/>
```

(List-view path is not changed — the list rendering doesn't get a sparkline.)

- [ ] **Step 5: Verify**

Run: `cd spa && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/balances/BalanceColumn.tsx
git commit -m "feat(spa): thread history map through BalanceColumn"
```

---

### Task 13: Wire the route query and update the existing balances test

**Files:**
- Modify: `spa/src/routes/balances.tsx`
- Modify: `spa/src/test/balances.test.tsx`

- [ ] **Step 1: Update the existing `balances.test.tsx` fetch stub to handle the new URL**

In `spa/src/test/balances.test.tsx`, inside the `vi.stubGlobal('fetch', vi.fn(...))` block, add (above the final `throw`):

```ts
      if (url === '/api/balances/history') {
        return Promise.resolve(
          okResponse({
            items: [
              {
                account_id: 1,
                currency: 'USD',
                points: [
                  { month: '2024-01', balance: 100000 },
                  { month: '2024-02', balance: 125000 },
                ],
              },
            ],
          }),
        );
      }
```

- [ ] **Step 2: Run the existing balances tests — should still pass**

Run: `cd spa && npx vitest run balances`
Expected: existing tests still PASS (the new mock is benign for now since the route doesn't fetch it yet).

- [ ] **Step 3: Update `routes/balances.tsx` to fetch history and build the map**

In `spa/src/routes/balances.tsx`:

- Add to imports: `getBalanceHistory` from `@/lib/api`, and `BalanceHistoryPoint` from `@/lib/types`.
- Inside `BalancesPage`, after the existing `useQuery({ queryKey: ['balances'], ... })` call, add:

```ts
  const historyQuery = useQuery({
    queryKey: ['balance-history'],
    queryFn: getBalanceHistory,
  });

  const historyByAccount = useMemo(() => {
    const m = new Map<number, BalanceHistoryPoint[]>();
    if (historyQuery.data) {
      for (const s of historyQuery.data.items) {
        m.set(s.account_id, s.points);
      }
    }
    return m;
  }, [historyQuery.data]);
```

- Pass `historyByAccount={historyByAccount}` to both `<BalanceColumn ...>` calls.

(The history query is independent: page rendering keeps gating on the main balances query only. If `historyQuery` errors or is still pending, `historyByAccount` is empty — cards render without sparklines.)

- [ ] **Step 4: Type-check**

Run: `cd spa && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 5: Run all SPA tests**

Run: `cd spa && npm test`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add spa/src/routes/balances.tsx spa/src/test/balances.test.tsx
git commit -m "feat(spa): fetch balance history and pass to balance columns"
```

---

### Task 14: End-to-end verification

**Files:** none — verification only.

- [ ] **Step 1: Full Go build + tests**

Run: `go build ./... && go test ./...`
Expected: build succeeds; all tests PASS.

- [ ] **Step 2: Full SPA build + tests + lint**

Run: `cd spa && npm test && npx tsc --noEmit && npx biome check src`
Expected: all PASS.

- [ ] **Step 3: Manual visual smoke test**

Run: `make run` in one terminal, open the SPA at the printed URL, navigate to `/balances`, switch to the **cards** view. Confirm:

- Each card with ≥2 months of history shows a small line at its bottom-right corner.
- Card height has grown but the grid layout still looks coherent.
- Stroke colors match the amount color (positive green for assets going up, etc.).
- An account with only one month of history (e.g., a brand-new account) shows no sparkline and the card still renders correctly.

If anything looks off, fix and re-run the full test suite before continuing.

- [ ] **Step 4: Final summary commit (only if needed)**

If any minor adjustments were made during the smoke test, commit them with a descriptive message. Otherwise skip.

```bash
git status
```

---

## Out of Scope (for follow-up plans)

- Sparkline on the list view.
- Sparkline on the account detail page.
- Daily granularity, hover tooltips, configurable time ranges.
- Cross-account shared y-axis, cross-currency comparison.
- Snapshot/cache table for monthly balances (only needed if perf becomes an issue).
- Color-by-segment (e.g., red below zero, green above).
