# SPA Breakdown Composition Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the per-row `ProportionBar` on `/reports/income-breakdown` and `/reports/expense-breakdown` with a single compact `CompositionBar`; the existing detail table acts as the bar's legend via colored swatches per row.

**Architecture:** A new `CompositionBar.tsx` component renders one horizontal bar with up to 6 colored segments plus an optional "Other" segment, with inline labels on wide-enough segments and hover tooltips. A pure helper `partitionForComposition` produces both the bar segments and a parallel `swatchColors` array for the table. `ReportRowTable` gains an optional `swatchColors` prop. The Income/Expense Breakdown route components swap `ProportionBar` for `CompositionBar` and pass swatches to the table. `ProportionBar` itself stays — Income Statement still uses it.

**Tech Stack:** React 18 + TypeScript, Tailwind CSS (existing palette only — no new colors), Vitest + Testing Library, `clsx`/`cn` for class composition. No new dependencies.

Spec: `docs/superpowers/specs/2026-06-24-spa-breakdown-composition-bar-design.md`

---

## File Map

**Frontend — create:**
- `spa/src/components/reports/CompositionBar.tsx` — bar component + `partitionForComposition` helper + `CompositionSegment` type.
- `spa/src/test/reports.composition-bar.test.tsx` — component + helper tests.

**Frontend — modify:**
- `spa/src/components/reports/ReportRowTable.tsx` — add optional `swatchColors?: string[]` prop and render a swatch dot before each account name when provided.
- `spa/src/routes/reports.income-breakdown.tsx` — replace `ProportionBar` with `CompositionBar`, derive `swatchColors`, pass to `ReportRowTable`.
- `spa/src/routes/reports.expense-breakdown.tsx` — same as above for expenses.
- `spa/src/test/reports.income-breakdown.test.tsx` — add assertions for the composition bar and swatch dots.
- `spa/src/test/reports.expense-breakdown.test.tsx` — same.

**Frontend — leave untouched:**
- `spa/src/components/reports/ProportionBar.tsx` and `spa/src/test/reports.proportion-bar.test.tsx` — still used by Income Statement.
- `spa/src/routes/reports.income-statement.tsx` — unchanged.

---

## Conventions used throughout

- All new files: no copyright header (matches existing SPA components like `NetWorthChart.tsx`).
- Type imports from `@/lib/types` (e.g. `ReportRow`).
- Class composition via `cn` from `@/lib/cn`.
- Money formatting via `useAmountFormat()` from `@/lib/server-config`.
- Account label stripping via `stripAccountTypePrefix` from `@/lib/accounts`.

---

## Task 1: Add `partitionForComposition` helper with tests

This is a pure function — implement and test first so later tasks can rely on it without doubt.

**Files:**
- Create: `spa/src/components/reports/CompositionBar.tsx` (helper + types only at this task)
- Create: `spa/src/test/reports.composition-bar.test.tsx`

- [ ] **Step 1: Write the failing tests**

Create `spa/src/test/reports.composition-bar.test.tsx`:

```tsx
import { expect, test } from 'vitest';
import { partitionForComposition } from '../components/reports/CompositionBar';
import type { ReportRow } from '../lib/types';

const row = (name: string, amount: number, offset = ''): ReportRow => ({
  account_name: name,
  offset_account: offset,
  amount,
  currency: 'USD',
  tx_count: 1,
});

test('partition: returns one segment per row when rows ≤ 6', () => {
  const rows = [
    row('Rent', 1800),
    row('Groceries', 642),
    row('Dining', 520),
  ];
  const { segments, swatchColors } = partitionForComposition(rows, 2962, 'expense');
  expect(segments).toHaveLength(3);
  expect(segments[0].label).toBe('Rent');
  expect(segments[0].amount).toBe(1800);
  expect(segments[0].isOther).toBe(false);
  expect(swatchColors).toHaveLength(3);
});

test('partition: rows beyond top 6 collapse into a single Other segment', () => {
  const rows = [
    row('A', 1000), row('B', 900), row('C', 800), row('D', 700),
    row('E', 600), row('F', 500), row('G', 400), row('H', 300), row('I', 200),
  ];
  const total = 5400;
  const { segments } = partitionForComposition(rows, total, 'expense');
  expect(segments).toHaveLength(7); // top 6 + Other
  const other = segments[6];
  expect(other.isOther).toBe(true);
  expect(other.amount).toBe(900); // G+H+I = 400+300+200
  expect(other.label).toBe('Other (3)');
});

test('partition: sorts segments by |amount| descending', () => {
  const rows = [
    row('Small', 100),
    row('Big', 5000),
    row('Mid', 1000),
  ];
  const { segments } = partitionForComposition(rows, 6100, 'expense');
  expect(segments.map((s) => s.label)).toEqual(['Big', 'Mid', 'Small']);
});

test('partition: segment pcts use |total| as denominator', () => {
  const rows = [row('A', -1000), row('B', -3000)];
  const { segments } = partitionForComposition(rows, -4000, 'expense');
  expect(segments[0].pct).toBeCloseTo(75, 5);
  expect(segments[1].pct).toBeCloseTo(25, 5);
});

test('partition: total of 0 yields pct=0 for every segment (no NaN)', () => {
  const rows = [row('A', 0), row('B', 0)];
  const { segments } = partitionForComposition(rows, 0, 'expense');
  expect(segments.every((s) => s.pct === 0)).toBe(true);
});

test('partition: expense variant uses red gradient, income uses emerald', () => {
  const rows = [row('A', 100)];
  expect(partitionForComposition(rows, 100, 'expense').segments[0].colorClass).toBe('bg-red-700');
  expect(partitionForComposition(rows, 100, 'income').segments[0].colorClass).toBe('bg-emerald-700');
});

test('partition: swatchColors is parallel to input rows (not segment order)', () => {
  // Big-Mid-Small is the sort order for segments, but the table needs
  // colors in input order: Small, Big, Mid.
  const rows = [
    row('Small', 100),
    row('Big', 5000),
    row('Mid', 1000),
  ];
  const { swatchColors, segments } = partitionForComposition(rows, 6100, 'expense');
  // The "Big" row (input index 1) maps to the darkest shade.
  expect(swatchColors[1]).toBe(segments[0].colorClass);
  // "Mid" → second darkest, "Small" → third darkest.
  expect(swatchColors[2]).toBe(segments[1].colorClass);
  expect(swatchColors[0]).toBe(segments[2].colorClass);
});

test('partition: rows beyond top 6 get the Other (neutral) swatch', () => {
  const rows = [
    row('A', 1000), row('B', 900), row('C', 800), row('D', 700),
    row('E', 600), row('F', 500), row('G', 400),
  ];
  const { swatchColors, segments } = partitionForComposition(rows, 4900, 'expense');
  const otherSeg = segments.find((s) => s.isOther);
  expect(otherSeg).toBeDefined();
  expect(swatchColors[6]).toBe(otherSeg?.colorClass); // G is collapsed
});
```

