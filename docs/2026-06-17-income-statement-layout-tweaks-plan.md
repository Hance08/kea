# Income Statement Layout Tweaks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Trim the `/reports/income-statement` page — top-5 mix bars, remove detail tables, and add prior-period diff lines on the Income / Expense / Net KPI cards.

**Architecture:** Pure SPA-side change. The diff is computed in the page by issuing a second `useQuery` against the existing `/api/reports/income-statement` endpoint with previous-period params; the prior-window is computed by a new `previousPeriod` helper in `lib/period.ts`. `KpiCard` gains an optional `diff` prop; the previous-period fetch is supplementary (no spinner / error UI). No backend changes.

**Tech Stack:** React, TypeScript, TanStack Query, TanStack Router, Tailwind, Vitest, Testing Library.

**Reference spec:** [docs/2026-06-17-income-statement-layout-tweaks-design.md](docs/2026-06-17-income-statement-layout-tweaks-design.md)

**Commands:**
- All SPA tests: `cd spa && npm test`
- Single SPA test file: `cd spa && npx vitest run src/test/<name>.test.tsx`
- Watch mode: `cd spa && npm run test:watch`
- Build (type-check): `cd spa && npm run build`

---

## File Structure

**Modified:**
- `spa/src/lib/period.ts` — add `previousPeriod` helper
- `spa/src/components/reports/KpiCard.tsx` — add optional `diff` prop and rendering
- `spa/src/routes/reports.income-statement.tsx` — limit→5, remove detail tables, add previous-period query, wire diffs
- `spa/src/test/reports.kpi-card.test.tsx` — new test cases for `diff`
- `spa/src/test/reports.income-statement.test.tsx` — assert detail tables removed; assert diff lines render; drop drill-down test (links lived in detail table)

**Created:**
- `spa/src/test/period.test.ts` — covers `previousPeriod` (new file; no existing `period` test)

**Not touched (deliberately):**
- `spa/src/components/reports/ProportionBar.tsx` — already accepts `limit` and has tests
- `spa/src/components/reports/ReportRowTable.tsx` — still used by `/reports/income-breakdown` and `/reports/expense-breakdown`
- API client `spa/src/lib/api/reports.ts` and hooks in `spa/src/lib/hooks/useReport.ts` — reused as-is
- All non-SPA Go code

---

### Task 1: Mix bar `limit` 8 → 5

**Files:**
- Modify: `spa/src/routes/reports.income-statement.tsx` (the two `<ProportionBar>` calls)

- [ ] **Step 1: Change `limit` on both `ProportionBar` calls**

In `spa/src/routes/reports.income-statement.tsx`, find the income/expense mix block (currently around lines 112–133):

```tsx
<ProportionBar
  rows={incomeRows}
  total={income}
  currency={currency}
  limit={8}
  variant="income"
/>
```

```tsx
<ProportionBar
  rows={expenseRows}
  total={expense}
  currency={currency}
  limit={8}
  variant="expense"
/>
```

Change both `limit={8}` to `limit={5}`.

- [ ] **Step 2: Run the existing income-statement integration test**

```bash
cd spa && npx vitest run src/test/reports.income-statement.test.tsx
```

Expected: PASS (current fixture has only 1 income row and 2 expense rows, so the limit change is invisible to it).

- [ ] **Step 3: Commit**

```bash
git add spa/src/routes/reports.income-statement.tsx
git commit -m "feat(spa): cap income statement mix bars at top 5"
```

---

### Task 2: Remove "Income detail" / "Expense detail" tables

**Files:**
- Modify: `spa/src/routes/reports.income-statement.tsx`
- Modify: `spa/src/test/reports.income-statement.test.tsx`

The detail tables and their drill-down links are going away. The standalone `/reports/income-breakdown` and `/reports/expense-breakdown` routes are unaffected.

- [ ] **Step 1: Update integration test to expect detail tables gone**

In `spa/src/test/reports.income-statement.test.tsx`:

1. **Drop the drill-down test** (it asserts a `link` for `Expenses:Rent`, which only existed inside the detail table). Delete the entire test starting at:
   ```tsx
   test('drill-down row links to /transactions with account_id and time bounds', async () => {
   ```
   and ending at the closing `});` of that test.

