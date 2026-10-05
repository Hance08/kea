# SPA Reports — Merge Net Worth into Balance Sheet Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate the Reports page so Net Worth is no longer its own tab. Add a Net Worth KPI card plus a daily net-worth line chart to the Balance Sheet page; remove the standalone Net Worth route; back the chart with a new daily-series backend endpoint.

**Architecture:** Backend exposes `GET /api/reports/net-worth-series` returning daily, front-filled net-worth points per currency. Frontend Balance Sheet page renders a 4-column KPI grid (Assets / Liabilities / Equity / Net Worth), a hand-rolled SVG line chart, and the Liabilities table only. The dedicated `/reports/net-worth` SPA route is deleted; the URL redirects to `/reports/balance-sheet`.

**Tech Stack:** Go (chi router, existing service/store pattern), React 18 + TypeScript, TanStack Router + Query, Tailwind CSS, Vitest + Testing Library. No new dependencies.

Spec: `docs/superpowers/specs/2026-06-21-spa-reports-balance-sheet-net-worth-merge-design.md`

---

## File Map

**Backend — create:**
- `internal/model/net_worth_series.go` — `DailyBalancePoint`, `CurrencyDailySeries` types.

**Backend — modify:**
- `internal/service/report_service.go` — add `GetDailyNetWorthSeries(ctx)`.
- `internal/service/report_service_test.go` — tests for the new service method.
- `internal/api/reports.go` — add `handleNetWorthSeries`.
- `internal/api/reports_test.go` — tests for the new handler.
- `internal/api/router.go` — register `GET /api/reports/net-worth-series`.

**Frontend — create:**
- `spa/src/components/reports/NetWorthChart.tsx` — SVG line chart.
- `spa/src/test/reports.net-worth-chart.test.tsx` — chart tests.

**Frontend — modify:**
- `spa/src/lib/types.ts` — add `DailyBalancePoint`, `CurrencyDailySeries`, `NetWorthSeriesResponse`.
- `spa/src/lib/api/reports.ts` — add `fetchNetWorthSeries`; remove `fetchNetWorth`.
- `spa/src/lib/hooks/useReport.ts` — add `useNetWorthSeries`; remove `useNetWorth`.
- `spa/src/components/reports/TabNav.tsx` — drop the Net Worth entry.
- `spa/src/routes/reports.balance-sheet.tsx` — new KPI grid, chart section, removed sections.
- `spa/src/routes/reports.index.tsx` — leave alone (it redirects `/reports/` already).
- `spa/src/test/reports.balance-sheet.test.tsx` — update assertions.
- `spa/src/test/reports.tab-nav.test.tsx` — remove Net Worth assertion.

**Frontend — delete:**
- `spa/src/routes/reports.net-worth.tsx`
- `spa/src/test/reports.net-worth.test.tsx`
- `spa/src/components/NetWorthCard.tsx` (only if no other consumer remains — verify)

**Frontend — create (redirect):**
- `spa/src/routes/reports.net-worth.tsx` — replaced with a redirect-only route to `/reports/balance-sheet` (preserves bookmarks).

---

## Task 1: Add daily net-worth series model

**Files:**
- Create: `internal/model/net_worth_series.go`

- [ ] **Step 1: Create the model file**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// DailyBalancePoint is a single end-of-day net-worth value, in cents.
type DailyBalancePoint struct {
	Date    string `json:"date"`    // "YYYY-MM-DD" (UTC)
	Balance int64  `json:"balance"` // cents
}