- [ ] **Step 2: Run tests to verify they all fail (import error)**

Run: `cd spa && npm test -- reports.composition-bar`

Expected: All tests fail because `CompositionBar.tsx` does not yet export `partitionForComposition`.

- [ ] **Step 3: Implement the helper**

Create `spa/src/components/reports/CompositionBar.tsx`:

```tsx
import type { ReportRow } from '@/lib/types';

export type CompositionVariant = 'income' | 'expense';

export interface CompositionSegment {
  label: string;            // account name (stripped of type prefix), or "Other (N)"
  amount: number;           // |amount|
  pct: number;              // 0..100, exact (not rounded)
  colorClass: string;       // Tailwind background class
  textClass: string;        // Tailwind text class for in-segment label contrast
  isOther: boolean;
}

const GRADIENT: Record<CompositionVariant, { bg: string; text: string }[]> = {
  expense: [
    { bg: 'bg-red-700', text: 'text-white' },
    { bg: 'bg-red-600', text: 'text-white' },
    { bg: 'bg-red-500', text: 'text-white' },
    { bg: 'bg-red-400', text: 'text-white' },
    { bg: 'bg-red-300', text: 'text-red-900' },
    { bg: 'bg-red-200', text: 'text-red-900' },
  ],
  income: [
    { bg: 'bg-emerald-700', text: 'text-white' },
    { bg: 'bg-emerald-600', text: 'text-white' },
    { bg: 'bg-emerald-500', text: 'text-white' },
    { bg: 'bg-emerald-400', text: 'text-white' },
    { bg: 'bg-emerald-300', text: 'text-emerald-900' },
    { bg: 'bg-emerald-200', text: 'text-emerald-900' },
  ],
};

const OTHER_BG = 'bg-slate-300';
const OTHER_TEXT = 'text-slate-700';

// Strip an account-type prefix like "Expenses:" or "Income:".
// Mirrors stripAccountTypePrefix from @/lib/accounts but inline so this
// helper has no surprise import cycles when used in tests.
function shortLabel(name: string): string {
  const i = name.indexOf(':');
  return i >= 0 ? name.slice(i + 1) : name;
}

export function partitionForComposition(
  rows: ReportRow[],
  total: number,
  variant: CompositionVariant,
  topN = 6,
): { segments: CompositionSegment[]; swatchColors: string[] } {
  const palette = GRADIENT[variant];
  const denom = Math.abs(total);

  // Sort by |amount| descending, but remember each row's original index so
  // we can build a parallel swatchColors array.
  const indexed = rows.map((r, i) => ({ row: r, i }));
  indexed.sort((a, b) => Math.abs(b.row.amount) - Math.abs(a.row.amount));

  const swatchColors: string[] = new Array(rows.length).fill(OTHER_BG);
  const segments: CompositionSegment[] = [];

  const primary = indexed.slice(0, topN);
  const rest = indexed.slice(topN);

  primary.forEach(({ row, i }, segIdx) => {
    const color = palette[segIdx];
    const amount = Math.abs(row.amount);
    segments.push({
      label: shortLabel(row.account_name),
      amount,
      pct: denom === 0 ? 0 : (amount / denom) * 100,
      colorClass: color.bg,
      textClass: color.text,
      isOther: false,
    });
    swatchColors[i] = color.bg;
  });

  if (rest.length > 0) {
    const sum = rest.reduce((acc, { row }) => acc + Math.abs(row.amount), 0);
    segments.push({
      label: `Other (${rest.length})`,
      amount: sum,
      pct: denom === 0 ? 0 : (sum / denom) * 100,
      colorClass: OTHER_BG,
      textClass: OTHER_TEXT,
      isOther: true,
    });
    // swatchColors for rest indices remain OTHER_BG (already set above).
  } else {
    // No rest → swatchColors for indices beyond primary should not exist;
    // but rows.length === primary.length here, so the fill is harmless.
  }

  return { segments, swatchColors };
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npm test -- reports.composition-bar`