2. **Tighten the first test** — replace the `expect(screen.getAllByText('Expenses:Rent').length).toBeGreaterThan(0);` and `expect(screen.getAllByText('Income:Salary').length).toBeGreaterThan(0);` assertions with:
   ```tsx
   // Row labels appear in the mix bars (not in any detail table).
   expect(screen.getByText('Expenses:Rent')).toBeInTheDocument();
   expect(screen.getByText('Income:Salary')).toBeInTheDocument();
   // No detail-table headings.
   expect(screen.queryByRole('heading', { name: /Income detail/i })).toBeNull();
   expect(screen.queryByRole('heading', { name: /Expense detail/i })).toBeNull();
   // No drill-down links to /transactions.
   expect(screen.queryByRole('link', { name: /Expenses:Rent/ })).toBeNull();
   ```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd spa && npx vitest run src/test/reports.income-statement.test.tsx
```

Expected: FAIL — the headings "Income detail" / "Expense detail" still render and the `/transactions` link is still present.

- [ ] **Step 3: Delete the detail-table block from the page**

In `spa/src/routes/reports.income-statement.tsx`:

Delete the entire JSX block (currently lines 135–154):
```tsx
<div className="grid grid-cols-2 gap-6">
  <section>
    <h2 className="mb-2 text-sm font-semibold">Income detail</h2>
    <ReportRowTable
      rows={incomeRows}
      currency={currency}
      nameToId={nameToId}
      period={{ startUnix: period.startUnix, endUnix: period.endUnix }}
    />
  </section>
  <section>
    <h2 className="mb-2 text-sm font-semibold">Expense detail</h2>
    <ReportRowTable
      rows={expenseRows}
      currency={currency}
      nameToId={nameToId}
      period={{ startUnix: period.startUnix, endUnix: period.endUnix }}
    />
  </section>
</div>
```

Also delete the now-dead bindings:

- The `balancesQuery` declaration:
  ```tsx
  const balancesQuery = useQuery({ queryKey: ['balances'], queryFn: getBalances });
  ```
- The `nameToId` memo (lines 32–38):
  ```tsx
  const nameToId = useMemo(() => {
    const m = new Map<string, number>();
    if (balancesQuery.data) {
      for (const row of balancesQuery.data.items) m.set(row.name, row.account_id);
    }
    return m;
  }, [balancesQuery.data]);
  ```

Update imports — remove only:
- `ReportRowTable` (from `@/components/reports/ReportRowTable`)
- `getBalances` (from `@/lib/api`)

Keep `useQuery` and `useMemo` — they will be re-used in Task 5 and by the still-present `period` memo.

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd spa && npx vitest run src/test/reports.income-statement.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Full SPA test + build**

```bash
cd spa && npm test
cd spa && npm run build
```

Expected: PASS (and no TypeScript errors from unused imports).

- [ ] **Step 6: Commit**

```bash
git add spa/src/routes/reports.income-statement.tsx spa/src/test/reports.income-statement.test.tsx
git commit -m "feat(spa): drop detail tables from income statement page"
```

---

### Task 3: `previousPeriod` helper in `lib/period.ts`

**Files:**
- Modify: `spa/src/lib/period.ts`
- Create: `spa/src/test/period.test.ts`

Compute the API params for the same-length window immediately preceding the current period.

- [ ] **Step 1: Write the failing tests**

Create `spa/src/test/period.test.ts`:

```ts
import { describe, expect, test } from 'vitest';
import { previousPeriod, resolvePeriod } from '../lib/period';

// Helper: build a `nowUnix` for a given UTC date.
function utc(year: number, month1to12: number, day: number): number {
  return Date.UTC(year, month1to12 - 1, day) / 1000;
}

