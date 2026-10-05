# Balance Card Sparkline Design

## Summary

Add a small line chart (sparkline) to the bottom-right of each balance card in the cards view on `/balances`. The line shows the account's full balance history at monthly granularity — its own min-to-max scale, no axes, no tooltip — so the user can read the long-term **shape** of each account at a glance. A new batched server endpoint `GET /api/balances/history` returns end-of-month balances for every Asset and Liability account.

## Motivation

The cards view today communicates current balance and share of the column total. Two accounts with the same balance can have very different stories: one may be steadily growing, one may have just received a one-off deposit, one may be slowly drawn down. The cards view doesn't show this. A sparkline per card surfaces the long-term shape — direction, smoothness, accumulation patterns — without taking the user off the page.

## User-Visible Changes

- Each card in cards view grows from ~88px to ~112px tall and gains a small sparkline in its bottom-right corner (~80×24px).
- The amount and the `<n>% of <assets|liabilities>` line stay on the left side of the bottom row; the sparkline sits to their right.
- The sparkline's stroke color matches the rule already used for the amount: `balanceColor(row.type, row.amount)` — positive green, negative red, zero default.
- The y-axis is each line's own min-to-max range. The chart shows shape, not absolute value.
- An account whose history has fewer than 2 monthly points renders no line — the slot is left empty so the card layout stays consistent.
- While the history is still loading, or if it failed to load, the slot is also left empty. The card stays usable; there is no spinner and no error UI on the card.
- List view is unchanged. The account detail page is unchanged. The sparkline is a cards-view-only enhancement for now.

## Backend

### New endpoint

`GET /api/balances/history`

No path params, no query params. Granularity is fixed to monthly; the response is the full available history per account.

Response shape:

```json
{
  "items": [
    {
      "account_id": 12,
      "currency": "USD",
      "points": [
        { "month": "2023-01", "balance": 125000 },
        { "month": "2023-02", "balance": 130000 }
      ]
    }
  ]
}
```

- `month` is `YYYY-MM` (ISO year-month, UTC).
- `balance` is end-of-month balance in cents (`int64`), already in the account's **natural amount** per [CLAUDE.md](../../../CLAUDE.md) (assets: debits add; liabilities: credits add). The curve looks like the natural-amount curve, matching what the rest of the Balances page shows.
- Only **Asset (`A`)** and **Liability (`L`)** accounts appear. Equity, Revenue, and Expense accounts are omitted — same scope as the balance cards themselves.
- The first point per account is the earliest month with activity. The history is **not** front-filled with zero months before the account's first transaction; the line naturally starts where the account starts.
- An account with only one month of activity returns `points` of length 1 (the frontend will render no line in that case).
- An account with no activity at all is omitted from `items`.

### Service & repository changes

A new method is added to `TransactionService`:

```go
GetMonthlyBalanceHistory(ctx context.Context) ([]model.AccountMonthlySeries, error)
```

The corresponding new type lives in `internal/model`:

```go
type AccountMonthlySeries struct {
    AccountID int64
    Currency  string
    Points    []MonthlyBalancePoint
}

type MonthlyBalancePoint struct {
    Month   string // "YYYY-MM"
    Balance int64  // cents, natural-amount
}
```

A new method is added to `TransactionRepository`:

```go
GetMonthlySplitTotalsForAssetsAndLiabilities(ctx context.Context) ([]MonthlySplitTotal, error)
```

returning, for every (account_id, month) with activity, the sum of split amounts in that month for accounts of type `A` or `L`. The store implementation is one SQL pass:

```sql
SELECT s.account_id, a.type, a.currency,
       strftime('%Y-%m', t.date) AS month,
       SUM(s.amount) AS total
FROM splits s
JOIN transactions t ON t.id = s.transaction_id
JOIN accounts     a ON a.id = s.account_id
WHERE a.type IN ('A', 'L')
GROUP BY s.account_id, month
ORDER BY s.account_id, month;
```

The service then, in Go:

1. Buckets rows by `account_id`.
2. For each account, walks rows in ascending month order, maintaining a running cumulative sum to produce end-of-month balances.
3. Applies natural-amount sign per account type (asset = balance; liability = `-balance`), matching existing conventions in `internal/utils` / model helpers.
4. Skips accounts whose cumulative-sum series is entirely zero.

### Handler

A new handler in `internal/api/`:

```go
func (s *Server) handleBalanceHistory(w http.ResponseWriter, r *http.Request) *apiError { ... }
```

Wired in [router.go](../../../internal/api/router.go) as:

```go
r.Method(http.MethodGet, "/balances/history", apiHandler(s.handleBalanceHistory))
```

Follows the same JSON-response and error-handling conventions used by the other report endpoints in [reports.go](../../../internal/api/reports.go).

## Frontend

### API client and types

In [api.ts](../../../spa/src/lib/api.ts):

```ts
export function getBalanceHistory(): Promise<BalanceHistoryResponse> {
  return apiFetch<BalanceHistoryResponse>('/api/balances/history');
}
```

In [types.ts](../../../spa/src/lib/types.ts):