Expected: All 8 helper tests pass.

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/reports/CompositionBar.tsx spa/src/test/reports.composition-bar.test.tsx
git commit -m "feat(spa): add partitionForComposition helper for breakdown bar"
```

---

## Task 2: Render the CompositionBar with segment widths and palette

Implement the bar element itself — segments, widths, classes — without inline labels, tooltip, or ticks yet.

**Files:**
- Modify: `spa/src/components/reports/CompositionBar.tsx`
- Modify: `spa/src/test/reports.composition-bar.test.tsx`

- [ ] **Step 1: Add failing tests for the component**

Append to `spa/src/test/reports.composition-bar.test.tsx`:

```tsx
import { render as rtlRender } from '@testing-library/react';
import type { ReactNode } from 'react';
import { CompositionBar } from '../components/reports/CompositionBar';
import { withServerConfig } from './test-app';

// All component tests wrap with withServerConfig so they keep working once
// Task 4 adds useAmountFormat() to the component.
const renderBar = (ui: ReactNode) => rtlRender(withServerConfig(ui));

test('component: returns null when rows is empty', () => {
  const { container } = renderBar(
    <CompositionBar rows={[]} total={0} currency="USD" variant="expense" />,
  );
  // ServerConfig wrapper renders, but the component itself returns null.
  expect(container.querySelector('[data-testid="composition-bar"]')).toBeNull();
});

test('component: returns null when total is 0', () => {
  const rows = [row('A', 0)];
  const { container } = renderBar(
    <CompositionBar rows={rows} total={0} currency="USD" variant="expense" />,
  );
  expect(container.querySelector('[data-testid="composition-bar"]')).toBeNull();
});

test('component: renders one DOM segment per partition segment', () => {
  const rows = [row('Rent', 1800), row('Groceries', 642), row('Dining', 520)];
  const { container } = renderBar(
    <CompositionBar rows={rows} total={2962} currency="USD" variant="expense" />,
  );
  const segs = container.querySelectorAll('[data-testid="composition-segment"]');
  expect(segs).toHaveLength(3);
});

test('component: segment widths reflect partition pcts', () => {
  const rows = [row('A', 7500), row('B', 2500)];
  const { container } = renderBar(
    <CompositionBar rows={rows} total={10000} currency="USD" variant="expense" />,
  );
  const segs = container.querySelectorAll<HTMLElement>(
    '[data-testid="composition-segment"]',
  );
  expect(segs[0].style.width).toBe('75%');
  expect(segs[1].style.width).toBe('25%');
});

test('component: expense variant applies red gradient to biggest segment', () => {
  const rows = [row('A', 100)];
  const { container } = renderBar(
    <CompositionBar rows={rows} total={100} currency="USD" variant="expense" />,
  );
  const seg = container.querySelector('[data-testid="composition-segment"]');
  expect(seg?.className).toContain('bg-red-700');
});

test('component: container has role=img and an aria-label', () => {
  const rows = [row('Rent', 1800), row('Groceries', 642), row('Dining', 520)];
  const { container } = renderBar(
    <CompositionBar rows={rows} total={2962} currency="USD" variant="expense" />,
  );
  const bar = container.querySelector('[data-testid="composition-bar"]');
  expect(bar?.getAttribute('role')).toBe('img');
  expect(bar?.getAttribute('aria-label')).toMatch(/Rent/);
});
```

- [ ] **Step 2: Run tests to confirm failures**

Run: `cd spa && npm test -- reports.composition-bar`

Expected: 6 new tests fail because `CompositionBar` is not exported.

- [ ] **Step 3: Implement the component**

Append to `spa/src/components/reports/CompositionBar.tsx`:

```tsx
import { cn } from '@/lib/cn';
import type { ReportRow } from '@/lib/types';

interface Props {
  rows: ReportRow[];
  total: number;
  currency: string;
  variant: CompositionVariant;
  className?: string;
}

function buildAriaLabel(segments: CompositionSegment[], variant: CompositionVariant): string {
  const which = variant === 'income' ? 'Income' : 'Expense';
  const top = segments.slice(0, 3).map((s) => `${s.label} ${Math.round(s.pct)}%`).join(', ');
  return `${which} composition: ${top}`;
}

export function CompositionBar({ rows, total, currency: _currency, variant, className }: Props) {
  if (rows.length === 0 || total === 0) return null;
  const { segments } = partitionForComposition(rows, total, variant);
  return (
    <div
      data-testid="composition-bar"
      role="img"
      aria-label={buildAriaLabel(segments, variant)}
      className={cn('flex h-7 w-full overflow-hidden rounded text-[10px] font-medium', className)}
    >
      {segments.map((seg, i) => (
        <div
          // Segment indices are stable within a single render; rows are not
          // mutated mid-render, so index keys are safe here.
          key={i}
          data-testid="composition-segment"
          className={cn('flex items-center justify-start', seg.colorClass, seg.textClass)}
          style={{ width: `${seg.pct}%` }}
          title={`${seg.label}: ${Math.round(seg.pct)}%`}
        />
      ))}
    </div>
  );
}
```

Note: the import for `cn` and the unused-currency rename (`_currency`) prevent the lint/TS noise; currency will be used in Task 4 (tooltip).

- [ ] **Step 4: Run tests**

Run: `cd spa && npm test -- reports.composition-bar`

Expected: All 14 tests pass (8 helper + 6 component).

- [ ] **Step 5: Type-check and lint**

Run: `cd spa && npm run check && tsc -b --noEmit`

Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/reports/CompositionBar.tsx spa/src/test/reports.composition-bar.test.tsx
git commit -m "feat(spa): render CompositionBar segments with width and palette"
```