describe('previousPeriod', () => {
  test('this-month: previous calendar month', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'this-month' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { month: '2026-05' },
    });
  });

  test('this-month: January rolls back into prior December', () => {
    const now = utc(2026, 1, 15);
    const search = { range: 'this-month' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { month: '2025-12' },
    });
  });

  test('this-month with explicit month string', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'this-month' as const, month: '2026-03' };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { month: '2026-02' },
    });
  });

  test('last-month: the month before last', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'last-month' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { month: '2026-04' },
    });
  });

  test('ytd: same MM-DD window in the previous year', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'ytd' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { from: '2025-01-01', to: '2025-06-17' },
    });
  });

  test('last-12mo: 12 months ending one day before current start', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'last-12mo' as const };
    const resolved = resolvePeriod(search, now);
    // current window starts at 2025-06-17; previous ends 2025-06-16, starts 2024-06-16.
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { from: '2024-06-16', to: '2025-06-16' },
    });
  });

  test('custom: same-length window ending one day before from', () => {
    const now = utc(2026, 6, 17);
    const search = {
      range: 'custom' as const,
      from: '2026-03-01',
      to: '2026-03-31',
    };
    const resolved = resolvePeriod(search, now);
    // March has 31 days; previous window is 31 days ending 2026-02-28 → starts 2026-01-29.
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { from: '2026-01-29', to: '2026-02-28' },
    });
  });

  test('custom with missing from/to returns null', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'custom' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toBeNull();
  });
});
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd spa && npx vitest run src/test/period.test.ts
```

Expected: FAIL — `previousPeriod` is not exported from `../lib/period`.

- [ ] **Step 3: Implement `previousPeriod`**

In `spa/src/lib/period.ts`, append below the existing `resolvePeriod` function:

```ts
import type { PeriodApiParams } from './api/reports';

export function previousPeriod(
  search: PeriodSearch,
  resolved: ResolvedPeriod,
  nowUnix: number = Date.now() / 1000,
): { apiParams: PeriodApiParams } | null {
  // this-month and last-month → step back exactly one calendar month
  // from the resolved month string ("YYYY-MM" via apiParams.month).
  if (search.range === 'this-month' || search.range === 'last-month') {
    const monthStr = resolved.apiParams.month;
    if (!monthStr) return null;
    const parsed = /^(\d{4})-(\d{2})$/.exec(monthStr);
    if (!parsed) return null;
    let y = Number(parsed[1]);
    let m = Number(parsed[2]) - 1;
    if (m === 0) {
      m = 12;
      y -= 1;
    }
    return { apiParams: { month: `${y}-${pad2(m)}` } };
  }

  if (search.range === 'ytd') {
    const now = new Date(nowUnix * 1000);
    const y = now.getUTCFullYear() - 1;
    const mm = pad2(now.getUTCMonth() + 1);
    const dd = pad2(now.getUTCDate());
    return { apiParams: { from: `${y}-01-01`, to: `${y}-${mm}-${dd}` } };
  }

  if (search.range === 'last-12mo' || search.range === 'custom') {
    if (!resolved.apiParams.from || !resolved.apiParams.to) return null;
    // Length includes both endpoints when measured in days.
    const lengthSec = resolved.endUnix - resolved.startUnix;
    // Previous window ends one second before current start; subtract length to get its start.
    const prevEnd = resolved.startUnix - 1;
    const prevStart = prevEnd - lengthSec;
    return {
      apiParams: {
        from: isoDate(prevStart),
        to: isoDate(prevEnd),
      },
    };
  }

  return null;
}
```

If `pad2` and `isoDate` are not exported at module scope (they are defined above as module-local in the existing file), no change is needed — the new function is in the same module.

Also add the import line at the top of the file (just below the existing imports/exports — keep adjacent):

```ts
import type { PeriodApiParams } from './api/reports';
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd spa && npx vitest run src/test/period.test.ts
```

Expected: PASS (all 8 cases).

If any of the `last-12mo` or `custom` boundary tests are off by a day or a second, double-check the rule:
- Resolved windows in `period.ts` represent the period as `[startUnix, endUnix]` where `endUnix` is 23:59:59 of the last day.
- `lengthSec = endUnix - startUnix` therefore measures one second short of `n * 86400` for an n-day window. The previous window is computed with the same convention so the helper returns matching ISO dates.

- [ ] **Step 5: Commit**

```bash
git add spa/src/lib/period.ts spa/src/test/period.test.ts
git commit -m "feat(spa): add previousPeriod helper for period.ts"
```

---

### Task 4: `KpiCard` `diff` prop

**Files:**
- Modify: `spa/src/components/reports/KpiCard.tsx`
- Modify: `spa/src/test/reports.kpi-card.test.tsx`

- [ ] **Step 1: Write the failing tests**

Append to `spa/src/test/reports.kpi-card.test.tsx`:

```tsx
test('diff: positive delta with goodWhen=up renders ▲, green color, amount, and percent', () => {
  const { container } = render(
    <KpiCard
      label="Income"
      amount={524800}
      currency="USD"
      variant="green"
      diff={{ delta: 120000, prevAmount: 404800, goodWhen: 'up' }}
    />,
  );
  const diff = container.querySelector('[data-testid="kpi-diff"]');
  expect(diff).not.toBeNull();
  expect(diff?.textContent).toContain('▲');
  expect(diff?.textContent).toContain('+$1,200.00');
  expect(diff?.textContent).toContain('+29.6%');
  expect(diff?.textContent).toContain('vs last period');
  expect(diff?.className).toContain('text-green');
});

