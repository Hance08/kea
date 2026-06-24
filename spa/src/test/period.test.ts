import { describe, expect, test } from 'vitest';
import { previousPeriod, resolvePeriod } from '../lib/period';

// Helper: build a `nowUnix` for a given UTC date.
function utc(year: number, month1to12: number, day: number): number {
  return Date.UTC(year, month1to12 - 1, day) / 1000;
}

describe('previousPeriod', () => {
  test('this-month: previous calendar month', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'this-month' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { month: '2026-05' },
    });
  });

  test('this-month: January rolls back into prior December', () => {
    const now = utc(2026, 1, 15);
    const search = { range: 'this-month' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { month: '2025-12' },
    });
  });

  test('this-month with explicit month string', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'this-month' as const, month: '2026-03' };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { month: '2026-02' },
    });
  });

  test('last-month: the month before last', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'last-month' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { month: '2026-04' },
    });
  });

  test('ytd: same MM-DD window in the previous year', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'ytd' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { from: '2025-01-01', to: '2025-06-17' },
    });
  });

  test('last-12mo: 12 months ending one day before current start', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'last-12mo' as const };
    const resolved = resolvePeriod(search, now);
    // current window starts at 2025-06-17; previous ends 2025-06-16, starts 2024-06-16.
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { from: '2024-06-16', to: '2025-06-16' },
    });
  });

  test('custom: same-length window ending one day before from', () => {
    const now = utc(2026, 6, 17);
    const search = {
      range: 'custom' as const,
      from: '2026-03-01',
      to: '2026-03-31',
    };
    const resolved = resolvePeriod(search, now);
    // March has 31 days; previous window is 31 days ending 2026-02-28 → starts 2026-01-29.
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { from: '2026-01-29', to: '2026-02-28' },
    });
  });

  test('custom with missing from/to returns null', () => {
    const now = utc(2026, 6, 17);
    const search = { range: 'custom' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toBeNull();
  });

  test('ytd: Feb 29 of a leap year clamps to Feb 28 in previous (non-leap) year', () => {
    const now = utc(2028, 2, 29);
    const search = { range: 'ytd' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { from: '2027-01-01', to: '2027-02-28' },
    });
  });

  test('ytd: non-leap-day still preserves exact MM-DD', () => {
    const now = utc(2028, 3, 1);
    const search = { range: 'ytd' as const };
    const resolved = resolvePeriod(search, now);
    expect(previousPeriod(search, resolved, now)).toEqual({
      apiParams: { from: '2027-01-01', to: '2027-03-01' },
    });
  });
});

describe('resolvePeriod: integer Unix timestamps', () => {
  // Regression: ResolvedPeriod's startUnix/endUnix flow into URL search params
  // (start_time/end_time) that the transactions page validates as int. A
  // fractional nowUnix (the default — Date.now()/1000) must not leak through.
  const fractionalNow = 1717200000.456;

  test.each([
    ['this-month', { range: 'this-month' } as const],
    ['last-month', { range: 'last-month' } as const],
    ['ytd', { range: 'ytd' } as const],
    ['last-12mo', { range: 'last-12mo' } as const],
    ['custom', { range: 'custom', from: '2026-01-15', to: '2026-03-20' } as const],
  ])('%s returns integer startUnix and endUnix', (_label, search) => {
    const resolved = resolvePeriod(search, fractionalNow);
    expect(Number.isInteger(resolved.startUnix)).toBe(true);
    expect(Number.isInteger(resolved.endUnix)).toBe(true);
  });
});
