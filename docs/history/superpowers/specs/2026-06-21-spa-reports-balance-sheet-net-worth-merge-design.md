# SPA Reports — Merge Net Worth into Balance Sheet

## Goal

Consolidate the Reports page so that Net Worth is no longer its own tab. The Balance Sheet tab gains a Net Worth KPI card and a line chart showing daily net worth over the account's full history. The dedicated Net Worth tab/route is removed.

## Scope

### In scope

- New backend endpoint returning a daily net-worth time series across all available history.
- Balance Sheet page changes: KPI grid layout, removed sections, new chart section.
- New `NetWorthChart` SVG component (no new chart library).
- Removal of the `Net Worth` tab, route file, and its tests.

### Out of scope

- The existing point-in-time `GET /api/reports/net-worth` endpoint is kept (it has independent consumers via `useNetWorth`, and its removal is unrelated to the UI consolidation).
- Multi-currency net worth charting. The chart renders the default currency only, matching how the rest of the Balance Sheet KPIs behave.
- Interactive features beyond a static dot marker at `as_of` (no hover tooltips, no zoom).

## Backend

### New endpoint

`GET /api/reports/net-worth-series`

Returns daily net-worth points spanning from the earliest transaction date through today. One series per currency.

Response shape:

```json
{
  "items": [
    {
      "currency": "USD",
      "points": [
        { "date": "2025-01-15", "balance": 123456 },
        { "date": "2025-01-16", "balance": 124000 }
      ]
    }
  ]
}
```

- `date`: `YYYY-MM-DD` (UTC day).
- `balance`: net worth in cents (assets + liabilities, where liabilities are stored as negative-sign per existing convention).
- Days with no activity are **front-filled** from the prior day's balance so the series is dense (one point per day) and the chart renders without gaps.
- If a currency's series would be entirely zero, omit it.
- Empty array if there is no qualifying activity at all.

### Service layer

Add `TransactionService.GetDailyNetWorthSeries(ctx) ([]model.CurrencyDailySeries, error)`.

Implementation:

1. Find the earliest posted-transaction date and today's UTC date.
2. For each currency, walk day-by-day from earliest → today, accumulating delta from that day's asset + liability splits.
3. Emit one `DailyBalancePoint{Date, Balance}` per day per currency.

A new model file `internal/model/net_worth_series.go` adds:

```go
type DailyBalancePoint struct {
    Date    string `json:"date"`    // "YYYY-MM-DD" (UTC)
    Balance int64  `json:"balance"` // cents
}

type CurrencyDailySeries struct {
    Currency string              `json:"currency"`
    Points   []DailyBalancePoint `json:"points"`
}
```

Routing: register in `internal/api/router.go` alongside the existing reports routes.

Tests cover: empty ledger, single-currency single-day, multi-currency, days without activity (verify front-fill), and asset/liability sign handling.

## Frontend

### Removed

- `spa/src/routes/reports.net-worth.tsx` — route and component.
- `spa/src/test/reports.net-worth.test.tsx` — tests.
- `Net Worth` entry in `spa/src/components/reports/TabNav.tsx`.
- `useNetWorth` hook in `spa/src/lib/hooks/useReport.ts` (no remaining consumer once the tab is gone).
- `fetchNetWorth` in `spa/src/lib/api/reports.ts` (no remaining consumer).
- `NetWorthResponse` import from `spa/src/lib/types` if it becomes orphaned (verify; otherwise keep).

The standalone `NetWorthCard` component in `spa/src/components/NetWorthCard.tsx` is also removed if its only consumer is the deleted route (verify with a search before deleting).

### Balance Sheet page (`reports.balance-sheet.tsx`)

KPI grid:

- Change `grid-cols-3` to `grid-cols-2 md:grid-cols-4`.
- Add a fourth `KpiCard` titled **Net Worth** showing `nw` for the default currency, `variant="neutral"`.
- Remove the `subLine={`Net worth: …`}` from the Equity card — the standalone card replaces it.

Removed sections:

- **Asset mix** (the `ProportionBar` block).
- **Assets** table (`ReportRowTable` for assets).
- **Equity** table (`ReportRowTable` for equity).

Kept sections:

- **Liabilities** table.
- The currency footer (no change).

New section: **Net worth over time**

- Render below the KPI grid, above the Liabilities table.
- Heading: `<h2 className="mb-2 text-sm font-semibold">Net worth over time</h2>`.
- Render `<NetWorthChart …>` (component below).
- Data: `useQuery` against the new `/api/reports/net-worth-series` endpoint, filter to default currency, filter points to `point.date <= as_of` (or all if `as_of` is undefined).
- Loading: skeleton placeholder. Error: same `Alert` pattern used elsewhere on the page. Empty: muted-text fallback "Not enough history to chart net worth."

### `NetWorthChart` component

Path: `spa/src/components/reports/NetWorthChart.tsx`.

Props:

```ts
interface Props {
  points: { date: string; balance: number }[]; // ascending by date
  currency: string;
  asOfDate?: string; // YYYY-MM-DD; marker location
}
```

Rendering:

- SVG, `viewBox="0 0 600 180"`, `preserveAspectRatio="none"` for the line geometry but text rendered separately at fixed size.
- Layout: left padding 56px for Y-axis labels, right padding 12px, top 12px, bottom 28px for X-axis labels.
- Line: `<polyline>` of points scaled to drawing area. `vectorEffect="non-scaling-stroke"`, themed via Tailwind class (use `stroke-primary`).
- Area fill: `<polygon>` underneath line down to baseline using `fill-primary/10`.
- Y-axis: two labels — min and max balance — formatted via `useAmountFormat().formatCents`.
- X-axis: three labels — first date, midpoint date, last date — formatted as `YYYY-MM-DD`.
- `asOfDate` marker: locate matching point (or nearest preceding point); render a `<circle>` (r=3) and a vertical dashed line at that X.
- If `points.length < 2`, render nothing and let the parent show the empty fallback.

The component uses no client-side animation and no new dependency — same approach as `Sparkline.tsx`.

### Tests

- `reports.balance-sheet.test.tsx`: update to assert the 4-card grid, presence of the Net Worth card, presence of the chart heading, absence of `Asset mix` / `Assets` / `Equity` headings.
- New `reports.net-worth-chart.test.tsx`: render with 0/1/many points, currency formatting in axis labels, marker presence when `asOfDate` is given.
- Delete `reports.net-worth.test.tsx`.
- Update `reports.tab-nav.test.tsx` to assert `Net Worth` tab is no longer rendered.

## Migration / Compatibility

- The deleted route `/reports/net-worth` becomes a 404 in the SPA. If we want graceful handling for bookmarked URLs, add a redirect from `/reports/net-worth` → `/reports/balance-sheet` in `reports.index.tsx`-style fashion. **Decision: add the redirect** — cheap and prevents broken links.
- The `/api/reports/net-worth` HTTP endpoint stays; no external clients are documented but removing it is out of scope and unrelated to UI consolidation.

## Risks

- **Large series performance**: a ledger with several years of history yields ~1500+ points per currency. SVG with that many `<polyline>` points still renders fine; no down-sampling is needed at this scale.
- **UTC day boundaries**: net worth at the end of day X may differ from "as of timestamp X" used by the existing KPI computation. The chart marker will display the daily-aggregated balance, which may differ by sub-day movements. Acceptable; the chart is illustrative, the KPI card is authoritative.
