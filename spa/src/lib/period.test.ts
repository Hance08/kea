import { expect, test } from 'vitest';
import { type PeriodSearch, resolvePeriod } from './period';

// Fix "now" so tests are deterministic. 2026-06-16T12:00:00Z → 1781697600.
const NOW = new Date('2026-06-16T12:00:00Z').getTime() / 1000;

function call(search: Partial<PeriodSearch>) {
  return resolvePeriod({ range: 'this-month', ...search }, NOW);
}

test('this-month resolves to current month', () => {
  const r = call({ range: 'this-month' });
  expect(r.apiParams).toEqual({ month: '2026-06' });
  expect(r.label).toBe('June 2026');
  expect(new Date(r.startUnix * 1000).toISOString()).toBe('2026-06-01T00:00:00.000Z');
  // end is last second of the month
  expect(new Date(r.endUnix * 1000).toISOString()).toBe('2026-06-30T23:59:59.000Z');
});

test('last-month resolves to previous month', () => {
  const r = call({ range: 'last-month' });
  expect(r.apiParams).toEqual({ month: '2026-05' });
  expect(r.label).toBe('May 2026');
});

test('last-month rolls over from January', () => {
  const jan = new Date('2026-01-15T00:00:00Z').getTime() / 1000;
  const r = resolvePeriod({ range: 'last-month' }, jan);
  expect(r.apiParams).toEqual({ month: '2025-12' });
  expect(r.label).toBe('December 2025');
});

test('ytd resolves from Jan 1 to today', () => {
  const r = call({ range: 'ytd' });
  expect(r.apiParams).toEqual({ from: '2026-01-01', to: '2026-06-16' });
  expect(r.label).toBe('YTD 2026');
});

test('last-12mo resolves to a 12-month window ending today', () => {
  const r = call({ range: 'last-12mo' });
  expect(r.apiParams).toEqual({ from: '2025-06-16', to: '2026-06-16' });
  expect(r.label).toBe('Last 12 months');
});

test('custom uses provided from/to dates', () => {
  const r = call({ range: 'custom', from: '2026-03-01', to: '2026-03-31' });
  expect(r.apiParams).toEqual({ from: '2026-03-01', to: '2026-03-31' });
  expect(r.label).toBe('Mar 1 – Mar 31, 2026');
});

test('custom spans a year boundary', () => {
  const r = call({ range: 'custom', from: '2025-12-01', to: '2026-01-31' });
  expect(r.label).toBe('Dec 1, 2025 – Jan 31, 2026');
});

test('custom without from/to falls back to this-month', () => {
  const r = call({ range: 'custom' });
  expect(r.apiParams).toEqual({ month: '2026-06' });
});

test('this-month with month override uses that month', () => {
  const r = call({ range: 'this-month', month: '2026-02' });
  expect(r.apiParams).toEqual({ month: '2026-02' });
  expect(r.label).toBe('February 2026');
  // Feb 2026 has 28 days
  expect(new Date(r.endUnix * 1000).toISOString()).toBe('2026-02-28T23:59:59.000Z');
});

test('leap year February gets 29 days', () => {
  const r = call({ range: 'this-month', month: '2024-02' });
  expect(new Date(r.endUnix * 1000).toISOString()).toBe('2024-02-29T23:59:59.000Z');
});
