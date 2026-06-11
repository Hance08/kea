# SPA Balances Page Layout Redesign

## Summary

Reorganize the SPA `/balances` route from a single mixed list of Asset and Liability accounts into two side-by-side columns (Assets left, Liabilities right). Each column has its own sortable Balance header, its own pagination at 8 rows per page, and its own type total displayed in the column's header bar. The existing `NetWorthCard` stays full-width on top. The separate `TypeTotalCard` row is removed (totals fold into the column headers).

## Motivation

The current layout shows a single flat list mixing Assets and Liabilities, with no headers, no sort control, and no pagination. The list grows unbounded and provides no visual separation between the two sides of the balance sheet. A split layout matches how users mentally model net worth (assets on one side, liabilities on the other) and is consistent with the sortable / paginated pattern already established on the `/accounts` page (`accounts.index.tsx`).

## User-Visible Changes

- `NetWorthCard` is unchanged at the top of the page.
- The two `TypeTotalCard` cards in their own row are removed.
- Below NetWorth, the page renders a `grid-cols-2 items-start gap-4` container with two columns.
- Each column is a self-contained card containing:
  1. A header bar showing the type label (`ASSETS` / `LIABILITIES`) and the type total (e.g. `$24,310.00`).
  2. A column-header row with `Account` (left) and `Balance` (right). The Balance label is a button with a sort arrow (`▼` for desc, `▲` for asc).
  3. Up to 8 rows. Each row is a link to `/accounts/$id` with the account name on the left and the balance on the right. Negative amounts render in `text-destructive`.
  4. A pagination control at the bottom — rendered only when the column has more than 8 rows.
- When a column has zero rows, the body shows an empty-state message (`No assets` / `No liabilities`) in muted text in place of rows and pagination.
- Currency excluded count remains surfaced on the `NetWorthCard` as today.

## Sort Behavior

- Default sort: descending by Balance for both columns.
- Sort uses `naturalAmount(type, stored)` (already in `spa/src/lib/accounts.ts`) so descending means "biggest assets" / "biggest debts" regardless of stored sign convention. The same comparator is used by `/accounts`.
- Sort state is independent per column. Clicking the Assets sort header toggles only Assets; clicking Liabilities toggles only Liabilities.
- Toggling the sort resets that column's offset to 0.

## Pagination Behavior

- 8 rows per page per side, fixed (no page-size selector). This is a route-level constant, not a URL param.
- Each side has its own offset.
- Reuses the existing `Pagination` component at `spa/src/components/transactions/Pagination.tsx`.
- The pagination control is hidden when the column has 8 or fewer rows.

## URL State

A new zod schema validates four optional search params on the `/balances` route:

```
/balances?a_offset=0&a_sort=balance_desc&l_offset=0&l_sort=balance_desc
```

| Param      | Type                            | Default         |
|------------|---------------------------------|-----------------|
| `a_offset` | non-negative integer            | `0`             |
| `a_sort`   | `balance_desc` \| `balance_asc` | `balance_desc`  |
| `l_offset` | non-negative integer            | `0`             |
| `l_sort`   | `balance_desc` \| `balance_asc` | `balance_desc`  |

The schema lives in `spa/src/lib/balances-search-params.ts` and is registered via TanStack Router's `validateSearch` on the route. This mirrors the existing `accounts-search-params.ts` pattern.

## Components

### New

- `spa/src/lib/balances-search-params.ts` — zod schema + `parseBalancesSearch` helper.
- `spa/src/components/balances/BalanceColumn.tsx` — one column. Props:
  - `label: 'Assets' | 'Liabilities'`
  - `total: number` (cents)
  - `currency: string`
  - `rows: AccountBalance[]` (already sorted and paged by the route)
  - `totalRowCount: number` (used by the pagination control and to decide whether to show it)
  - `sortDir: 'asc' | 'desc'`
  - `onToggleSort: () => void`
  - `offset: number`
  - `onOffsetChange: (offset: number) => void`
  - `emptyText: string`
- `spa/src/components/balances/BalanceColumnRow.tsx` — one row. Props: `row: AccountBalance`. Links to `/accounts/$id`; negative amounts render in `text-destructive`.

### Modified

- `spa/src/routes/balances.tsx`:
  - Adds `validateSearch: parseBalancesSearch` to the route definition.
  - Splits `summary.included` into `assets` and `liabilities` arrays by `type`.
  - For each side, applies `naturalAmount`-based sort by the side's `sort` param, then slices by the side's `offset` and the 8-row constant.
  - Renders `<NetWorthCard>` then a `<div className="grid grid-cols-2 items-start gap-4">` containing two `<BalanceColumn>` instances.
  - Skeleton and error states are updated to reflect the new layout (two column skeletons instead of one tall list).

### Removed

- `spa/src/components/AccountListRow.tsx` — only caller is the balances route.
- `spa/src/components/TypeTotalCard.tsx` — only caller is the balances route.

## Constants

- `PAGE_SIZE = 8` defined at the top of `balances.tsx` (route-local; not shared because the value is page-specific).

## Testing

Unit tests (Vitest, `spa/src/test/`):

- `parseBalancesSearch` returns defaults when empty, coerces string offsets to numbers, rejects invalid sort values.
- `BalanceColumn` renders:
  - the label and total in the header bar,
  - the sort arrow that matches `sortDir`,
  - the empty text when `rows` is empty (and no pagination),
  - the pagination control only when `totalRowCount > 8`.

Integration tests on the route (existing balances test patterns):

- Mixed asset/liability data partitions correctly into the two columns.
- Default sort is descending for both columns.
- Clicking the Assets sort header changes `a_sort` and not `l_sort`, and vice versa.
- Clicking the Assets pagination advances `a_offset` only.
- Negative liability amount uses `naturalAmount` so the biggest debt sorts first by default.

## Non-Goals

- No change to the `/api/balances` endpoint or to `summarizeBalances`.
- No filter controls (account-type filter, search) on this page. The split is purely visual.
- No persistence of sort/offset beyond URL state.
- No responsive single-column fallback on small screens — out of scope for this change; the page remains two-column.

## Open Questions

None.