---

## Task 3: Add inline segment labels and tick markers

Show `name · NN%` inline on segments ≥9%, just `NN%` between 5–9%, nothing below 5%. Add the 0% / 50% / 100% tick row.

**Files:**
- Modify: `spa/src/components/reports/CompositionBar.tsx`
- Modify: `spa/src/test/reports.composition-bar.test.tsx`

- [ ] **Step 1: Add failing tests**

Append to `spa/src/test/reports.composition-bar.test.tsx`:

```tsx
test('inline label: segment ≥9% shows "name · NN%"', () => {
  // 1800/2962 ≈ 60.8% → ≥9%
  const rows = [row('Rent', 1800), row('Groceries', 642), row('Dining', 520)];
  const { container } = renderBar(
    <CompositionBar rows={rows} total={2962} currency="USD" variant="expense" />,
  );
  const big = container.querySelectorAll<HTMLElement>(
    '[data-testid="composition-segment"]',
  )[0];
  expect(big.textContent).toContain('Rent');
  expect(big.textContent).toMatch(/61%/);
});

test('inline label: segment between 5% and 9% shows only "NN%"', () => {
  // Construct: total 100, one row at 7
  const rows = [row('Big', 93), row('Tiny', 7)];
  const { container } = renderBar(
    <CompositionBar rows={rows} total={100} currency="USD" variant="expense" />,
  );
  const small = container.querySelectorAll<HTMLElement>(
    '[data-testid="composition-segment"]',
  )[1];
  expect(small.textContent?.trim()).toBe('7%');
  expect(small.textContent).not.toContain('Tiny');
});

test('inline label: segment <5% shows nothing', () => {
  const rows = [row('Big', 98), row('Tiny', 2)];
  const { container } = renderBar(
    <CompositionBar rows={rows} total={100} currency="USD" variant="expense" />,
  );
  const small = container.querySelectorAll<HTMLElement>(
    '[data-testid="composition-segment"]',
  )[1];
  expect(small.textContent?.trim()).toBe('');
});

test('ticks: 0% / 50% / 100% labels render below the bar', () => {
  const rows = [row('A', 100)];
  const { getByText } = renderBar(
    <CompositionBar rows={rows} total={100} currency="USD" variant="expense" />,
  );
  expect(getByText('0%')).toBeInTheDocument();
  expect(getByText('50%')).toBeInTheDocument();
  expect(getByText('100%')).toBeInTheDocument();
});
```

- [ ] **Step 2: Run tests to confirm failures**

Run: `cd spa && npm test -- reports.composition-bar`

Expected: The 4 new tests fail (no inline labels, no ticks).

- [ ] **Step 3: Update the component**

Edit `spa/src/components/reports/CompositionBar.tsx`. Replace the `CompositionBar` function body with:

```tsx
function labelFor(seg: CompositionSegment): string {
  const rounded = Math.round(seg.pct);
  if (seg.pct >= 9) return `${seg.label} · ${rounded}%`;
  if (seg.pct >= 5) return `${rounded}%`;
  return '';
}

export function CompositionBar({ rows, total, currency: _currency, variant, className }: Props) {
  if (rows.length === 0 || total === 0) return null;
  const { segments } = partitionForComposition(rows, total, variant);
  return (
    <div className={cn('w-full', className)}>
      <div
        data-testid="composition-bar"
        role="img"
        aria-label={buildAriaLabel(segments, variant)}
        className="flex h-7 w-full overflow-hidden rounded text-[10px] font-medium"
      >
        {segments.map((seg, i) => (
          <div
            key={i}
            data-testid="composition-segment"
            className={cn(
              'flex items-center overflow-hidden whitespace-nowrap',
              seg.colorClass,
              seg.textClass,
              seg.pct >= 9 ? 'px-1.5' : seg.pct >= 5 ? 'px-1 justify-center' : '',
            )}
            style={{ width: `${seg.pct}%` }}
            title={`${seg.label}: ${Math.round(seg.pct)}%`}
          >
            {labelFor(seg)}
          </div>
        ))}
      </div>
      <div className="mt-1 flex justify-between text-[9px] text-muted-foreground">
        <span>0%</span>
        <span>50%</span>
        <span>100%</span>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run tests**

Run: `cd spa && npm test -- reports.composition-bar`

Expected: All 18 tests pass.

- [ ] **Step 5: Type-check and lint**

Run: `cd spa && npm run check && tsc -b --noEmit`

Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/reports/CompositionBar.tsx spa/src/test/reports.composition-bar.test.tsx
git commit -m "feat(spa): inline segment labels and tick row for CompositionBar"
```

---

## Task 4: Add hover tooltip and keyboard focus accessibility

On hover or keyboard focus of any segment, show a tooltip with full account name, formatted amount, and percent; dim other segments. Make segments focusable buttons with the same behavior on focus.

**Files:**
- Modify: `spa/src/components/reports/CompositionBar.tsx`
- Modify: `spa/src/test/reports.composition-bar.test.tsx`

- [ ] **Step 1: Add failing tests**

Append to `spa/src/test/reports.composition-bar.test.tsx`:

