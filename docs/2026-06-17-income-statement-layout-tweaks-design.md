# Income Statement Layout Tweaks

## Goal

Tighten the `/reports/income-statement` page:

1. Show only the top 5 rows in the Income mix and Expense mix bars (down from 8).
2. Remove the Income detail and Expense detail row tables from this page (the standalone `/reports/income-breakdown` and `/reports/expense-breakdown` routes are unaffected).
3. Show a prior-period diff on the Income, Expense, and Net KPI cards.

## Files affected

- `spa/src/routes/reports.income-statement.tsx` — main page wiring
- `spa/src/lib/period.ts` — new `previousPeriod` helper
- `spa/src/lib/hooks/useReport.ts` — already exposes `useIncomeStatement`; no new hook needed (callsite passes a different `params` object)
- `spa/src/components/reports/KpiCard.tsx` — new optional `diff` prop
- `spa/src/test/period.test.ts` — new file, `previousPeriod` cases
- `spa/src/test/reports.kpi-card.test.tsx` — diff prop cases
- `spa/src/test/reports.income-statement.test.tsx` — assert layout changes
- Existing `ReportRowTable` component and its test stay (still used by breakdown routes)

## 1. Mix bar limit

Change `limit={8}` → `limit={5}` on both `ProportionBar` calls in [reports.income-statement.tsx](spa/src/routes/reports.income-statement.tsx). The existing "+ N more" expand button is unchanged.

## 2. Remove detail tables

Delete the second `grid grid-cols-2 gap-6` block containing the two Income/Expense detail `ReportRowTable` sections.

Resulting dead code to also remove from `reports.income-statement.tsx`:

- `balancesQuery` (line 31)
- `nameToId` memo (lines 32–38)
- Imports: `ReportRowTable`, `getBalances`

(`useQuery` import stays — section 3.2 reuses it for the previous-period fetch.)

The Currency footer stays.

## 3. Prior-period diff on KPI cards

### 3.1 `previousPeriod` helper

Add to `spa/src/lib/period.ts`:

```ts
export function previousPeriod(
  search: PeriodSearch,
  resolved: ResolvedPeriod,
  nowUnix: number = Date.now() / 1000,
): { apiParams: PeriodApiParams } | null
```

