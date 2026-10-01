# Breakdown Row Table 8-Row Scroll Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cap the `ReportRowTable` at 8 visible rows with a sticky header and vertical scroll on the Income/Expense Breakdown pages, leaving Balance Sheet's usage unchanged.

**Architecture:** `ReportRowTable` gains an optional `maxVisibleRows?: number` prop. When set, the table is wrapped in a scroll container whose `max-height` is `(rows * 2 + 2)rem` (accounting for the 2rem-tall sticky header), and the existing `<thead>` becomes `sticky top-0 z-10 bg-background`. When unset, the rendering is byte-identical to today.

**Tech Stack:** React 18 + TypeScript, Tailwind CSS (inline `style` for the dynamic max-height because Tailwind's JIT can't template arbitrary runtime classnames), Vitest + Testing Library. No new dependencies.

Spec: `docs/superpowers/specs/2026-06-24-report-row-table-scroll-design.md`

---

## File Map

**Frontend — modify:**
- `spa/src/components/reports/ReportRowTable.tsx` — new optional prop, conditional scroll wrapper, sticky header classes.
- `spa/src/test/components/ReportRowTable.test.tsx` — 2 new tests (wrapper absent when prop unset; wrapper + sticky-header present when prop set).
- `spa/src/routes/reports.expense-breakdown.tsx` — pass `maxVisibleRows={8}`.
- `spa/src/routes/reports.income-breakdown.tsx` — pass `maxVisibleRows={8}`.
- `internal/web/dist/index.html` — refresh bundle placeholder so `kea serve` picks up the change.

**Frontend — leave untouched:**
- `spa/src/routes/reports.balance-sheet.tsx` — does not pass the new prop, so its Liabilities table renders unchanged.

---

## Conventions

- Run tests from `/Users/hance/programming/kea` with `cd spa && npm test -- <pattern>`.
- Run lint/typecheck with `cd spa && npm run check && tsc -b`. 3 pre-existing errors in `ChartRangeSelector.tsx`, `NetWorthChart.tsx`, and `reports.income-statement.tsx:32` are not yours.
- The full `npm test` suite has 6 pre-existing failures in `balances.test.tsx` and `balances.link.test.tsx` — confirmed not caused by this work.
- Bundle build: `cd spa && npm run build`. Only commit `internal/web/dist/index.html` (the rest under `dist/` is gitignored).

---

## Task 1: Add `maxVisibleRows` prop with scroll wrapper and sticky header

**Files:**
- Modify: `spa/src/components/reports/ReportRowTable.tsx`
- Modify: `spa/src/test/components/ReportRowTable.test.tsx`

- [ ] **Step 1: Add failing tests**

Append to `spa/src/test/components/ReportRowTable.test.tsx`:

```tsx
test('ReportRowTable: no scroll wrapper when maxVisibleRows is undefined', () => {
  const { container } = render(
    withServerConfig(
      <ReportRowTable rows={rows} currency="USD" nameToId={nameToId} period={null} />,
    ),
  );
  expect(container.querySelector('[data-testid="report-row-scroll"]')).toBeNull();
});

test('ReportRowTable: scroll wrapper and sticky header when maxVisibleRows is set', () => {
  const { container } = render(
    withServerConfig(
      <ReportRowTable
        rows={rows}
        currency="USD"
        nameToId={nameToId}
        period={null}
        maxVisibleRows={8}
      />,
    ),
  );
  const wrapper = container.querySelector<HTMLElement>('[data-testid="report-row-scroll"]');
  expect(wrapper).not.toBeNull();
  expect(wrapper?.className).toContain('overflow-y-auto');
  // 8 rows × 2rem + 2rem (sticky header) = 18rem
  expect(wrapper?.style.maxHeight).toBe('18rem');

  const thead = container.querySelector('thead');
  expect(thead?.className).toContain('sticky');
  expect(thead?.className).toContain('top-0');
});
```

- [ ] **Step 2: Run tests to verify failures**

Run: `cd spa && npm test -- ReportRowTable`

Expected: 2 new tests fail — `report-row-scroll` testid not found; the `maxVisibleRows` prop is rejected by TypeScript (compile error surfaces as failed render).

- [ ] **Step 3: Implement the prop and conditional wrapping**