```tsx
import { withServerConfig } from './test-app';
import userEvent from '@testing-library/user-event';

test('a11y: segments are focusable buttons', () => {
  const rows = [row('Rent', 1800), row('Groceries', 642)];
  const { container } = render(
    withServerConfig(
      <CompositionBar rows={rows} total={2442} currency="USD" variant="expense" />,
    ),
  );
  const segs = container.querySelectorAll<HTMLElement>(
    '[data-testid="composition-segment"]',
  );
  expect(segs[0].tagName).toBe('BUTTON');
  expect(segs[0].getAttribute('type')).toBe('button');
});

test('tooltip: hovering a segment shows name, amount, and percent', async () => {
  const user = userEvent.setup();
  const rows = [row('Rent', 180000), row('Groceries', 64200)];
  const { container, findByText } = render(
    withServerConfig(
      <CompositionBar rows={rows} total={244200} currency="USD" variant="expense" />,
    ),
  );
  const seg = container.querySelector<HTMLElement>(
    '[data-testid="composition-segment"]',
  )!;
  await user.hover(seg);
  // formatCents("USD", 180000) → "$1,800.00" with default config
  expect(await findByText('$1,800.00')).toBeInTheDocument();
  expect(await findByText('Rent')).toBeInTheDocument();
});

test('tooltip: hovered segment dims the others (opacity-60 on non-hovered)', async () => {
  const user = userEvent.setup();
  const rows = [row('A', 60), row('B', 40)];
  const { container } = render(
    withServerConfig(
      <CompositionBar rows={rows} total={100} currency="USD" variant="expense" />,
    ),
  );
  const segs = container.querySelectorAll<HTMLElement>(
    '[data-testid="composition-segment"]',
  );
  await user.hover(segs[0]);
  // Hovered segment NOT dimmed; sibling dimmed.
  expect(segs[0].className).not.toContain('opacity-60');
  expect(segs[1].className).toContain('opacity-60');
});

test('tooltip: keyboard focus also activates the tooltip', async () => {
  const user = userEvent.setup();
  const rows = [row('Rent', 180000), row('Groceries', 64200)];
  const { container, findByText } = render(
    withServerConfig(
      <CompositionBar rows={rows} total={244200} currency="USD" variant="expense" />,
    ),
  );
  const seg = container.querySelector<HTMLElement>(
    '[data-testid="composition-segment"]',
  )!;
  seg.focus();
  // Same content as hover tooltip.
  expect(await findByText('$1,800.00')).toBeInTheDocument();
  // Tab to next segment; tooltip swaps.
  await user.tab();
  expect(await findByText('$642.00')).toBeInTheDocument();
});
```

- [ ] **Step 2: Run tests to confirm failures**

Run: `cd spa && npm test -- reports.composition-bar`

Expected: 4 new tests fail.

- [ ] **Step 3: Add original account name to segments and rewrite the component**

First, extend the partition output so the tooltip has the full (non-stripped) account name and stripped label both available.

Edit `partitionForComposition` in `spa/src/components/reports/CompositionBar.tsx` — replace the primary-loop body so it stores the original name:

```tsx
// In the CompositionSegment interface, add:
//   fullName: string; // for "Other": "Other (N)"; same as label otherwise
```

Update the interface:

```tsx
export interface CompositionSegment {
  label: string;
  fullName: string;
  amount: number;
  pct: number;
  colorClass: string;
  textClass: string;
  isOther: boolean;
}
```

In `partitionForComposition`, when pushing a primary segment, include `fullName: row.account_name`. For the Other segment, set `fullName` equal to its `label` (e.g. `"Other (3)"`).

Now rewrite `CompositionBar`:

```tsx
import { useState } from 'react';
import { useAmountFormat } from '@/lib/server-config';

export function CompositionBar({ rows, total, currency, variant, className }: Props) {
  const { formatCents } = useAmountFormat();
  const [activeIdx, setActiveIdx] = useState<number | null>(null);

  if (rows.length === 0 || total === 0) return null;
  const { segments } = partitionForComposition(rows, total, variant);

  return (
    <div className={cn('w-full', className)}>
      <div className="relative">
        <div
          data-testid="composition-bar"
          role="img"
          aria-label={buildAriaLabel(segments, variant)}
          className="flex h-7 w-full overflow-hidden rounded text-[10px] font-medium"
        >
          {segments.map((seg, i) => (
            <button
              key={i}
              type="button"
              data-testid="composition-segment"
              className={cn(
                'flex items-center overflow-hidden whitespace-nowrap transition-opacity',
                seg.colorClass,
                seg.textClass,
                seg.pct >= 9 ? 'px-1.5' : seg.pct >= 5 ? 'px-1 justify-center' : '',
                activeIdx !== null && activeIdx !== i ? 'opacity-60' : '',
              )}
              style={{ width: `${seg.pct}%` }}
              onPointerEnter={() => setActiveIdx(i)}
              onPointerLeave={() => setActiveIdx((cur) => (cur === i ? null : cur))}
              onFocus={() => setActiveIdx(i)}
              onBlur={() => setActiveIdx((cur) => (cur === i ? null : cur))}
              aria-label={`${seg.fullName}: ${formatCents(seg.amount, currency)} (${Math.round(seg.pct)}%)`}
            >
              {labelFor(seg)}
            </button>
          ))}
        </div>

        {activeIdx !== null &&
          (() => {
            const seg = segments[activeIdx];
            // Anchor tooltip at the segment's left edge in % of bar width.
            // Center it under the segment using its width; flip when near the right edge.
            const leftPctSum = segments.slice(0, activeIdx).reduce((acc, s) => acc + s.pct, 0);
            const centerPct = leftPctSum + seg.pct / 2;
            const flip = centerPct > 70;
            return (
              <div
                role="status"
                aria-live="polite"
                data-testid="composition-tooltip"
                className={cn(
                  'pointer-events-none absolute z-10 mt-1 rounded-md border border-border bg-popover px-2 py-1 text-[11px] text-popover-foreground shadow-sm',
                )}
                style={{
                  left: `${centerPct}%`,
                  top: '100%',
                  transform: flip ? 'translateX(-100%)' : 'translateX(-50%)',
                }}
              >
                <div className="font-medium">{seg.fullName}</div>
                <div className="font-mono">{formatCents(seg.amount, currency)}</div>
                <div className="text-muted-foreground">{Math.round(seg.pct)}%</div>
              </div>
            );
          })()}
      </div>

      <div className="mt-1 flex justify-between text-[9px] text-muted-foreground">
        <span>0%</span>
        <span>50%</span>
        <span>100%</span>
      </div>
    </div>
  );
}
```

