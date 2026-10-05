# SPA Reports — Cap Breakdown Row Table at 8 Rows with Sticky Header

## Goal

On the Income Breakdown and Expense Breakdown report pages, cap the detail row table at 8 visible rows. When there are more rows, the table scrolls vertically inside its container and the column headers stay visible at the top via a sticky `<thead>`. The Balance Sheet's Liabilities table is unchanged.

## Scope

### In scope

- New optional prop `maxVisibleRows?: number` on `ReportRowTable`.
- When the prop is set, the table is wrapped in a scroll container that limits height and enables vertical scrolling.
- When the prop is set, the `<thead>` becomes `sticky top-0` so headers remain visible while scrolling.
- Pass `maxVisibleRows={8}` from `reports.income-breakdown.tsx` and `reports.expense-breakdown.tsx`.

### Out of scope

- Balance Sheet's Liabilities table — leaves it tall and unscrolled (it typically has few rows; the prop is opt-in).
- Income Statement's Top-5 ProportionBars — they already show a hard limit.
- A configurable per-page row-cap (e.g. a "show 25 / 50 / 100" picker). 8 is a fixed product decision for this iteration.
- Virtualized rendering — at the row counts in this app (single-digit to low hundreds), plain DOM scroll is sufficient. No `react-window` or similar dependency.
- Horizontal scrolling. Existing `truncate` on long account names handles overflow already.

## Frontend

### `ReportRowTable` changes

File: `spa/src/components/reports/ReportRowTable.tsx`.

Add to `interface Props`:

```ts
maxVisibleRows?: number; // when set, table scrolls with a sticky header after this many rows
```

The render becomes:

- If `maxVisibleRows` is undefined, render exactly as today (no wrapper, no max-height, no sticky header).
- If `maxVisibleRows` is set:
  - Wrap the existing `<table>` in a scroll container: `<div data-testid="report-row-scroll" className="overflow-y-auto rounded-md border" style={{ maxHeight: \`${maxVisibleRows * 2 + 2}rem\` }}>`. The body row height is ~2rem (32px from `py-1.5` + `text-sm`), and the sticky header eats ~2rem of the same space (`py-1` + `text-xs` + a bit of headroom). So `(rows × 2) + 2` rem fits exactly the requested number of data rows AND the visible header, with `rows + 1` rows triggering scroll. The container's max-height scales with the prop value, so a future `maxVisibleRows={5}` produces a 5-row cap (12rem) without spec changes.
  - Add `sticky top-0 z-10 bg-background` to the existing `<thead>` element so the headers stay glued to the top of the scroll container. Keep the existing `text-xs uppercase text-muted-foreground` classes intact.
  - No change to row markup, swatches, or links.

Implementation note: an inline `style` is used (instead of a Tailwind utility) because Tailwind cannot template `max-h-[${N}rem]` at runtime — the JIT compiler scans the source for literal class names. Inline style sidesteps that constraint without adding a config entry.

### Breakdown pages

In `spa/src/routes/reports.expense-breakdown.tsx` and `spa/src/routes/reports.income-breakdown.tsx`, pass `maxVisibleRows={8}` to the existing `<ReportRowTable>` element. No other changes.

### Tests

In `spa/src/test/components/ReportRowTable.test.tsx`, add two tests:

- `'ReportRowTable: no scroll wrapper when maxVisibleRows is undefined'` — renders without `maxVisibleRows`, asserts that the rendered output has no element matching `[data-testid="report-row-scroll"]` (selector for the scroll wrapper).
- `'ReportRowTable: scroll wrapper and sticky header when maxVisibleRows is set'` — renders with `maxVisibleRows={8}`, asserts the presence of `[data-testid="report-row-scroll"]`, that the wrapper's `className` contains `overflow-y-auto`, and that the wrapper's inline `style.maxHeight` is `'18rem'` (= 8 × 2 + 2 rem, accounting for the sticky header). Asserts the `<thead>` element's `className` contains `sticky` and `top-0`.

Update the page-level tests so they continue to pass — `reports.expense-breakdown.test.tsx` and `reports.income-breakdown.test.tsx` already query by `data-testid="row-swatch"` and `data-testid="composition-segment"`, both of which still render inside the new wrapper.

## Migration / Compatibility

No API changes. No URL changes. The new prop is opt-in, so existing callers (Balance Sheet) are unaffected.

## Risks

- **Row height drift**: the 320px max-height assumes ~32px per row. If a future style change (larger font, more padding) makes rows taller, 8 rows may overflow and only 7 fit before scroll kicks in. Mitigation: the scroll just engages slightly earlier, no functional break. If precise "exactly 8 rows" matters later, switch to a CSS-variable-driven height.
- **Sticky header z-index**: the new `z-10` on `<thead>` keeps it above row swatches and link styles. Verify in dev that no overlapping fixed element (e.g. the page's `PeriodPicker`) appears above the table at the same z-level.
- **Bundle rebuild**: the SPA bundle (`internal/web/dist/index.html` and assets) must be rebuilt and committed so users running `kea serve` from the binary see the change.