Edit `spa/src/components/reports/ReportRowTable.tsx`:

- Add to the `Props` interface (place under the existing `swatchColors?: string[]` field):

```ts
// when set, the table scrolls vertically and the header sticks to the top
// after this many rows fit. Unset → render unchanged (no wrapper, no sticky).
maxVisibleRows?: number;
```

- Update the destructure in the function signature:

```ts
export function ReportRowTable({
  rows,
  currency,
  nameToId,
  period,
  swatchColors,
  maxVisibleRows,
}: Props) {
```

- After the existing early `if (rows.length === 0) return …` guard, but before the `return (` block, build the table element once and wrap it conditionally. Replace the entire `return ( <table …> … </table> );` with the block below. Note the `<thead>` inside this block also picks up the conditional sticky classes — that's the single source of truth for the header markup; there is no separate `<thead>` change.

```tsx
const table = (
  <table className="w-full text-sm">
    <thead
      className={cn(
        'text-xs uppercase text-muted-foreground',
        maxVisibleRows !== undefined && 'sticky top-0 z-10 bg-background',
      )}
    >
      <tr>
        <th className="py-1 text-left font-medium">Account</th>
        <th className="py-1 text-left font-medium">Offset</th>
        <th className="py-1 text-right font-medium">Amount</th>
        <th className="py-1 text-right font-medium">Tx</th>
      </tr>
    </thead>
    <tbody>
      {rows.map((row, rowIndex) => {
        const id = nameToId.get(row.account_name);
        const labelSpan = (
          <span className="truncate" title={row.account_name}>
            {stripAccountTypePrefix(row.account_name)}
          </span>
        );
        const swatch = swatchColors?.[rowIndex] ? (
          <span
            data-testid="row-swatch"
            className={cn(
              'mr-2 inline-block h-2 w-2 rounded-[2px] align-middle',
              swatchColors[rowIndex],
            )}
            aria-hidden="true"
          />
        ) : null;
        const linkContent = swatch ? (
          <span className="inline-flex min-w-0 items-center">
            {swatch}
            {labelSpan}
          </span>
        ) : (
          labelSpan
        );
        return (
          <tr key={row.account_name} className="border-t hover:bg-muted/40">
            <td className="py-1.5 max-w-[260px]">
              {period === null ? (
                id !== undefined ? (
                  <Link
                    to="/accounts/$id"
                    params={{ id: String(id) }}
                    search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
                    className="hover:underline"
                  >
                    {linkContent}
                  </Link>
                ) : (
                  linkContent
                )
              ) : (
                <Link
                  to="/transactions"
                  search={{
                    ...(id !== undefined ? { account_id: id } : {}),
                    start_time: period.startUnix,
                    end_time: period.endUnix,
                    limit: DEFAULT_TRANSACTIONS_LIMIT,
                    offset: 0,
                  }}
                  className="hover:underline"
                >
                  {linkContent}
                </Link>
              )}
            </td>
            <td className="py-1.5 text-muted-foreground" title={row.offset_account}>
              {stripAccountTypePrefix(row.offset_account)}
            </td>
            <td className="py-1.5 text-right font-mono tabular-nums">
              {formatCents(row.amount, currency)}
            </td>
            <td className="py-1.5 text-right text-muted-foreground">{row.tx_count}</td>
          </tr>
        );
      })}
    </tbody>
  </table>
);

if (maxVisibleRows === undefined) return table;

return (
  <div
    data-testid="report-row-scroll"
    className="overflow-y-auto rounded-md border"
    style={{ maxHeight: `${maxVisibleRows * 2 + 2}rem` }}
  >
    {table}
  </div>
);
```

- [ ] **Step 4: Run tests**

Run: `cd spa && npm test -- ReportRowTable`

Expected: all 4 tests in this file pass (2 previously existing + 2 new).

- [ ] **Step 5: Type-check and lint**

Run: `cd spa && npm run check && tsc -b`