Remember to update the test from Task 2 that checked `seg.tagName === 'BUTTON'` — already covered in the new tests. The earlier "renders one DOM segment per partition segment" test still passes because the `data-testid` matches the `<button>`.

Update the earlier helper test for `swatchColors` parallelism and the segment interface — `fullName` is a new required field, so existing tests that destructure `segments[0]` still pass (extra field doesn't break assertions). No edits needed there.

- [ ] **Step 4: Run tests**

Run: `cd spa && npm test -- reports.composition-bar`

Expected: All 22 tests pass.

- [ ] **Step 5: Type-check and lint**

Run: `cd spa && npm run check && tsc -b --noEmit`

Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/reports/CompositionBar.tsx spa/src/test/reports.composition-bar.test.tsx
git commit -m "feat(spa): hover/focus tooltip and dimming for CompositionBar"
```

---

## Task 5: Add `swatchColors` support to `ReportRowTable`

**Files:**
- Modify: `spa/src/components/reports/ReportRowTable.tsx`
- Modify: `spa/src/test/components/BalanceColumn.test.tsx` (verify untouched — sanity)
- New tests: append to an existing or new file — use `spa/src/test/reports.composition-bar.test.tsx` for swatch-rendering tests if you prefer a focused location, otherwise create `spa/src/test/components/ReportRowTable.test.tsx`. This plan uses the latter.

- [ ] **Step 1: Create the new test file**

Create `spa/src/test/components/ReportRowTable.test.tsx`:

```tsx
import { render } from '@testing-library/react';
import { expect, test } from 'vitest';
import { ReportRowTable } from '../../components/reports/ReportRowTable';
import type { ReportRow } from '../../lib/types';
import { withServerConfig } from '../test-app';

// ReportRowTable needs a Router for <Link>. The simplest workaround for a
// unit test is to render only the swatch by stubbing the Link import via a
// vi.mock at module scope. To avoid that complexity, this test uses the
// real router-aware render path via makeTestApp in the page tests
// (Tasks 7/8). Here we just assert the swatch markup with a Router-less
// render — Link will throw without a router, so we wrap rendering in a
// try/catch and only assert on what renders before the error.
//
// Simpler approach: skip the unit test here and rely on page-level tests
// (Tasks 7/8) for swatch assertions.

// Placeholder so the file is non-empty if you choose this option.
test('see page-level tests for swatch rendering', () => {
  expect(true).toBe(true);
});
```

If you instead want a true unit test, prefer mocking `@tanstack/react-router` for this file:

```tsx
import { render } from '@testing-library/react';
import { expect, test, vi } from 'vitest';
import type { ReportRow } from '../../lib/types';
import { withServerConfig } from '../test-app';

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
}));

// Import AFTER the mock so the component picks up the stub.
import { ReportRowTable } from '../../components/reports/ReportRowTable';

const rows: ReportRow[] = [
  { account_name: 'Expenses:Rent', offset_account: '', amount: 1800, currency: 'USD', tx_count: 1 },
  { account_name: 'Expenses:Food', offset_account: '', amount: 642, currency: 'USD', tx_count: 1 },
];
const nameToId = new Map<string, number>([
  ['Expenses:Rent', 1],
  ['Expenses:Food', 2],
]);

test('ReportRowTable: renders no swatches when swatchColors is undefined', () => {
  const { container } = render(
    withServerConfig(
      <ReportRowTable rows={rows} currency="USD" nameToId={nameToId} period={null} />,
    ),
  );
  expect(container.querySelectorAll('[data-testid="row-swatch"]').length).toBe(0);
});