test('diff: positive delta with goodWhen=down renders red (e.g. expense grew)', () => {
  const { container } = render(
    <KpiCard
      label="Expense"
      amount={253000}
      currency="USD"
      variant="red"
      diff={{ delta: 30000, prevAmount: 223000, goodWhen: 'down' }}
    />,
  );
  const diff = container.querySelector('[data-testid="kpi-diff"]');
  expect(diff?.className).toContain('text-red');
  expect(diff?.textContent).toContain('▲');
  expect(diff?.textContent).toContain('+$300.00');
});

test('diff: negative delta with goodWhen=up renders ▼ and red', () => {
  const { container } = render(
    <KpiCard
      label="Income"
      amount={400000}
      currency="USD"
      variant="green"
      diff={{ delta: -50000, prevAmount: 450000, goodWhen: 'up' }}
    />,
  );
  const diff = container.querySelector('[data-testid="kpi-diff"]');
  expect(diff?.className).toContain('text-red');
  expect(diff?.textContent).toContain('▼');
  expect(diff?.textContent).toContain('-$500.00');
  expect(diff?.textContent).toContain('-11.1%');
});

test('diff: zero delta renders em-dash and muted color, no percent', () => {
  const { container } = render(
    <KpiCard
      label="Income"
      amount={100000}
      currency="USD"
      variant="green"
      diff={{ delta: 0, prevAmount: 100000, goodWhen: 'up' }}
    />,
  );
  const diff = container.querySelector('[data-testid="kpi-diff"]');
  expect(diff?.textContent).toContain('—');
  expect(diff?.className).toContain('text-muted-foreground');
  expect(diff?.textContent).not.toMatch(/%/);
});

test('diff: omits percent when prevAmount is 0', () => {
  const { container } = render(
    <KpiCard
      label="Income"
      amount={500}
      currency="USD"
      variant="green"
      diff={{ delta: 500, prevAmount: 0, goodWhen: 'up' }}
    />,
  );
  const diff = container.querySelector('[data-testid="kpi-diff"]');
  expect(diff?.textContent).toContain('+$5.00');
  expect(diff?.textContent).not.toMatch(/%/);
});

test('diff: coexists with subLine — both render', () => {
  const { container } = render(
    <KpiCard
      label="Net"
      amount={271800}
      currency="USD"
      variant="neutral"
      subLine="▲ 6.2% net worth"
      diff={{ delta: 10000, prevAmount: 261800, goodWhen: 'up' }}
    />,
  );
  expect(container.querySelector('[data-testid="kpi-subline"]')).not.toBeNull();
  expect(container.querySelector('[data-testid="kpi-diff"]')).not.toBeNull();
});

test('diff: no kpi-diff element when diff prop omitted', () => {
  const { container } = render(
    <KpiCard label="Income" amount={524800} currency="USD" variant="green" />,
  );
  expect(container.querySelector('[data-testid="kpi-diff"]')).toBeNull();
});
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd spa && npx vitest run src/test/reports.kpi-card.test.tsx
```

Expected: FAIL — `diff` prop does not exist.

- [ ] **Step 3: Implement `diff` on `KpiCard`**

Replace `spa/src/components/reports/KpiCard.tsx` with:

```tsx
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';

export type KpiVariant = 'green' | 'red' | 'neutral';

export interface KpiDiff {
  delta: number;
  prevAmount: number;
  goodWhen: 'up' | 'down';
}

