import { render as rtlRender } from '@testing-library/react';
import type { ReactNode } from 'react';
import { expect, test } from 'vitest';
import { CompositionBar, partitionForComposition } from '../components/reports/CompositionBar';
import type { ReportRow } from '../lib/types';
import { withServerConfig } from './test-app';

const row = (name: string, amount: number, offset = ''): ReportRow => ({
  account_name: name,
  offset_account: offset,
  amount,
  currency: 'USD',
  tx_count: 1,
});

test('partition: returns one segment per row when rows ≤ 6', () => {
  const rows = [row('Rent', 1800), row('Groceries', 642), row('Dining', 520)];
  const { segments, swatchColors } = partitionForComposition(rows, 2962, 'expense');
  expect(segments).toHaveLength(3);
  expect(segments[0].label).toBe('Rent');
  expect(segments[0].amount).toBe(1800);
  expect(segments[0].isOther).toBe(false);
  expect(swatchColors).toHaveLength(3);
});

test('partition: rows beyond top 6 collapse into a single Other segment', () => {
  const rows = [
    row('A', 1000),
    row('B', 900),
    row('C', 800),
    row('D', 700),
    row('E', 600),
    row('F', 500),
    row('G', 400),
    row('H', 300),
    row('I', 200),
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
  const rows = [row('Small', 100), row('Big', 5000), row('Mid', 1000)];
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
  expect(partitionForComposition(rows, 100, 'income').segments[0].colorClass).toBe(
    'bg-emerald-700',
  );
});

test('partition: swatchColors is parallel to input rows (not segment order)', () => {
  // Big-Mid-Small is the sort order for segments, but the table needs
  // colors in input order: Small, Big, Mid.
  const rows = [row('Small', 100), row('Big', 5000), row('Mid', 1000)];
  const { swatchColors, segments } = partitionForComposition(rows, 6100, 'expense');
  // The "Big" row (input index 1) maps to the darkest shade.
  expect(swatchColors[1]).toBe(segments[0].colorClass);
  // "Mid" → second darkest, "Small" → third darkest.
  expect(swatchColors[2]).toBe(segments[1].colorClass);
  expect(swatchColors[0]).toBe(segments[2].colorClass);
});

test('partition: rows beyond top 6 get the Other (neutral) swatch', () => {
  const rows = [
    row('A', 1000),
    row('B', 900),
    row('C', 800),
    row('D', 700),
    row('E', 600),
    row('F', 500),
    row('G', 400),
  ];
  const { swatchColors, segments } = partitionForComposition(rows, 4900, 'expense');
  const otherSeg = segments.find((s) => s.isOther);
  expect(otherSeg).toBeDefined();
  expect(swatchColors[6]).toBe(otherSeg?.colorClass); // G is collapsed
});

test('partition: pct is clamped to 100 when |amount| exceeds |total|', () => {
  // Mixed-sign rows can produce a single |amount| > |total|.
  // Without clamping, pct would be 160 here.
  const rows = [row('A', 8000), row('B', -3000)];
  const { segments } = partitionForComposition(rows, 5000, 'expense');
  expect(segments[0].pct).toBeLessThanOrEqual(100);
  expect(segments[0].pct).toBe(100);
});

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
  const segs = container.querySelectorAll<HTMLElement>('[data-testid="composition-segment"]');
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
