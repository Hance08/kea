# SPA Reports — Replace Per-Row Proportion Bars with a Composition Bar

## Goal

On the Income Breakdown and Expense Breakdown report pages, replace the current per-row `ProportionBar` (one horizontal bar per category, stacked vertically) with a single compact "composition bar" that shows the whole breakdown at a glance. The detail table below the bar acts as the legend and source of exact numbers. This removes the redundancy between bars and table, reduces vertical space, and makes proportions easier to compare.

## Scope

### In scope

- New `CompositionBar` component used on `/reports/income-breakdown` and `/reports/expense-breakdown`.
- Augment `ReportRowTable` to render a small colored swatch next to each row label, sharing the same color palette as the bar so the table acts as the bar's legend.
- Remove `ProportionBar` usage from the two breakdown pages.

### Out of scope

- Income Statement page (`/reports/income-statement`) keeps its existing `ProportionBar` Top-5 visuals. It is not redundant there (no detail table beneath) and is not part of this redesign.
- Deleting the `ProportionBar` component itself. It is still used by Income Statement, so it stays.
- Backend changes. The existing `useIncomeBreakdown` / `useExpenseBreakdown` data is sufficient.
- Click-to-drill-down on bar segments. The detail table already exposes per-account drill-down links.
- Multi-currency composition. Both pages already filter rows to the default currency; the bar renders that single currency, matching the page.

## Frontend

### New `CompositionBar` component

Path: `spa/src/components/reports/CompositionBar.tsx`.

Props:

```ts
interface Props {
  rows: ReportRow[];        // pre-filtered to the displayed currency
  total: number;            // same currency; may be 0
  currency: string;
  variant: 'income' | 'expense';
  className?: string;
}
```

Rendering:

- A single horizontal bar, `h-7` (28px), rounded `rounded` (4px), `overflow-hidden`, full container width.
- Each segment width is `(|row.amount| / |total|) * 100%`. `total === 0` → render nothing and let the parent's empty state apply.
- Top 6 rows by `|amount|` get their own segment. Remaining rows (if any) collapse into a single trailing **Other** segment whose amount is the sum of the remainder and whose count is `rows.length - 6`. If there are exactly 7 rows, the 7th still becomes "Other (1)"; if there are ≤6 rows, no Other segment is rendered.
- **Color palette**: single-hue gradient using existing Tailwind shades. Order is darkest → lightest, biggest → smallest:
  - `expense`: `bg-red-700`, `bg-red-600`, `bg-red-500`, `bg-red-400`, `bg-red-300`, `bg-red-200`.
  - `income`: `bg-emerald-700`, `bg-emerald-600`, `bg-emerald-500`, `bg-emerald-400`, `bg-emerald-300`, `bg-emerald-200`.
  - **Other** segment: `bg-slate-300` (light mode) / matching neutral via tokens — distinct from the gradient so it reads as "remainder", not a category. Text inside Other uses `text-slate-700`.
- **Inline label** inside each segment: shows `<short-name> · NN%` when the segment is ≥ 9% of total width; just `NN%` when between 5% and 9%; nothing below 5%. Text is `text-[10px] font-medium`, `text-white` for the gradient segments (sufficient contrast on shades 400–700; for 200/300 use `text-red-900` / `text-emerald-900` respectively). Truncate with `overflow-hidden whitespace-nowrap`.
- **Hover tooltip** on each segment: shows account name (full, not truncated), formatted amount, and percent. Implementation: a small absolutely-positioned `div` inside the bar container, made visible with `:hover` via React state (`hoverIdx`). Other segments dim to `opacity-60` while one is hovered. Pointer events use `onPointerEnter`/`onPointerLeave` on each segment.
- **Tick markers**: a row of three muted labels below the bar — `0%`, `50%`, `100%` — at the left/center/right. `text-[9px] text-muted-foreground` with `mt-1`.

Accessibility:

- The bar's container has `role="img"` and an `aria-label` like `"Expense composition: Rent 39%, Groceries 14%, …, Other 14%"` (top 3 segments + Other if present).
- Each segment is a `<button type="button">` so it is keyboard-focusable and triggers the tooltip on focus, not only on hover. (Buttons inside the bar keep visual consistency; they have no `onClick` action in v1.)

Empty / edge cases:

- `rows.length === 0` or `total === 0`: component returns `null`. The parent page already shows "No income in …" / "No expenses in …".
- Single row: full-width segment, name and percent inline.
- Negative `total` (signed convention): use `Math.abs(total)` as the denominator, matching today's `ProportionBar` logic.

### `ReportRowTable` — colored swatches

Path: `spa/src/components/reports/ReportRowTable.tsx`.

- Add an optional prop `swatchColors?: string[]` — a list of Tailwind background-color classes, parallel to `rows` (index `i` matches `rows[i]`). When provided, render an `8×8px rounded-[2px]` `<span>` before the account name in each row, using the matching class.
- For rows beyond the top 6 (i.e., rows collapsed into "Other" in the bar), use the same neutral swatch as the bar's Other segment so the visual mapping holds.
- When `swatchColors` is undefined, the table renders unchanged (preserves use on pages that have no bar).

### `reports.income-breakdown.tsx` and `reports.expense-breakdown.tsx`

Replace the `<ProportionBar … />` element with:

1. A small section heading: `<h2 className="mb-2 text-sm font-semibold">Composition</h2>`.
2. `<CompositionBar rows={rows} total={income|expense} currency={currency} variant="income"|"expense" />`.

Pass `swatchColors` to `<ReportRowTable>`. The page derives `swatchColors` from the same top-6-plus-Other partitioning logic as the bar. Factor the partitioning into a small pure helper (`partitionForComposition(rows, n=6) → { primary, other }`) co-located with `CompositionBar` and exported, so both the bar and the page produce identical groupings without duplication.

Remove the now-unused `ProportionBar` import from these two route files. Keep the import in `reports.income-statement.tsx`.

### Tests

- New `reports.composition-bar.test.tsx`:
  - Renders one segment per row when `rows.length ≤ 6`.
  - Collapses rows 7+ into a single Other segment with the correct aggregated amount and count.
  - Segment widths sum to 100% (assert each segment's inline `style.width`).
  - Inline-label visibility: a row with ≥ 9% shows `name · NN%`; a row with 2% shows no inline label.
  - `aria-label` includes the top 3 categories.
  - Empty rows / zero total → `container.firstChild` is `null`.
  - Variant switch: `expense` → red palette, `income` → emerald palette (assert a sentinel class is present).
- Update `reports.income-breakdown.test.tsx` and `reports.expense-breakdown.test.tsx`:
  - Replace existing assertions that target `data-testid="prop-bar"` / `data-testid="bar-fill"` with assertions targeting the composition bar (`data-testid="composition-bar"`).
  - Assert the table rows render colored swatches (presence of swatch element).
- `reports.proportion-bar.test.tsx` is unchanged — `ProportionBar` is still alive for Income Statement.
- Existing `reports.income-statement.test.tsx` is unchanged.

## Migration / Compatibility

- No URL or API changes. The change is purely the SPA's report-page rendering.
- The `ProportionBar` component, hook of consumers, and its tests stay in place for Income Statement.

## Risks

- **Color contrast** on the lightest segments (`-200`, `-300`) with white text: the spec uses dark-on-light (`text-red-900` / `text-emerald-900`) on those shades to keep WCAG contrast. Verify visually after implementation.
- **Many tiny segments visually noisy**: capped at 6 + Other, predictable. If a future user has all 30 categories at near-equal share, the bar still works but loses precision — acceptable, the table covers that.
- **Sub-percent rounding**: percentages displayed inline are `Math.round(pct)`. Sum of displayed percents may equal 99% or 101%. Acceptable; the bar itself uses unrounded widths so it is geometrically correct.
- **Tooltip overflow**: when a segment near the right edge is hovered, the tooltip may clip. Mirror the hover-tooltip pattern from `NetWorthChart` (flip alignment when `leftPct > 70`).