interface Props {
  label: string;
  amount: number;
  currency: string;
  variant: KpiVariant;
  subLine?: string;
  diff?: KpiDiff;
}

const VARIANT_CLASS: Record<KpiVariant, string> = {
  green: 'text-green-700 dark:text-green-400',
  red: 'text-red-700 dark:text-red-400',
  neutral: 'text-foreground',
};

const GOOD_CLASS = 'text-green-700 dark:text-green-400';
const BAD_CLASS = 'text-red-700 dark:text-red-400';
const NEUTRAL_DIFF_CLASS = 'text-muted-foreground';

function formatDiff(diff: KpiDiff, currency: string): { text: string; className: string } {
  if (diff.delta === 0) {
    return { text: '— no change vs last period', className: NEUTRAL_DIFF_CLASS };
  }
  const arrow = diff.delta > 0 ? '▲' : '▼';
  const sign = diff.delta > 0 ? '+' : '-';
  const absAmount = formatCents(Math.abs(diff.delta), currency);
  let pctPart = '';
  if (diff.prevAmount !== 0) {
    const pct = (diff.delta / Math.abs(diff.prevAmount)) * 100;
    const pctSign = pct > 0 ? '+' : pct < 0 ? '-' : '';
    pctPart = ` (${pctSign}${Math.abs(pct).toFixed(1)}%)`;
  }
  const isGood =
    (diff.delta > 0 && diff.goodWhen === 'up') ||
    (diff.delta < 0 && diff.goodWhen === 'down');
  return {
    text: `${arrow} ${sign}${absAmount}${pctPart} vs last period`,
    className: isGood ? GOOD_CLASS : BAD_CLASS,
  };
}

export function KpiCard({ label, amount, currency, variant, subLine, diff }: Props) {
  const diffRendered = diff ? formatDiff(diff, currency) : null;
  return (
    <div className="rounded-md border bg-card p-4">
      <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
      <div
        data-testid="kpi-amount"
        className={cn('mt-1 text-2xl font-semibold', VARIANT_CLASS[variant])}
      >
        {formatCents(amount, currency)}
      </div>
      {subLine && (
        <div data-testid="kpi-subline" className="mt-1 text-xs text-muted-foreground">
          {subLine}
        </div>
      )}
      {diffRendered && (
        <div data-testid="kpi-diff" className={cn('mt-1 text-xs', diffRendered.className)}>
          {diffRendered.text}
        </div>
      )}
    </div>
  );
}
```

Notes for the implementer:
- `formatCents` is at `@/lib/format`. It uses `Intl.NumberFormat` with `style: 'currency'`, so passing a positive cent value (e.g. `120000`) yields `$1,200.00`. Always pass `Math.abs(diff.delta)` and prepend the sign manually so the output is `+$1,200.00` / `-$500.00` rather than the locale's accounting `($500.00)` form.
- Percent calculation uses `Math.abs(prevAmount)` so the sign comes from `delta` alone — important when the previous expense total is positive but conceptually a "cost".
- The em-dash variant's full text is `"— no change vs last period"` (matches the test which only checks for `—`).

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd spa && npx vitest run src/test/reports.kpi-card.test.tsx
```