// CurrencyDailySeries is the full daily net-worth history for one currency.
// Points are ordered ascending by date and dense (one per day from the first
// activity day through today) — gap days are front-filled from the prior day.
type CurrencyDailySeries struct {
	Currency string              `json:"currency"`
	Points   []DailyBalancePoint `json:"points"`
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/model/...`
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/model/net_worth_series.go
git commit -m "feat(model): add DailyBalancePoint and CurrencyDailySeries"
```

---

## Task 2: Service method — GetDailyNetWorthSeries (failing tests)

**Files:**
- Modify: `internal/service/report_service_test.go`

- [ ] **Step 1: Append the new test block to the bottom of the test file**

Add after the existing `TestGetNetWorthAt` block:

```go
// ──────────────────────────────────────────────
// GetDailyNetWorthSeries
// ──────────────────────────────────────────────

func TestGetDailyNetWorthSeries(t *testing.T) {
	// 2026-01-01 00:00:00 UTC and 2026-01-03 00:00:00 UTC.
	day1 := int64(1767225600)
	day3 := day1 + 2*86400

	t.Run("empty ledger returns empty slice", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		out, err := svc.GetDailyNetWorthSeries(context.Background())
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("single-day single-currency emits one point", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.txs = append(txRepo.txs, &model.Transaction{ID: 1, Timestamp: day1})
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		out, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day1)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, "USD", out[0].Currency)
		require.Len(t, out[0].Points, 1)
		assert.Equal(t, "2026-01-01", out[0].Points[0].Date)
		assert.Equal(t, int64(10000), out[0].Points[0].Balance)
	})

	t.Run("front-fills days without activity", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.txs = append(txRepo.txs,
			&model.Transaction{ID: 1, Timestamp: day1},
			&model.Transaction{ID: 2, Timestamp: day3},
		)
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
		)
		addTxSplits(txRepo.splitsWithAccts, 2,
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 5000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -5000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		out, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day3)
		require.NoError(t, err)
		require.Len(t, out, 1)
		pts := out[0].Points
		require.Len(t, pts, 3)
		assert.Equal(t, "2026-01-01", pts[0].Date)
		assert.Equal(t, int64(10000), pts[0].Balance)
		assert.Equal(t, "2026-01-02", pts[1].Date)
		assert.Equal(t, int64(10000), pts[1].Balance) // front-filled
		assert.Equal(t, "2026-01-03", pts[2].Date)
		assert.Equal(t, int64(15000), pts[2].Balance)
	})

	t.Run("multi-currency keeps series separate", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.txs = append(txRepo.txs, &model.Transaction{ID: 1, Timestamp: day1})
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:USD_Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Assets:TWD_Bank", AccountType: model.AccountTypeAsset, Amount: 50000, Currency: "TWD"},
			model.SplitDetail{AccountName: "Equity:Opening_USD", AccountType: model.AccountTypeEquity, Amount: -10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening_TWD", AccountType: model.AccountTypeEquity, Amount: -50000, Currency: "TWD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		out, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day1)
		require.NoError(t, err)
		require.Len(t, out, 2)
		bySeries := map[string]int64{}
		for _, s := range out {
			require.Len(t, s.Points, 1)
			bySeries[s.Currency] = s.Points[0].Balance
		}
		assert.Equal(t, int64(10000), bySeries["USD"])
		assert.Equal(t, int64(50000), bySeries["TWD"])
	})

	t.Run("excludes income and expense splits", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.txs = append(txRepo.txs, &model.Transaction{ID: 1, Timestamp: day1})
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500, Currency: "USD"},
			model.SplitDetail{AccountName: "Revenue:Salary", AccountType: model.AccountTypeRevenue, Amount: -500, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		out, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day1)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("liability reduces net worth", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.txs = append(txRepo.txs, &model.Transaction{ID: 1, Timestamp: day1})
		addTxSplits(txRepo.splitsWithAccts, 1,
			model.SplitDetail{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 10000, Currency: "USD"},
			model.SplitDetail{AccountName: "Liabilities:Card", AccountType: model.AccountTypeLiability, Amount: -3000, Currency: "USD"},
			model.SplitDetail{AccountName: "Equity:Opening", AccountType: model.AccountTypeEquity, Amount: -7000, Currency: "USD"},
		)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		out, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day1)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, int64(7000), out[0].Points[0].Balance)
	})

	t.Run("repo failure returns error", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.splitsRangeErr = assert.AnError
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		_, err := svc.GetDailyNetWorthSeriesUntil(context.Background(), day1)
		assert.Error(t, err)
	})
}
```

Note: `GetDailyNetWorthSeriesUntil` is an internal helper that pins "today" to a fixed timestamp so tests are deterministic. The exported `GetDailyNetWorthSeries(ctx)` calls it with `time.Now().Unix()`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestGetDailyNetWorthSeries`
Expected: FAIL — `svc.GetDailyNetWorthSeriesUntil undefined` and `svc.GetDailyNetWorthSeries undefined`.

- [ ] **Step 3: Verify the existing mockTransactionRepo already supports `txs`**

Run: `grep -n "^	txs " /Users/hance/programming/kea/internal/service/testhelper_test.go`
Expected: a `txs []*model.Transaction` field. If absent, you must extend the mock — but the existing income-statement tests already use it. Do NOT add it again if already present.

- [ ] **Step 4: Commit**

```bash
git add internal/service/report_service_test.go
git commit -m "test(service): add failing tests for GetDailyNetWorthSeries"
```

---

## Task 3: Service method — GetDailyNetWorthSeries implementation

**Files:**
- Modify: `internal/service/report_service.go`

- [ ] **Step 1: Add the implementation at the bottom of `report_service.go`, above the closing helpers**

Add at the end of the file:

```go
// GetDailyNetWorthSeries returns daily net-worth points across all currencies,
// dense from the earliest transaction day through today (UTC), front-filled
// over days without activity. Income/expense splits are excluded.
func (ts *TransactionService) GetDailyNetWorthSeries(ctx context.Context) ([]model.CurrencyDailySeries, error) {
	return ts.GetDailyNetWorthSeriesUntil(ctx, time.Now().Unix())
}

// GetDailyNetWorthSeriesUntil is the deterministic variant used by tests; it
// pins the upper bound of the series to a fixed Unix timestamp.
func (ts *TransactionService) GetDailyNetWorthSeriesUntil(ctx context.Context, until int64) ([]model.CurrencyDailySeries, error) {
	txs, err := ts.txRepo.GetTransactionsByDateRange(ctx, 0, until)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}
	if len(txs) == 0 {
		return []model.CurrencyDailySeries{}, nil
	}

	splitsByTx, err := ts.txRepo.GetSplitsWithAccountsByDateRange(ctx, 0, until)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch splits: %w", err)
	}

	// Bucket day deltas per currency: map[currency]map[YYYY-MM-DD]int64.
	deltas := map[string]map[string]int64{}
	firstDay := ""
	for _, tx := range txs {
		day := time.Unix(tx.Timestamp, 0).UTC().Format("2006-01-02")
		if firstDay == "" || day < firstDay {
			firstDay = day
		}
		for _, sp := range splitsByTx[tx.ID] {
			if sp.AccountType != model.AccountTypeAsset && sp.AccountType != model.AccountTypeLiability {
				continue
			}
			byDay, ok := deltas[sp.Currency]
			if !ok {
				byDay = map[string]int64{}
				deltas[sp.Currency] = byDay
			}
			byDay[day] += sp.Amount
		}
	}
	if len(deltas) == 0 {
		return []model.CurrencyDailySeries{}, nil
	}

	lastDay := time.Unix(until, 0).UTC().Format("2006-01-02")
	currencies := make([]string, 0, len(deltas))
	for c := range deltas {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)

	out := make([]model.CurrencyDailySeries, 0, len(currencies))
	for _, ccy := range currencies {
		byDay := deltas[ccy]
		var points []model.DailyBalancePoint
		var running int64
		day, _ := time.Parse("2006-01-02", firstDay)
		end, _ := time.Parse("2006-01-02", lastDay)
		for !day.After(end) {
			d := day.Format("2006-01-02")
			running += byDay[d]
			points = append(points, model.DailyBalancePoint{Date: d, Balance: running})
			day = day.Add(24 * time.Hour)
		}
		// Skip series that never moved off zero.
		nonZero := false
		for _, p := range points {
			if p.Balance != 0 {
				nonZero = true
				break
			}
		}
		if !nonZero {
			continue
		}
		out = append(out, model.CurrencyDailySeries{Currency: ccy, Points: points})
	}
	return out, nil
}
```

- [ ] **Step 2: Run the new tests**

Run: `go test ./internal/service/ -run TestGetDailyNetWorthSeries -v`
Expected: PASS (all 7 sub-tests).

- [ ] **Step 3: Run the full service test suite to confirm no regressions**

Run: `go test ./internal/service/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/service/report_service.go
git commit -m "feat(service): add GetDailyNetWorthSeries with front-filled daily points"
```

---

## Task 4: HTTP handler — handleNetWorthSeries (failing test)

**Files:**
- Modify: `internal/api/reports_test.go`

- [ ] **Step 1: Inspect the existing handler-test style**

Run: `grep -n "handleBalanceHistory\|TestBalanceHistory" /Users/hance/programming/kea/internal/api/reports_test.go`
Read the surrounding test for `handleBalanceHistory` to mirror its structure (response decoding, server harness).

- [ ] **Step 2: Add a new test `TestHandleNetWorthSeries` to `internal/api/reports_test.go`**

Place after the existing balance-history test. Mirror the same `newTestServer(t, …)` harness used elsewhere in that file:

```go
func TestHandleNetWorthSeries(t *testing.T) {
	srv, _ := newTestServer(t)
	day := int64(1767225600) // 2026-01-01 UTC
	mustCreateTransaction(t, srv, day,
		split{name: "Assets:Bank", amount: 10000, currency: "USD"},
		split{name: "Equity:Opening", amount: -10000, currency: "USD"},
	)

	rr := doGET(t, srv, "/api/reports/net-worth-series")
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Items []model.CurrencyDailySeries `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Currency != "USD" {
		t.Fatalf("unexpected items: %+v", resp.Items)
	}
	if len(resp.Items[0].Points) < 1 || resp.Items[0].Points[0].Balance != 10000 {
		t.Fatalf("unexpected points: %+v", resp.Items[0].Points)
	}
}
```

Note: If `mustCreateTransaction`, `split`, and `doGET` are not the exact helper names used in `reports_test.go`, **read the existing tests first** and adapt to the actual helpers (e.g., the file may use a builder type or seed via store directly). The test must use the same seeding mechanism the existing tests use — do not invent new helpers.

- [ ] **Step 3: Run the new test to verify it fails**

Run: `go test ./internal/api/ -run TestHandleNetWorthSeries -v`
Expected: FAIL — most likely `404 Not Found` because the route isn't registered yet, or a compile error if the handler/route don't exist.

- [ ] **Step 4: Commit**

```bash
git add internal/api/reports_test.go
git commit -m "test(api): add failing test for net-worth-series endpoint"
```

---

## Task 5: HTTP handler — implementation and route

**Files:**
- Modify: `internal/api/reports.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Add the handler to `internal/api/reports.go`**

Append at the bottom of the file:

```go
type netWorthSeriesResponse struct {
	Items []model.CurrencyDailySeries `json:"items"`
}

func (s *Server) handleNetWorthSeries(w http.ResponseWriter, r *http.Request) error {
	items, err := s.svc.Transaction().GetDailyNetWorthSeries(r.Context())
	if err != nil {
		return err
	}
	if items == nil {
		items = []model.CurrencyDailySeries{}
	}
	return writeJSON(w, http.StatusOK, netWorthSeriesResponse{Items: items})
}
```

- [ ] **Step 2: Register the route in `internal/api/router.go`**

In `internal/api/router.go`, immediately below the existing line `r.Method(http.MethodGet, "/reports/net-worth", apiHandler(s.handleNetWorth))`, add:

```go
			r.Method(http.MethodGet, "/reports/net-worth-series", apiHandler(s.handleNetWorthSeries))
```

(Match the indentation of the surrounding lines exactly — tab-indented inside the `r.Route("/api", …)` closure.)

- [ ] **Step 3: Run the new handler test**

Run: `go test ./internal/api/ -run TestHandleNetWorthSeries -v`
Expected: PASS.

- [ ] **Step 4: Run the full API test suite**

Run: `go test ./internal/api/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/reports.go internal/api/router.go
git commit -m "feat(api): add GET /api/reports/net-worth-series endpoint"
```

---

## Task 6: Frontend types and API client

**Files:**
- Modify: `spa/src/lib/types.ts`
- Modify: `spa/src/lib/api/reports.ts`

- [ ] **Step 1: Add the new types after `BalanceHistoryResponse` in `spa/src/lib/types.ts`**

Insert below the existing `BalanceHistoryResponse` interface (around line 154):

```ts
export interface DailyBalancePoint {
  date: string; // "YYYY-MM-DD"
  balance: number; // cents
}

export interface CurrencyDailySeries {
  currency: string;
  points: DailyBalancePoint[];
}

export interface NetWorthSeriesResponse {
  items: CurrencyDailySeries[];
}
```

Leave `NetWorthResponse` in place — the backend endpoint stays; only the client wrapper for it is removed in a later task.

- [ ] **Step 2: Update `spa/src/lib/api/reports.ts`**

Replace the file contents with:

```ts
import { apiFetch } from '../api';
import type { BalanceSheetResult, NetWorthSeriesResponse, ReportResult } from '../types';

function buildQuery(params: { [k: string]: string | number | undefined }): string {
  const usp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '') continue;
    usp.set(k, String(v));
  }
  const s = usp.toString();
  return s ? `?${s}` : '';
}