test('ReportRowTable: renders one swatch per row when swatchColors provided', () => {
  const { container } = render(
    withServerConfig(
      <ReportRowTable
        rows={rows}
        currency="USD"
        nameToId={nameToId}
        period={null}
        swatchColors={['bg-red-700', 'bg-red-600']}
      />,
    ),
  );
  const swatches = container.querySelectorAll('[data-testid="row-swatch"]');
  expect(swatches).toHaveLength(2);
  expect(swatches[0].className).toContain('bg-red-700');
  expect(swatches[1].className).toContain('bg-red-600');
});
```

Use the second (real-test) version of the file. Delete the placeholder version.

- [ ] **Step 2: Run tests to confirm failures**

Run: `cd spa && npm test -- ReportRowTable`

Expected: Both tests fail — `swatchColors` prop not recognized; no swatch element.

- [ ] **Step 3: Update `ReportRowTable.tsx`**

Edit `spa/src/components/reports/ReportRowTable.tsx`:

- Add to `interface Props`:

```ts
swatchColors?: string[]; // parallel to rows; if provided, renders a colored swatch before each account name
```

- Update the function signature to accept it:

```tsx
export function ReportRowTable({ rows, currency, nameToId, period, swatchColors }: Props) {
```

- Inside the row map, change `linkContent` to include the swatch when present:

```tsx
const swatch =
  swatchColors && swatchColors[/* rowIndex */ ] ? (
    <span
      data-testid="row-swatch"
      className={cn('mr-2 inline-block h-2 w-2 rounded-[2px] align-middle', swatchColors[/* rowIndex */])}
      aria-hidden="true"
    />
  ) : null;

const linkContent = (
  <span className="inline-flex items-center min-w-0">
    {swatch}
    <span className="truncate" title={row.account_name}>
      {stripAccountTypePrefix(row.account_name)}
    </span>
  </span>
);
```

You'll need the row index — change `rows.map((row) => …)` to `rows.map((row, rowIndex) => …)` and substitute `rowIndex` in the two `[/* rowIndex */]` placeholders above.

Add `import { cn } from '@/lib/cn';` to the imports.

- [ ] **Step 4: Run tests**

Run: `cd spa && npm test -- ReportRowTable`

Expected: Both tests pass.

Also run the full SPA test suite to make sure no existing test broke:

```bash
cd spa && npm test
```

Expected: all tests pass (the composition-bar suite still passes, and existing pages that don't pass `swatchColors` still render exactly the same DOM minus a wrapping `<span class="inline-flex …">` — verify by spot-checking failures, if any. If wrapping the link content changed test expectations in `reports.balance-sheet.test.tsx` or similar, adjust the inline-flex wrap so behavior matches more closely; see Step 5).

- [ ] **Step 5: If wrapping the link content broke an existing test, narrow the change**

Only wrap with `inline-flex` when `swatchColors` is provided; otherwise leave the original `<span class="truncate">` exactly as it was. Update the JSX:

```tsx
const labelSpan = (
  <span className="truncate" title={row.account_name}>
    {stripAccountTypePrefix(row.account_name)}
  </span>
);

const linkContent = swatch ? (
  <span className="inline-flex min-w-0 items-center">
    {swatch}
    {labelSpan}
  </span>
) : (
  labelSpan
);
```

Re-run `cd spa && npm test` — all tests pass.

- [ ] **Step 6: Type-check and lint**

Run: `cd spa && npm run check && tsc -b --noEmit`

Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add spa/src/components/reports/ReportRowTable.tsx spa/src/test/components/ReportRowTable.test.tsx
git commit -m "feat(spa): optional colored swatches in ReportRowTable"
```

---

## Task 6: Wire CompositionBar into Expense Breakdown page

**Files:**
- Modify: `spa/src/routes/reports.expense-breakdown.tsx`

- [ ] **Step 1: Edit the route**

Open `spa/src/routes/reports.expense-breakdown.tsx`.

- Remove the import:

```tsx
import { ProportionBar } from '@/components/reports/ProportionBar';
```

- Add:

```tsx
import { CompositionBar, partitionForComposition } from '@/components/reports/CompositionBar';
```

- Replace the line:

```tsx
<ProportionBar rows={rows} total={expense} currency={currency} variant="expense" />
```

with:

```tsx
<section>
  <h2 className="mb-2 text-sm font-semibold">Composition</h2>
  <CompositionBar rows={rows} total={expense} currency={currency} variant="expense" />
</section>
```

- Just above the existing `<ReportRowTable …>`, compute swatch colors:

```tsx
const { swatchColors } = partitionForComposition(rows, expense, 'expense');
```

Add this inside the component, after `const rows = …` and before the `return`. (Recomputation on each render is fine — pure function, O(n log n) where n is small.)

- Update the table prop list:

```tsx
<ReportRowTable
  rows={rows}
  currency={currency}
  nameToId={nameToId}
  period={{ startUnix: period.startUnix, endUnix: period.endUnix }}
  swatchColors={swatchColors}
/>
```

- [ ] **Step 2: Type-check and lint**

Run: `cd spa && npm run check && tsc -b --noEmit`

Expected: clean. If `ProportionBar` still appears in an unused-import error, remove the leftover import line.

- [ ] **Step 3: Run page tests**

Run: `cd spa && npm test -- reports.expense-breakdown`

Expected: existing test (`renders total expense KPI and rows`) still passes — the assertions check KPI text and row labels, not the chart.

- [ ] **Step 4: Commit**

```bash
git add spa/src/routes/reports.expense-breakdown.tsx
git commit -m "feat(spa): use CompositionBar on Expense Breakdown page"
```

---

## Task 7: Wire CompositionBar into Income Breakdown page

**Files:**
- Modify: `spa/src/routes/reports.income-breakdown.tsx`

- [ ] **Step 1: Edit the route**

Open `spa/src/routes/reports.income-breakdown.tsx`. Apply the same changes as Task 6, with these differences:

- `variant="income"`
- Replace `<ProportionBar rows={rows} total={income} currency={currency} variant="income" />` with the new heading + `<CompositionBar … variant="income">`.
- Compute swatches as: `const { swatchColors } = partitionForComposition(rows, income, 'income');`
- Pass `swatchColors` to `<ReportRowTable>`.

- [ ] **Step 2: Type-check and lint**

Run: `cd spa && npm run check && tsc -b --noEmit`

Expected: clean.

- [ ] **Step 3: Run page tests**

Run: `cd spa && npm test -- reports.income-breakdown`

Expected: existing test passes unchanged.

- [ ] **Step 4: Commit**

```bash
git add spa/src/routes/reports.income-breakdown.tsx
git commit -m "feat(spa): use CompositionBar on Income Breakdown page"
```

---

## Task 8: Add page-level assertions for the new bar and swatches

Add a small assertion to each breakdown page test that the composition bar is rendered and that the table rows include swatches.

**Files:**
- Modify: `spa/src/test/reports.expense-breakdown.test.tsx`
- Modify: `spa/src/test/reports.income-breakdown.test.tsx`

- [ ] **Step 1: Edit `reports.expense-breakdown.test.tsx`**

In the existing test, add assertions after the KPI/row checks:

```tsx
// Composition bar is rendered.
const bar = document.querySelector('[data-testid="composition-bar"]');
expect(bar).not.toBeNull();
expect(document.querySelectorAll('[data-testid="composition-segment"]').length).toBe(2);

// Table rows have colored swatches matching the bar segments.
const swatches = document.querySelectorAll('[data-testid="row-swatch"]');
expect(swatches.length).toBe(2);
// Top expense (Rent, 180000) is the darkest red.
expect(swatches[0].className).toContain('bg-red-700');
```

(The fixture has `expense_rows` of length 2: Rent then Food, both within top-6 so no Other.)

Note: the existing fixture has `Expenses:Rent` first in the rows array, but `partitionForComposition` sorts by `|amount|` desc, and Rent's amount is larger than Food's. So the row at table index 0 (Rent) gets the darkest swatch — matches the assertion.

- [ ] **Step 2: Edit `reports.income-breakdown.test.tsx`**

Same pattern, for income:

```tsx
const bar = document.querySelector('[data-testid="composition-bar"]');
expect(bar).not.toBeNull();
expect(document.querySelectorAll('[data-testid="composition-segment"]').length).toBe(2);

const swatches = document.querySelectorAll('[data-testid="row-swatch"]');
expect(swatches.length).toBe(2);
// Top income (Salary, 520000) is the darkest emerald.
expect(swatches[0].className).toContain('bg-emerald-700');
```

- [ ] **Step 3: Run both page tests**

Run: `cd spa && npm test -- reports.income-breakdown reports.expense-breakdown`

Expected: both files pass.

- [ ] **Step 4: Run the full SPA suite**

Run: `cd spa && npm test`

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add spa/src/test/reports.expense-breakdown.test.tsx spa/src/test/reports.income-breakdown.test.tsx
git commit -m "test(spa): assert CompositionBar and swatches on breakdown pages"
```

---

## Task 9: Manual smoke check and final commit (if needed)

The SPA bundles into `internal/web/dist/`. The dev server gives the fastest visual loop.

- [ ] **Step 1: Start the dev server**

```bash
cd spa && npm run dev
```

This serves the SPA at `http://localhost:5173` and proxies API calls to the Go server. In another terminal, start the Go server against a real or seeded ledger:

```bash
go run ./cmd/kea serve
```

- [ ] **Step 2: Verify Expense Breakdown**

Navigate to `http://localhost:5173/reports/expense-breakdown`. Confirm:

- The Composition heading and bar render above the detail table.
- Segments are red, darkest on the biggest. "Other" (if >6 expense categories) is grey at the end.
- Hover a segment → tooltip shows full account name, amount in the right currency, and percent. Other segments dim.
- Tab into the bar with the keyboard → focus moves segment-to-segment, tooltip swaps each time.
- The detail table shows a small colored dot before each account name, matching the bar.
- 0% / 50% / 100% ticks render under the bar.

- [ ] **Step 3: Verify Income Breakdown**

Same checks at `http://localhost:5173/reports/income-breakdown`, with emerald-green palette.

- [ ] **Step 4: Verify Income Statement is unchanged**

Navigate to `http://localhost:5173/reports/income-statement`. Confirm the existing Top 5 Income / Top 5 Expense `ProportionBar`s render exactly as before.

- [ ] **Step 5: If everything looks correct, no commit needed**

The previous tasks have already committed the implementation. If you spotted a visual bug, fix it in a small additional commit referencing the specific issue.

- [ ] **Step 6: Build the bundle**

Run: `cd spa && npm run build`

Expected: build succeeds. The built bundle replaces files under `internal/web/dist/`; commit only `internal/web/dist/index.html` if it changed (per `.gitignore`, only the placeholder is tracked).

```bash
git status internal/web/dist/
# Commit only if index.html changed:
git add internal/web/dist/index.html && git commit -m "build: refresh embedded SPA bundle placeholder"
```

---

## Self-Review Notes

- All spec requirements have a task: partition + Other (T1), bar widths and gradient (T2), inline labels and ticks (T3), tooltip + focus + dim + a11y (T4), table swatches (T5), wiring + tests + smoke (T6–T9).
- `ProportionBar` deliberately stays alive (Income Statement still uses it) — matches spec's "Out of scope".
- Type consistency: `CompositionSegment` is defined in T1, extended in T4 with `fullName`. Both interfaces are listed explicitly in their tasks. `partitionForComposition`'s signature is stable from T1 onward.
- No placeholders: every step has the code/command/expected output it needs.
