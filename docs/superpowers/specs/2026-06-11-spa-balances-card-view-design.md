# SPA Balances Card View Design

## Summary

Add a card-grid alternative to the list view on `/balances`. A small top-right toggle switches between **List** (the existing layout) and **Cards** (a 2-column inner grid inside each column, showing one card per account with name, currency badge, balance, and share-of-column-total). Per-side sort and per-side 8/page pagination apply to both views and use the same URL params; only the rendering changes.

## Motivation

The list view is dense and scannable, but it doesn't surface relative weight at a glance. A card view makes it easier to compare accounts by share of total and gives the page more visual structure. Rather than replacing the list, this introduces the card view as an alternative the user can toggle into and back out of, with their sort/page state preserved.

## User-Visible Changes

- A small two-button icon toggle appears above the Assets / Liabilities column grid, right-aligned. Icons: list (☰) and grid (▦). The active state has the standard shadcn highlighted button look.
- Default view is `list` — current behavior is unchanged on a fresh `/balances` visit.
- Clicking the grid icon switches both columns to cards mode. Clicking the list icon switches back.
- In cards mode, each column's body becomes a 2-column inner grid (`grid-cols-2 gap-3`). With the existing 8-per-page cap, that's 4 rows × 2 cols per page.
- Each card shows:
  1. Account name with the column-type prefix stripped (e.g., `Assets:Investments:00878` → `Investments:00878`). The stripped prefixes are the canonical names: literal `Assets:` in the Assets column and literal `Liabilities:` in the Liabilities column. Accounts that don't start with the canonical prefix render with their full name unchanged. If the displayed name is still wider than the card, truncated with ellipsis; full original (un-stripped) name is in a `title` tooltip on every card.
  2. Currency code in a small pill, top-right of the card.
  3. Balance formatted as `$X,XXX.XX` with no sign and no currency code, colored by sign (positive green, negative red, zero default) — same rule as the list rows.
  4. Share line: `<n>% of <assets|liabilities>` immediately below the balance. Hidden when the column total is zero.
- The whole card is a link to `/accounts/$id` (same target as a list row).
- The sort affordance moves with the view. In list mode it stays in the `Account / Balance` subheader row. In cards mode it renders inside the column header bar next to the total: a button showing `▼` or `▲` followed by the total amount; clicking it toggles `a_sort` / `l_sort` exactly as the list-mode button does.
- Pagination control stays at the bottom of each column body in both views and uses the same per-side offset.

## URL State

Add one new optional param to `balancesSearchSchema`:

| Param  | Type                 | Default  |
|--------|----------------------|----------|
| `view` | `'list'` \| `'cards'` | `'list'` |

The view toggle updates only `view`; it does not reset `a_offset`, `a_sort`, `l_offset`, `l_sort`. Likewise, sort toggles and pagination clicks do not change `view`.

## Card Layout

```
┌────────────────────────────────┐
│ Investments:00878         USD  │   ← stripped name + currency badge
│                                │
│ $162,979.00                    │   ← balance, colored by sign
│ 75% of assets                  │   ← share of column total (hidden if total = 0)
└────────────────────────────────┘
```

Card visual structure: a rounded-border container with `flex flex-col justify-between` content, the same `Card`-style border/background as the list-mode column. The link spans the entire card; hover shows the standard `bg-muted/40` treatment used elsewhere.

## Sort Affordance in Cards Mode

Cards mode has no `Account / Balance` subheader row, so the sort toggle is moved into the column header bar. The bar layout becomes:

```
ASSETS                                      ▼ $24,310.00
```

Where `▼` (or `▲`) is the clickable sort indicator. The total amount stays right-aligned. The handler is the same `onToggleSort` already used by the list-mode subheader button. Hover affordance: the arrow + amount cluster is wrapped in a single `<button>` whose hover state shifts the muted arrow color to `text-foreground` (matching the existing `hover:text-foreground` pattern on the list-mode sort button).

## Share Calculation

For each row in a column, the share is `Math.round((|row.amount| / |columnTotal|) * 100)`. When `columnTotal === 0` the share is `null` and the share line is not rendered for any card in that column (a half-empty column with NaN%s would be worse than just omitting the line).

This calculation happens once in `balances.tsx` per column after the sort/slice memo chain, so the card component remains a pure presentational unit that receives the share as a prop.

## Fixed Column Height

The same "pad with invisible placeholders to `BALANCE_COLUMN_PAGE_SIZE` slots" trick used by the list view applies in cards mode — render empty `<div>` placeholders to fill the remaining grid cells so the pagination control stays at a fixed vertical position even on the last (partial) page.

## Components

### New

- `spa/src/components/balances/ViewToggle.tsx`
  - Props: `value: 'list' | 'cards'`, `onChange: (next: 'list' | 'cards') => void`
  - Renders two icon buttons (list + grid) from `lucide-react`, with the active button styled distinctly.
- `spa/src/components/balances/BalanceCardGrid.tsx`
  - Props: identical to the list-mode rows section: `rows`, `shares`, `columnLabel` (used by each card for prefix stripping and share-line wording).
  - Renders a `grid-cols-2 gap-3` grid of `BalanceCard`s plus the placeholder padding.
- `spa/src/components/balances/BalanceCard.tsx`
  - Props: `row: AccountBalance`, `columnLabel: 'Assets' | 'Liabilities'`, `share: number | null`.
  - Strips the prefix, applies truncation + tooltip, renders the currency badge + balance + share line, wraps everything in a `<Link>` to `/accounts/$id`.

### Modified

- `spa/src/lib/balances-search-params.ts` — adds the `view` field.
- `spa/src/components/balances/BalanceColumn.tsx` — accepts a new `view` prop. In list mode renders the existing body; in cards mode renders `<BalanceCardGrid>` and moves the sort toggle into the header bar. New prop `shares: (number | null)[]` aligned to `rows`.
- `spa/src/routes/balances.tsx` — reads `search.view`, computes per-row shares once per column, renders `<ViewToggle>` above the column grid, passes `view` and `shares` to each `BalanceColumn`.

## Testing

Unit tests (Vitest, `spa/src/test/`):

- `parseBalancesSearch`: defaults `view` to `'list'`, accepts `'cards'`, rejects unknown values.
- `BalanceCard`:
  - Strips `Assets:` prefix in an Assets column and `Liabilities:` prefix in a Liabilities column.
  - Renders the `title` tooltip with the original (un-stripped) name.
  - Shows the share line when `share` is a number, hides it when `share` is `null`.
  - Applies the correct sign-based color to the balance.
- `BalanceColumn` in cards mode:
  - Renders the grid, not the list body.
  - Renders the sort button in the header bar, not in a subheader row.
  - Hides the sort button when the column is empty (same rule as list mode).

Integration tests on the route:

- Default view is `list`; clicking the cards icon updates only the `view` URL param; clicking it again returns to `list`.
- Switching view preserves `a_offset`, `a_sort`, `l_offset`, `l_sort`.
- In cards mode with mixed asset/liability fixtures, cards land in the correct column and the share line totals to 100% (within rounding) per column.
- Cards-mode pagination advances `a_offset` only when clicked on the Assets side.

## Non-Goals

- No keyboard shortcut for switching views (mouse only for now).
- No per-column view toggle — single page-wide setting.
- No animation on view transition — instant swap.
- No card-mode-specific sort options (e.g., sort by share); same balance sort applies.
- No responsive collapse of the 2-column inner grid on narrow viewports. Cards mode inherits the same "no responsive fallback" non-goal as the current layout.

## Open Questions

None.