Returns `null` when no sensible prior window exists (shouldn't happen for the four canonical ranges, but guards custom edge cases). Otherwise returns API params suitable for `fetchIncomeStatement`.

Rules per range:

| Current range | Previous range |
|---|---|
| `this-month` (Y-M) | previous calendar month (handles Jan → Dec of prior year) |
| `last-month` | the month before that |
| `ytd` (Jan 1 → today of year Y) | `from=(Y-1)-01-01, to=(Y-1)-MM-DD` (same MM-DD as today) |
| `last-12mo` (12 months ending today) | the 12 months ending one day before current `startUnix` |
| `custom from..to` | same-length window ending one day before `from`. Length = `to - from` in days. |

For `this-month` / `last-month` / previous-of-`ytd`, return `{ month }` or `{ from, to }` matching the existing API shape.

### 3.2 Data fetch

In `reports.income-statement.tsx`:

```ts
const previous = useMemo(() => previousPeriod(search, period), [search, period]);
const previousQuery = useQuery({
  queryKey: ['reports', 'income-statement', previous?.apiParams],
  queryFn: () => fetchIncomeStatement(previous!.apiParams),
  enabled: previous !== null,
});
```

The previous query is **supplementary**: if it's loading or errored, KPI cards render without a diff. No spinner, no error UI. The main page's existing pending/error states only key off the primary `query`.

### 3.3 `KpiCard` `diff` prop

```ts
interface DiffInfo {
  delta: number;            // current - previous, in cents
  prevAmount: number;       // for percent calc; if 0, percent is omitted
  goodWhen: 'up' | 'down';  // semantic direction
}

interface Props {
  // ...existing...
  diff?: DiffInfo;
}
```

Render as an additional sub-line below `subLine` (so the Net card shows both net-worth growth AND the period-over-period diff):

- Arrow: `▲` when `delta > 0`, `▼` when `delta < 0`, `—` when `delta === 0`
- Format: `{arrow} {±formatCents(delta, currency)}{ (±X.X%) if prevAmount !== 0} vs last period`
- Color:
  - `delta === 0` → muted
  - sign matches `goodWhen` → green (`text-green-700 dark:text-green-400`)
  - sign opposes `goodWhen` → red (`text-red-700 dark:text-red-400`)
- Test id: `data-testid="kpi-diff"`

Sign convention: "good" means `delta > 0 && goodWhen === 'up'` OR `delta < 0 && goodWhen === 'down'`.

Percent is computed as `(delta / Math.abs(prevAmount)) * 100`, displayed with one decimal, sign included. Skipped entirely when `prevAmount === 0`.

### 3.4 Wiring per card

```ts
const prevIncome  = previousQuery.data?.total_income[currency]  ?? 0;
const prevExpense = previousQuery.data?.total_expense[currency] ?? 0;
const prevNet     = previousQuery.data?.net_amount[currency]    ?? 0;
const hasPrev     = previousQuery.isSuccess;

<KpiCard ... diff={hasPrev ? { delta: income - prevIncome, prevAmount: prevIncome, goodWhen: 'up' }   : undefined} />
<KpiCard ... diff={hasPrev ? { delta: expense - prevExpense, prevAmount: prevExpense, goodWhen: 'down' } : undefined} />
<KpiCard ... subLine={netSubLine} diff={hasPrev ? { delta: net - prevNet, prevAmount: prevNet, goodWhen: 'up' } : undefined} />
```

Note `expense` and `prevExpense` are positive cents (totals are reported as positive magnitudes in this codebase — verified against existing usage at [reports.income-statement.tsx:81](spa/src/routes/reports.income-statement.tsx:81)). `delta > 0` for expense means expense grew, which `goodWhen: 'down'` correctly colors red.

## Testing

### `period.test.ts` (new)

Cases for `previousPeriod`:

- `this-month` 2026-06 → `{ month: '2026-05' }`
- `this-month` 2026-01 → `{ month: '2025-12' }` (year rollover)
- `last-month` (`nowUnix` in 2026-06) → previous calendar month two back, i.e. `{ month: '2026-04' }`
- `ytd` with `nowUnix=2026-06-17` → `{ from: '2025-01-01', to: '2025-06-17' }`
- `last-12mo` with `nowUnix=2026-06-17` → 12 months ending one day before current window's start
- `custom from=2026-03-01,to=2026-03-31` → `{ from: '2026-01-29', to: '2026-02-28' }` (same length, ending one day before `from`)

### `reports.kpi-card.test.tsx` (updated)

- Renders diff line with `▲` arrow and green text when `delta > 0` and `goodWhen='up'`
- Renders diff line with `▲` arrow and red text when `delta > 0` and `goodWhen='down'`
- Renders diff line with `▼` arrow and red text when `delta < 0` and `goodWhen='up'`
- Renders diff line with `—` and muted text when `delta === 0`
- Omits percent when `prevAmount === 0`
- Coexists with `subLine` — both render
- No `data-testid="kpi-diff"` element when `diff` prop omitted

### `reports.income-statement.test.tsx` (updated)

- "Income detail" and "Expense detail" headings no longer present
- `ReportRowTable` not rendered on this page (assert absence by test-id or heading)
- ProportionBar shows at most 5 rows initially
- When mock fixture has previous-period data, KPI cards render `data-testid="kpi-diff"`
- When previous-period query is pending or errored, KPI cards render without `data-testid="kpi-diff"`

## Out of scope

- The standalone `/reports/income-breakdown` and `/reports/expense-breakdown` routes
- Changing the `CurrencyFooter`
- Any backend / API changes
- Persisting "expanded" state across navigation