Expected: 3 pre-existing errors unrelated to this file. No new errors on `ReportRowTable.tsx` or its test. If Biome rewrites your formatting, accept with `npx biome check --write src/components/reports/ReportRowTable.tsx` and re-run the tests.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/reports/ReportRowTable.tsx spa/src/test/components/ReportRowTable.test.tsx
git commit -m "feat(spa): optional row-cap scroll + sticky header in ReportRowTable"
```

---

## Task 2: Wire `maxVisibleRows={8}` into both breakdown pages

**Files:**
- Modify: `spa/src/routes/reports.expense-breakdown.tsx`
- Modify: `spa/src/routes/reports.income-breakdown.tsx`

- [ ] **Step 1: Edit Expense Breakdown page**

In `spa/src/routes/reports.expense-breakdown.tsx`, find the `<ReportRowTable>` element. Add `maxVisibleRows={8}` to its prop list. Final element:

```tsx
<ReportRowTable
  rows={rows}
  currency={currency}
  nameToId={nameToId}
  period={{ startUnix: period.startUnix, endUnix: period.endUnix }}
  swatchColors={swatchColors}
  maxVisibleRows={8}
/>
```

- [ ] **Step 2: Edit Income Breakdown page**

Same change in `spa/src/routes/reports.income-breakdown.tsx` — add `maxVisibleRows={8}` to the `<ReportRowTable>` prop list.

- [ ] **Step 3: Run the breakdown page tests**

Run: `cd spa && npm test -- reports.income-breakdown reports.expense-breakdown`

Expected: both files pass. The existing tests query by `data-testid="row-swatch"` and `data-testid="composition-segment"`; both still render inside the new scroll wrapper so the queries continue to work.

- [ ] **Step 4: Run the full breakdown-related suite as belt and suspenders**

Run: `cd spa && npm test -- reports`

Expected: all matching tests pass except the 6 pre-existing `balances*` failures (those don't match `reports`, so this command won't surface them anyway).

- [ ] **Step 5: Type-check and lint**

Run: `cd spa && npm run check && tsc -b`

Expected: still just the 3 pre-existing errors.

- [ ] **Step 6: Commit**

```bash
git add spa/src/routes/reports.expense-breakdown.tsx spa/src/routes/reports.income-breakdown.tsx
git commit -m "feat(spa): cap breakdown row tables at 8 visible rows"
```

---

## Task 3: Rebuild and commit the embedded SPA bundle

**Files:**
- Modify: `internal/web/dist/index.html` (the rest of `dist/` is gitignored).

- [ ] **Step 1: Build the SPA**

Run: `cd spa && npm run build`

Expected: build succeeds, no errors. The build writes new hashed asset files under `internal/web/dist/assets/`, and `internal/web/dist/index.html` updates its `<script>` / `<link>` references to the new hashes.

- [ ] **Step 2: Check the index.html diff**

Run: `cd /Users/hance/programming/kea && git diff -- internal/web/dist/index.html`

Expected: a small diff updating the asset hash filenames in the `<script type="module" src="…">` and `<link rel="stylesheet" href="…">` attributes.

- [ ] **Step 3: Commit if changed**

```bash
git add internal/web/dist/index.html
git commit -m "build(spa): refresh embedded bundle"
```

If `git status` shows `internal/web/dist/index.html` is unchanged after the build (unlikely, since the source changed), no commit is needed.

- [ ] **Step 4: Final sanity check**

Run: `cd /Users/hance/programming/kea && git log --oneline fb3d940..HEAD`

Expected: three new commits, in order:

```
<sha> build(spa): refresh embedded bundle
<sha> feat(spa): cap breakdown row tables at 8 visible rows
<sha> feat(spa): optional row-cap scroll + sticky header in ReportRowTable
```

---

## Self-Review Notes

- **Spec coverage**: every requirement maps to a task. Prop name and signature (T1), conditional wrapper and sticky header (T1), inline-style max-height formula `(rows × 2 + 2)rem` (T1 Step 3 + test in T1 Step 1), two new tests (T1 Step 1), `maxVisibleRows={8}` on both breakdown pages (T2), bundle rebuild (T3).
- **Out of scope respected**: Balance Sheet (`reports.balance-sheet.tsx`) is not modified, so its Liabilities table stays unscrolled — matches spec.
- **No placeholders**: every step has the actual code or the actual command + expected output.
- **Type consistency**: `maxVisibleRows?: number` defined once in T1, consumed unchanged in T2's prop usage.