```ts
export interface BalanceHistoryPoint {
  month: string;   // "YYYY-MM"
  balance: number; // cents, natural amount
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

### Data fetching

In [balances.tsx](../../../spa/src/routes/balances.tsx):

- Add a second `useQuery({ queryKey: ['balance-history'], queryFn: getBalanceHistory })`. It is **independent** of the balances query — the page renders as soon as balances arrive; sparklines appear when history resolves.
- Convert the response into `Map<number, BalanceHistoryPoint[]>` keyed by `account_id`, memoized on the query data.
- Pass per-row `points` (or `undefined`) down: `BalanceColumn` → `BalanceCardGrid` → `BalanceCard`. The map lookup happens at the `BalanceCardGrid` level so cards stay dumb.
- If the history query errors, no error UI is rendered on the page; cards simply render without sparklines.

### Sparkline component

New file `spa/src/components/balances/Sparkline.tsx`:

```tsx
interface Props {
  points: BalanceHistoryPoint[];
  className?: string;     // controls width/height, e.g. "h-6 w-20"
  strokeClassName: string; // Tailwind stroke-* class
}
```

Implementation notes:

- Hand-rolled SVG, no charting library. The visual is ~80×24px with no axes, no dots, no tooltip, no interactivity — bringing in a 30–80 KB chart dependency is not justified.
- Renders a single `<polyline>` with `fill="none"`, stroke from `strokeClassName`, `vector-effect="non-scaling-stroke"`.
- Y-axis scales to the line's own min and max. When min === max (flat history) the line is drawn as a horizontal line at vertical center.
- When `points.length < 2`, the component returns `null` (renders no SVG).
- The SVG `viewBox` is `0 0 100 30`. X coordinates are evenly spaced across `points.length - 1`. The `className` (e.g. `h-6 w-20`) sizes the rendered SVG.
- `aria-hidden="true"` — purely decorative. The amount and share line carry the accessible information.

### Card layout

In [BalanceCard.tsx](../../../spa/src/components/balances/BalanceCard.tsx):

- The bottom block changes from a single column (amount, then share line) to a flex row:
  - **Left column**: amount on top, share line below — same content as today.
  - **Right side**: `<Sparkline>` pinned to the bottom-right with `self-end`.
- The overall card grows in height. The placeholder card in [BalanceCardGrid.tsx](../../../spa/src/components/balances/BalanceCardGrid.tsx) updates from `h-[88px]` to the new height (target ~112px; final value tuned in implementation to match actual card height with a typical balance).
- The sparkline's `strokeClassName` is derived in the card from `balanceColor(row.type, row.amount)`, mapping its text-color class to the matching `stroke-*` class. The mapping lives next to `balanceColor` so both rules stay in sync.
- The card prop signature gains `points?: BalanceHistoryPoint[]`. When `points` is missing or has fewer than 2 entries, the Sparkline returns null and the right side of the row is empty.

## Edge Cases

| Situation | Behavior |
|---|---|
| History query still loading | Card renders with empty sparkline slot; no spinner. |
| History query errored | Card renders with empty sparkline slot; no error UI on the page. |
| Account in balances list but missing from history response | Empty sparkline slot. |
| Account has only one month of activity | `points.length === 1`; Sparkline renders null; empty slot. |
| All points equal (flat history) | Horizontal line at vertical center of the SVG. |
| Account became negative then positive (or vice versa) | Single-color stroke matching current balance color — the line crosses zero without changing color. Acceptable for v1; we are showing shape, not sign-by-segment. |
| Account currency differs from the page default | Sparkline renders unchanged; cross-currency comparison is not part of v1. |

## Testing

### Backend

- Service-layer white-box test on `GetMonthlyBalanceHistory` using the existing mock pattern in [testhelper_test.go](../../../internal/service/testhelper_test.go):
  - Multi-month activity on an asset: running balance is correct end-of-month for each month.
  - Multi-month activity on a liability: natural-amount sign applied (positive curve when the liability grows in magnitude).
  - One month of activity: single point returned.
  - No activity: account omitted from `items`.
  - Equity / Revenue / Expense accounts excluded from `items`.
  - Empty ledger: `items` is empty.
- Store-layer SQL test for `GetMonthlySplitTotalsForAssetsAndLiabilities` if the existing store test harness covers similar bulk-by-period queries; otherwise leave the SQL covered by the service tests against the in-memory mock and follow up later.
- Handler test in `internal/api/` modeled on [reports_test.go](../../../internal/api/reports_test.go):
  - 200 happy path returning expected JSON shape.
  - Error path: service error → 500 with the standard error envelope.

### Frontend

- `Sparkline.test.tsx`:
  - Renders a `<polyline>` when given ≥2 points.
  - Returns null for `points.length < 2`.
  - Flat input (all balances equal) renders a horizontal line at vertical center.
  - Coordinates for a known small input match the expected scaled values.
- `BalanceCard.test.tsx`:
  - Renders sparkline slot when `points` provided.
  - Does not blow up and renders the rest of the card when `points` is undefined or has length < 2.
- No new tests added for the `balances.tsx` route. Existing balances-page tests must keep passing with the second `useQuery` added; the history query failing must not break the page.

## Out of Scope

- Sparklines in list view.
- Sparklines on the account detail page (could be reused there later but not required for v1).
- Daily granularity, configurable time windows, hover tooltips, axes.
- Cross-account or cross-currency comparison; shared y-axis.
- Caching / snapshot tables. We compute on demand on the server for v1; if perf becomes a concern we can move to a `account_monthly_balance` cache table in a follow-up.
- Color-by-segment (e.g., red below zero, green above zero) — single-color stroke is fine for v1.