export interface PeriodApiParams {
  [k: string]: string | undefined;
  month?: string;
  from?: string;
  to?: string;
}

export function fetchIncomeStatement(params: PeriodApiParams): Promise<ReportResult> {
  return apiFetch<ReportResult>(`/api/reports/income-statement${buildQuery(params)}`);
}

export function fetchIncomeBreakdown(params: PeriodApiParams): Promise<ReportResult> {
  return apiFetch<ReportResult>(`/api/reports/income-breakdown${buildQuery(params)}`);
}

export function fetchExpenseBreakdown(params: PeriodApiParams): Promise<ReportResult> {
  return apiFetch<ReportResult>(`/api/reports/expense-breakdown${buildQuery(params)}`);
}

export function fetchBalanceSheet(params: {
  [k: string]: number | undefined;
  as_of?: number;
}): Promise<BalanceSheetResult> {
  return apiFetch<BalanceSheetResult>(`/api/reports/balance-sheet${buildQuery(params)}`);
}

export function fetchNetWorthSeries(): Promise<NetWorthSeriesResponse> {
  return apiFetch<NetWorthSeriesResponse>('/api/reports/net-worth-series');
}
```

(`fetchNetWorth` is removed; `NetWorthResponse` type is no longer imported here.)

- [ ] **Step 3: Update `spa/src/lib/hooks/useReport.ts`**

Replace its contents with:

```ts
import { useQuery } from '@tanstack/react-query';
import {
  type PeriodApiParams,
  fetchBalanceSheet,
  fetchExpenseBreakdown,
  fetchIncomeBreakdown,
  fetchIncomeStatement,
  fetchNetWorthSeries,
} from '../api/reports';