Expected: PASS (all original 5 + new 7 cases).

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/reports/KpiCard.tsx spa/src/test/reports.kpi-card.test.tsx
git commit -m "feat(spa): add diff prop to KpiCard"
```

---

### Task 5: Wire previous-period fetch and diffs into the page

**Files:**
- Modify: `spa/src/routes/reports.income-statement.tsx`
- Modify: `spa/src/test/reports.income-statement.test.tsx`

- [ ] **Step 1: Extend the integration test fixture and add diff assertions**

In `spa/src/test/reports.income-statement.test.tsx`, modify the `beforeEach` fetch mock so the `/api/reports/income-statement` branch differentiates by query string:

```tsx
const PREV_REPORT_PAYLOAD = {
  ...REPORT_PAYLOAD,
  period: 'May 2026',
  total_income: { USD: 400000 },   // prev income = $4,000.00; current = $5,248.00 → +$1,248.00 (+31.2%)
  total_expense: { USD: 300000 },  // prev expense = $3,000.00; current = $2,530.00 → -$470.00 (-15.7%)
  net_amount: { USD: 100000 },     // prev net = $1,000.00; current = $2,718.00 → +$1,718.00 (+171.8%)
};
```

Add it just below the existing `REPORT_PAYLOAD` constant.

Replace the `/api/reports/income-statement` branch in the default `beforeEach` fetch mock:

```tsx
if (url.startsWith('/api/reports/income-statement')) {
  // Default search for /reports/income-statement resolves to `month=YYYY-MM`
  // for the current month. The previous-period request uses the previous month.
  // Use the URL to distinguish: any URL whose month query is the test "now" month
  // returns current; everything else returns previous.
  if (url.includes('month=') && !url.includes('month=2026-05')) {
    return Promise.resolve(okResponse(REPORT_PAYLOAD));
  }
  return Promise.resolve(okResponse(PREV_REPORT_PAYLOAD));
}
```

> **Note for the implementer:** the tests don't currently freeze "now". `resolvePeriod` defaults to `Date.now()`. To make the differentiation deterministic regardless of when the test runs, use a substring match that distinguishes "any month param vs. a specific previous month" — adjust by switching to `vi.useFakeTimers().setSystemTime(new Date('2026-06-17T00:00:00Z'))` in `beforeEach` and `vi.useRealTimers()` in `afterEach`. The simpler implementation: freeze time, then current uses `month=2026-06` and previous uses `month=2026-05`. Replace the branch with:

```tsx
if (url.startsWith('/api/reports/income-statement')) {
  if (url.includes('month=2026-05')) {
    return Promise.resolve(okResponse(PREV_REPORT_PAYLOAD));
  }
  return Promise.resolve(okResponse(REPORT_PAYLOAD));
}
```

And at the top of `beforeEach`:
```tsx
vi.useFakeTimers({ shouldAdvanceTime: true });
vi.setSystemTime(new Date('2026-06-17T00:00:00Z'));
```

And in `afterEach`:
```tsx
vi.useRealTimers();
```

(`shouldAdvanceTime: true` keeps queueMicrotask / Promises working.)

Then add new assertions to the first test (the "renders KPI cards…" test):

```tsx
// Diff lines (current minus previous) appear on each KPI card.
await waitFor(() => {
  const diffs = document.querySelectorAll('[data-testid="kpi-diff"]');
  expect(diffs.length).toBe(3);
});
const diffTexts = Array.from(
  document.querySelectorAll('[data-testid="kpi-diff"]'),
).map((el) => el.textContent ?? '');
// Income: +$1,248.00 vs prev $4,000.00
expect(diffTexts.some((t) => t.includes('+$1,248.00'))).toBe(true);
// Expense: -$470.00 vs prev $3,000.00
expect(diffTexts.some((t) => t.includes('-$470.00'))).toBe(true);
// Net: +$1,718.00 vs prev $1,000.00
expect(diffTexts.some((t) => t.includes('+$1,718.00'))).toBe(true);
```

Add a new test for the no-previous-data path:

```tsx
test('omits diff lines when previous-period query is errored', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      }
      if (url === '/api/balances') {
        return Promise.resolve(okResponse({ items: [], total_count: 0, limit: 0, offset: 0 }));
      }
      if (url.startsWith('/api/reports/income-statement')) {
        if (url.includes('month=2026-05')) {
          return Promise.resolve(new Response('boom', { status: 500 }));
        }
        return Promise.resolve(okResponse(REPORT_PAYLOAD));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
  render(makeTestApp('/reports/income-statement'));
  await waitFor(() => {
    expect(screen.getByText('Income')).toBeInTheDocument();
  });
  // Give the previous-period query time to settle.
  await waitFor(() => {
    expect(document.querySelectorAll('[data-testid="kpi-diff"]').length).toBe(0);
  });
});
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
cd spa && npx vitest run src/test/reports.income-statement.test.tsx
```

Expected: FAIL — no `kpi-diff` elements render yet because the page does not fetch the previous period.

- [ ] **Step 3: Wire the previous-period query and pass `diff` to KPI cards**

In `spa/src/routes/reports.income-statement.tsx`:

1. Update imports at the top:
   ```tsx
   import { CurrencyFooter } from '@/components/reports/CurrencyFooter';
   import { KpiCard } from '@/components/reports/KpiCard';
   import { PeriodPicker } from '@/components/reports/PeriodPicker';
   import { ProportionBar } from '@/components/reports/ProportionBar';
   import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
   import { Button } from '@/components/ui/button';
   import { Skeleton } from '@/components/ui/skeleton';
   import { fetchIncomeStatement } from '@/lib/api/reports';
   import { useIncomeStatement } from '@/lib/hooks/useReport';
   import { previousPeriod, resolvePeriod } from '@/lib/period';
   import { type PeriodSearchParams, parsePeriodSearch } from '@/lib/reports-search-params';
   import { useServerConfig } from '@/lib/server-config';
   import { useQuery } from '@tanstack/react-query';
   import { createFileRoute, useNavigate } from '@tanstack/react-router';
   import { useMemo } from 'react';
   ```

2. After the existing `const query = useIncomeStatement(period.apiParams);` line, add:

   ```tsx
   const previous = useMemo(() => previousPeriod(search, period), [search, period]);
   const previousQuery = useQuery({
     queryKey: ['reports', 'income-statement', 'previous', previous?.apiParams],
     queryFn: () => fetchIncomeStatement(previous!.apiParams),
     enabled: previous !== null,
   });
   ```

3. Replace the KPI card grid block with:

   ```tsx
   <div className="grid grid-cols-3 gap-3">
     <KpiCard
       label="Income"
       amount={income}
       currency={currency}
       variant="green"
       diff={
         previousQuery.isSuccess
           ? {
               delta: income - (previousQuery.data.total_income[currency] ?? 0),
               prevAmount: previousQuery.data.total_income[currency] ?? 0,
               goodWhen: 'up',
             }
           : undefined
       }
     />
     <KpiCard
       label="Expense"
       amount={expense}
       currency={currency}
       variant="red"
       diff={
         previousQuery.isSuccess
           ? {
               delta: expense - (previousQuery.data.total_expense[currency] ?? 0),
               prevAmount: previousQuery.data.total_expense[currency] ?? 0,
               goodWhen: 'down',
             }
           : undefined
       }
     />
     <KpiCard
       label="Net"
       amount={net}
       currency={currency}
       variant="neutral"
       subLine={netSubLine}
       diff={
         previousQuery.isSuccess
           ? {
               delta: net - (previousQuery.data.net_amount[currency] ?? 0),
               prevAmount: previousQuery.data.net_amount[currency] ?? 0,
               goodWhen: 'up',
             }
           : undefined
       }
     />
   </div>
   ```

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd spa && npx vitest run src/test/reports.income-statement.test.tsx
```

