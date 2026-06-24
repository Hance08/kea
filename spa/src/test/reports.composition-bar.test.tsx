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