export function useIncomeStatement(params: PeriodApiParams) {
  return useQuery({
    queryKey: ['reports', 'income-statement', params],
    queryFn: () => fetchIncomeStatement(params),
  });
}

export function useIncomeBreakdown(params: PeriodApiParams) {
  return useQuery({
    queryKey: ['reports', 'income-breakdown', params],
    queryFn: () => fetchIncomeBreakdown(params),
  });
}

export function useExpenseBreakdown(params: PeriodApiParams) {
  return useQuery({
    queryKey: ['reports', 'expense-breakdown', params],
    queryFn: () => fetchExpenseBreakdown(params),
  });
}

export function useBalanceSheet(params: { as_of?: number }) {
  return useQuery({
    queryKey: ['reports', 'balance-sheet', params],
    queryFn: () => fetchBalanceSheet(params),
  });
}

export function useNetWorthSeries() {
  return useQuery({
    queryKey: ['reports', 'net-worth-series'],
    queryFn: fetchNetWorthSeries,
  });
}
```

- [ ] **Step 4: Type-check the SPA**

Run: `cd spa && npx tsc -b`
Expected: ERRORS — referencing `NetWorthResponse`/`useNetWorth`/`fetchNetWorth` from `routes/reports.net-worth.tsx`. This is expected; Task 8 deletes that file. Move on; do not chase these errors yet.

If there are unrelated TS errors (i.e., not from `routes/reports.net-worth.tsx`), fix them now.

- [ ] **Step 5: Commit**

```bash
git add spa/src/lib/types.ts spa/src/lib/api/reports.ts spa/src/lib/hooks/useReport.ts
git commit -m "feat(spa): add net-worth-series client and remove single-point net-worth hook"
```

---

## Task 7: NetWorthChart component (failing test)

**Files:**
- Create: `spa/src/test/reports.net-worth-chart.test.tsx`

- [ ] **Step 1: Create the test file**

```tsx
import { render, screen } from '@testing-library/react';
import { describe, expect, test } from 'vitest';
import { NetWorthChart } from '@/components/reports/NetWorthChart';