Expected: PASS — three `kpi-diff` elements with the expected amounts; the "errored previous" test sees zero `kpi-diff` elements.

- [ ] **Step 5: Full SPA test + build**

```bash
cd spa && npm test
cd spa && npm run build
```

Expected: PASS, no TypeScript errors.

- [ ] **Step 6: Manual smoke (optional but recommended for UI work)**

```bash
cd spa && npm run dev
```

Open the dev URL, navigate to `/reports/income-statement`, and confirm:
- Three KPI cards show a `▲`/`▼`/`—` diff line below them.
- Income card diff is green when income grew; red when it shrank.
- Expense card diff is red when expense grew; green when it shrank.
- Mix bars show at most 5 rows initially with "+ N more" if there are more accounts.
- The old "Income detail" / "Expense detail" tables are gone.
- The currency footer is still visible.

- [ ] **Step 7: Commit**

```bash
git add spa/src/routes/reports.income-statement.tsx spa/src/test/reports.income-statement.test.tsx
git commit -m "feat(spa): show prior-period diff on income statement KPI cards"
```

---

### Task 6: Final pass

- [ ] **Step 1: Full test + build from repo root**

```bash
cd spa && npm test && npm run build
cd .. && go test ./...
```

Expected: all green. Go tests are unrelated to this change but worth confirming nothing accidentally moved.

- [ ] **Step 2: `git log` review**

```bash
git log --oneline origin/master..HEAD
```

Expected (5 commits):
1. cap mix bars at top 5
2. drop detail tables
3. add `previousPeriod` helper
4. add `diff` prop to `KpiCard`
5. show prior-period diff on KPI cards

If commits look right, this plan is done.