const formatUSD = (cents: number) =>
  `$${(cents / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;

describe('NetWorthChart', () => {
  test('renders nothing when fewer than 2 points', () => {
    const { container } = render(
      <NetWorthChart
        points={[{ date: '2026-01-01', balance: 100 }]}
        currency="USD"
        formatCents={formatUSD}
      />,
    );
    expect(container.querySelector('svg')).toBeNull();
  });

  test('renders an svg with polyline and min/max axis labels', () => {
    const points = [
      { date: '2026-01-01', balance: 100_00 },
      { date: '2026-01-02', balance: 150_00 },
      { date: '2026-01-03', balance: 200_00 },
    ];
    render(<NetWorthChart points={points} currency="USD" formatCents={formatUSD} />);

    const svg = screen.getByRole('img', { name: /net worth/i });
    expect(svg).toBeInTheDocument();
    expect(svg.querySelector('polyline')).not.toBeNull();
    expect(screen.getByText(formatUSD(100_00))).toBeInTheDocument();
    expect(screen.getByText(formatUSD(200_00))).toBeInTheDocument();
    expect(screen.getByText('2026-01-01')).toBeInTheDocument();
    expect(screen.getByText('2026-01-03')).toBeInTheDocument();
  });

  test('renders a marker circle at the asOfDate', () => {
    const points = [
      { date: '2026-01-01', balance: 100 },
      { date: '2026-01-02', balance: 200 },
      { date: '2026-01-03', balance: 300 },
    ];
    const { container } = render(
      <NetWorthChart
        points={points}
        currency="USD"
        formatCents={formatUSD}
        asOfDate="2026-01-02"
      />,
    );
    const circles = container.querySelectorAll('circle');
    expect(circles.length).toBeGreaterThanOrEqual(1);
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd spa && npx vitest run src/test/reports.net-worth-chart.test.tsx`
Expected: FAIL — `Cannot find module '@/components/reports/NetWorthChart'`.

- [ ] **Step 3: Commit**

```bash
git add spa/src/test/reports.net-worth-chart.test.tsx
git commit -m "test(spa): add failing tests for NetWorthChart"
```

---

## Task 8: NetWorthChart component (implementation)

**Files:**
- Create: `spa/src/components/reports/NetWorthChart.tsx`

- [ ] **Step 1: Create the component**

```tsx
import { cn } from '@/lib/cn';
import type { DailyBalancePoint } from '@/lib/types';

interface Props {
  points: DailyBalancePoint[];
  currency: string;
  formatCents: (cents: number) => string;
  asOfDate?: string; // "YYYY-MM-DD"
  className?: string;
}

const VIEWBOX_W = 600;
const VIEWBOX_H = 180;
const PAD_LEFT = 56;
const PAD_RIGHT = 12;
const PAD_TOP = 12;
const PAD_BOTTOM = 28;

export function NetWorthChart({ points, currency, formatCents, asOfDate, className }: Props) {
  if (points.length < 2) return null;

  const balances = points.map((p) => p.balance);
  const min = Math.min(...balances);
  const max = Math.max(...balances);
  const range = max - min || 1;

  const drawW = VIEWBOX_W - PAD_LEFT - PAD_RIGHT;
  const drawH = VIEWBOX_H - PAD_TOP - PAD_BOTTOM;
  const xStep = drawW / (points.length - 1);

  const xy = points.map((p, i) => {
    const x = PAD_LEFT + i * xStep;
    const y = PAD_TOP + drawH - ((p.balance - min) / range) * drawH;
    return { x, y, point: p };
  });

  const polylinePoints = xy.map((c) => `${c.x},${c.y}`).join(' ');
  const baselineY = PAD_TOP + drawH;
  const areaPoints = `${PAD_LEFT},${baselineY} ${polylinePoints} ${PAD_LEFT + (points.length - 1) * xStep},${baselineY}`;

  const firstDate = points[0].date;
  const midDate = points[Math.floor(points.length / 2)].date;
  const lastDate = points[points.length - 1].date;

  let markerIdx: number | null = null;
  if (asOfDate) {
    let best = -1;
    for (let i = 0; i < points.length; i++) {
      if (points[i].date <= asOfDate) best = i;
      else break;
    }
    if (best >= 0) markerIdx = best;
  }

  return (
    <svg
      role="img"
      aria-label={`Net worth over time, ${currency}`}
      viewBox={`0 0 ${VIEWBOX_W} ${VIEWBOX_H}`}
      preserveAspectRatio="none"
      className={cn('h-48 w-full', className)}
    >
      <polygon points={areaPoints} className="fill-primary/10" />
      <polyline
        points={polylinePoints}
        fill="none"
        strokeWidth={1.5}
        vectorEffect="non-scaling-stroke"
        className="stroke-primary"
      />

      {/* Y-axis labels */}
      <text
        x={PAD_LEFT - 6}
        y={PAD_TOP + 4}
        textAnchor="end"
        className="fill-muted-foreground text-[10px]"
      >
        {formatCents(max)}
      </text>
      <text
        x={PAD_LEFT - 6}
        y={baselineY}
        textAnchor="end"
        className="fill-muted-foreground text-[10px]"
      >
        {formatCents(min)}
      </text>

      {/* X-axis labels */}
      <text
        x={PAD_LEFT}
        y={VIEWBOX_H - 8}
        textAnchor="start"
        className="fill-muted-foreground text-[10px]"
      >
        {firstDate}
      </text>
      <text
        x={PAD_LEFT + drawW / 2}
        y={VIEWBOX_H - 8}
        textAnchor="middle"
        className="fill-muted-foreground text-[10px]"
      >
        {midDate}
      </text>
      <text
        x={PAD_LEFT + drawW}
        y={VIEWBOX_H - 8}
        textAnchor="end"
        className="fill-muted-foreground text-[10px]"
      >
        {lastDate}
      </text>

      {markerIdx !== null && (
        <>
          <line
            x1={xy[markerIdx].x}
            x2={xy[markerIdx].x}
            y1={PAD_TOP}
            y2={baselineY}
            strokeDasharray="3 3"
            vectorEffect="non-scaling-stroke"
            className="stroke-muted-foreground/40"
          />
          <circle
            cx={xy[markerIdx].x}
            cy={xy[markerIdx].y}
            r={3}
            className="fill-primary"
          />
        </>
      )}
    </svg>
  );
}
```

- [ ] **Step 2: Run the chart test**

Run: `cd spa && npx vitest run src/test/reports.net-worth-chart.test.tsx`
Expected: PASS (all three tests).

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/reports/NetWorthChart.tsx
git commit -m "feat(spa): add NetWorthChart SVG line chart component"
```

---

## Task 9: Balance Sheet page — update KPI grid, drop sections, add chart

**Files:**
- Modify: `spa/src/routes/reports.balance-sheet.tsx`

- [ ] **Step 1: Replace `spa/src/routes/reports.balance-sheet.tsx` with the new layout**

```tsx
import { AsOfPicker } from '@/components/reports/AsOfPicker';
import { CurrencyFooter } from '@/components/reports/CurrencyFooter';
import { KpiCard } from '@/components/reports/KpiCard';
import { NetWorthChart } from '@/components/reports/NetWorthChart';
import { ReportRowTable } from '@/components/reports/ReportRowTable';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getBalances } from '@/lib/api';
import { useBalanceSheet, useNetWorthSeries } from '@/lib/hooks/useReport';
import { type AsOfSearchParams, parseAsOfSearch } from '@/lib/reports-search-params';
import { useAmountFormat, useServerConfig } from '@/lib/server-config';
import { useQuery } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useMemo } from 'react';

export const Route = createFileRoute('/reports/balance-sheet')({
  validateSearch: (s): AsOfSearchParams => parseAsOfSearch(s),
  component: BalanceSheetPage,
});

function BalanceSheetPage() {
  const { defaults } = useServerConfig();
  const { formatCents } = useAmountFormat();
  const currency = defaults.currency;
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/reports/balance-sheet' });
  const query = useBalanceSheet(search);
  const seriesQuery = useNetWorthSeries();

  const balancesQuery = useQuery({ queryKey: ['balances'], queryFn: getBalances });
  const nameToId = useMemo(() => {
    const m = new Map<string, number>();
    if (balancesQuery.data)
      for (const row of balancesQuery.data.items) m.set(row.name, row.account_id);
    return m;
  }, [balancesQuery.data]);

  const setAsOf = (v: number | undefined) =>
    navigate({ search: () => (v === undefined ? {} : { as_of: v }) });

  if (query.isPending) {
    return (
      <div className="space-y-4">
        <AsOfPicker label="As of" value={search.as_of} onChange={setAsOf} />
        <Skeleton className="h-20" />
        <Skeleton className="h-48" />
      </div>
    );
  }
  if (query.isError) {
    return (
      <div className="space-y-3">
        <AsOfPicker label="As of" value={search.as_of} onChange={setAsOf} />
        <Alert variant="destructive">
          <AlertTitle>Failed to load balance sheet</AlertTitle>
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

  const result = query.data;
  const ta = result.total_assets[currency] ?? 0;
  const tl = result.total_liabilities[currency] ?? 0;
  const te = result.total_equity[currency] ?? 0;
  const nw = result.net_worth[currency] ?? 0;
  const liabilities = (result.liabilities ?? []).filter((r) => r.currency === currency);

  const assetsCount = (result.assets ?? []).filter((r) => r.currency === currency).length;
  const equityCount = (result.equity ?? []).filter((r) => r.currency === currency).length;

  const seriesForCurrency = seriesQuery.data?.items.find((s) => s.currency === currency);
  const asOfDate =
    search.as_of !== undefined
      ? new Date(search.as_of * 1000).toISOString().slice(0, 10)
      : undefined;
  const chartPoints = seriesForCurrency
    ? asOfDate
      ? seriesForCurrency.points.filter((p) => p.date <= asOfDate)
      : seriesForCurrency.points
    : [];

  if (assetsCount === 0 && liabilities.length === 0 && equityCount === 0) {
    return (
      <div className="space-y-4">
        <AsOfPicker label="As of" value={search.as_of} onChange={setAsOf} />
        <p className="text-sm text-muted-foreground">No balance-sheet activity at this date.</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <AsOfPicker label="As of" value={search.as_of} onChange={setAsOf} />

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {assetsCount > 0 && (
          <KpiCard label="Total Assets" amount={ta} currency={currency} variant="green" />
        )}
        {liabilities.length > 0 && (
          <KpiCard label="Total Liabilities" amount={tl} currency={currency} variant="red" />
        )}
        {equityCount > 0 && (
          <KpiCard label="Total Equity" amount={te} currency={currency} variant="neutral" />
        )}
        <KpiCard label="Net Worth" amount={nw} currency={currency} variant="neutral" />
      </div>

      <section>
        <h2 className="mb-2 text-sm font-semibold">Net worth over time</h2>
        {seriesQuery.isPending ? (
          <Skeleton className="h-48 w-full" />
        ) : seriesQuery.isError ? (
          <Alert variant="destructive">
            <AlertTitle>Failed to load net worth history</AlertTitle>
            <AlertDescription className="mt-2 space-y-3">
              <div>
                {seriesQuery.error instanceof Error ? seriesQuery.error.message : 'Unknown error'}
              </div>
              <Button onClick={() => seriesQuery.refetch()} size="sm">
                Retry
              </Button>
            </AlertDescription>
          </Alert>
        ) : chartPoints.length < 2 ? (
          <p className="text-sm text-muted-foreground">Not enough history to chart net worth.</p>
        ) : (
          <NetWorthChart
            points={chartPoints}
            currency={currency}
            formatCents={(c) => formatCents(c, currency)}
            asOfDate={asOfDate}
          />
        )}
      </section>

      {liabilities.length > 0 && (
        <section>
          <h2 className="mb-2 text-sm font-semibold">Liabilities</h2>
          <ReportRowTable
            rows={liabilities}
            currency={currency}
            nameToId={nameToId}
            period={null}
          />
        </section>
      )}

      <CurrencyFooter
        defaultCurrency={currency}
        entries={[
          { label: 'Assets', byCurrency: result.total_assets },
          { label: 'Liabilities', byCurrency: result.total_liabilities },
          { label: 'Equity', byCurrency: result.total_equity },
          { label: 'Net worth', byCurrency: result.net_worth },
        ]}
      />
    </div>
  );
}
```

- [ ] **Step 2: Update the existing Balance Sheet test**

Replace `spa/src/test/reports.balance-sheet.test.tsx` with:

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config')
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      if (url === '/api/ledgers')
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      if (url === '/api/balances')
        return Promise.resolve(
          okResponse({
            items: [
              {
                account_id: 1,
                name: 'Assets:Bank',
                type: 'A',
                currency: 'USD',
                amount: 125000,
                is_hidden: false,
              },
            ],
            total_count: 1,
            limit: 0,
            offset: 0,
          }),
        );
      if (url.startsWith('/api/reports/balance-sheet'))
        return Promise.resolve(
          okResponse({
            assets: [
              {
                account_name: 'Assets:Bank',
                offset_account: '',
                amount: 125000,
                currency: 'USD',
                tx_count: 4,
              },
            ],
            liabilities: [],
            equity: [
              {
                account_name: 'Equity:OpeningBalances_USD',
                offset_account: '',
                amount: 125000,
                currency: 'USD',
                tx_count: 1,
              },
            ],
            total_assets: { USD: 125000 },
            total_liabilities: {},
            total_equity: { USD: 125000 },
            net_worth: { USD: 125000 },
            as_of: 1781697600,
          }),
        );
      if (url === '/api/reports/net-worth-series')
        return Promise.resolve(
          okResponse({
            items: [
              {
                currency: 'USD',
                points: [
                  { date: '2026-01-01', balance: 50000 },
                  { date: '2026-01-02', balance: 100000 },
                  { date: '2026-01-03', balance: 125000 },
                ],
              },
            ],
          }),
        );
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('renders 4-column KPI grid with Net Worth and hides empty Liabilities card', async () => {
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    expect(screen.getByText('Total Assets')).toBeInTheDocument();
  });
  expect(screen.getByText('Total Equity')).toBeInTheDocument();
  expect(screen.getByText('Net Worth')).toBeInTheDocument();
  expect(screen.queryByText('Total Liabilities')).not.toBeInTheDocument();
});

test('removes Asset mix, Assets, and Equity sections', async () => {
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    expect(screen.getByText('Total Assets')).toBeInTheDocument();
  });
  expect(screen.queryByRole('heading', { name: 'Asset mix' })).toBeNull();
  expect(screen.queryByRole('heading', { name: 'Assets' })).toBeNull();
  expect(screen.queryByRole('heading', { name: 'Equity' })).toBeNull();
});

test('renders the Net worth over time chart heading', async () => {
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /net worth over time/i })).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run the balance-sheet tests**

Run: `cd spa && npx vitest run src/test/reports.balance-sheet.test.tsx`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add spa/src/routes/reports.balance-sheet.tsx spa/src/test/reports.balance-sheet.test.tsx
git commit -m "feat(spa): rework Balance Sheet page with Net Worth card and chart"
```

---

## Task 10: Drop Net Worth tab, redirect old route, delete dead files

**Files:**
- Modify: `spa/src/components/reports/TabNav.tsx`
- Modify: `spa/src/test/reports.tab-nav.test.tsx`
- Replace: `spa/src/routes/reports.net-worth.tsx` (becomes a redirect)
- Delete: `spa/src/test/reports.net-worth.test.tsx`

- [ ] **Step 1: Remove the Net Worth entry from `TabNav.tsx`**

Edit `spa/src/components/reports/TabNav.tsx`. Change the `TABS` array to:

```tsx
const TABS = [
  { to: '/reports/income-statement', label: 'Income Statement' },
  { to: '/reports/income-breakdown', label: 'Income Breakdown' },
  { to: '/reports/expense-breakdown', label: 'Expense Breakdown' },
  { to: '/reports/balance-sheet', label: 'Balance Sheet' },
] as const;
```

- [ ] **Step 2: Update `spa/src/test/reports.tab-nav.test.tsx`**

Read the file first, then remove or update assertions that mention Net Worth.

Run: `grep -n "Net Worth\|net-worth" /Users/hance/programming/kea/spa/src/test/reports.tab-nav.test.tsx`

For each match, either remove the assertion line (if it asserts presence) or replace it with `expect(screen.queryByText('Net Worth')).toBeNull()`.

If unsure which transform applies to a specific line, read 5 lines of surrounding context and decide. Do not leave orphan comments.

- [ ] **Step 3: Replace `spa/src/routes/reports.net-worth.tsx` with a redirect**

Replace the file contents with:

```tsx
import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/reports/net-worth')({
  beforeLoad: () => {
    throw redirect({ to: '/reports/balance-sheet', search: {} });
  },
});
```

- [ ] **Step 4: Delete the old Net Worth tests**

```bash
rm spa/src/test/reports.net-worth.test.tsx
```

- [ ] **Step 5: Check whether `NetWorthCard.tsx` is still used**

Run: `grep -rn "NetWorthCard" /Users/hance/programming/kea/spa/src`
Expected: zero matches.

If zero matches, delete it:

```bash
rm spa/src/components/NetWorthCard.tsx
```

If there are matches outside `NetWorthCard.tsx` itself, leave it in place.

- [ ] **Step 6: Type-check the SPA**

Run: `cd spa && npx tsc -b`
Expected: PASS (no errors). Any remaining errors are bugs in the rewrite — fix them before continuing.

- [ ] **Step 7: Run the full SPA test suite**

Run: `cd spa && npx vitest run`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add -A spa/src
git commit -m "refactor(spa): remove Net Worth tab and route, redirect for bookmarks"
```

---

## Task 11: End-to-end verification

**Files:**
- None (verification only)

- [ ] **Step 1: Build the backend**

Run: `make build`
Expected: `./kea_test` binary created.

- [ ] **Step 2: Run all Go tests**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 3: Run all SPA tests**

Run: `cd spa && npx vitest run`
Expected: PASS.

- [ ] **Step 4: Type-check + lint the SPA**

Run: `cd spa && npx tsc -b && npx biome check src`
Expected: no errors.

- [ ] **Step 5: Manual smoke check (only if dev server is set up locally)**

Run the SPA dev server and verify in the browser:
- `/reports` redirects to Income Statement (unchanged behaviour).
- The Net Worth tab is gone from the report tab strip.
- Visiting `/reports/net-worth` directly redirects to `/reports/balance-sheet`.
- The Balance Sheet page shows four KPI cards (Assets, Liabilities, Equity, Net Worth) and a Net worth over time chart with a marker at the As Of date.
- The Liabilities table still renders; Asset mix, Assets, and Equity tables are gone.

Skip this step if the user is not requesting visual verification. The vitest suite already exercises the rendered structure.

- [ ] **Step 6: Final commit (only if any fixups landed)**

If steps 1-5 produced any fix commits already, this step is a no-op.

---

## Self-Review Notes

- **Spec coverage:** New endpoint (Task 1-5), Balance Sheet KPI grid + chart (Task 6-9), Net Worth tab/route removal + redirect (Task 10). All four spec sections (Backend, Frontend, Migration, Risks) have corresponding tasks.
- **Type consistency:** `DailyBalancePoint` / `CurrencyDailySeries` use the same field names (`date`, `balance`, `currency`, `points`) on both sides of the wire. `NetWorthChart` props match its test usage. `useNetWorthSeries` returns the shape consumed by the Balance Sheet route.
- **No placeholders:** all code blocks are complete; no "TBD" or "add validation" handwaving.
- **Risks called out in spec:** Sub-day asof drift is acceptable per the spec ("chart is illustrative; KPI card is authoritative") — no special handling required.

---

Plan complete and saved to `docs/superpowers/plans/2026-06-21-spa-reports-balance-sheet-net-worth-merge.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
