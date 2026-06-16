import { expect, test } from 'vitest';
import { parseAsOfSearch, parseAtSearch, parsePeriodSearch } from './reports-search-params';

test('period schema defaults to this-month with no params', () => {
  expect(parsePeriodSearch({})).toEqual({ range: 'this-month' });
});

test('period schema preserves valid range', () => {
  expect(parsePeriodSearch({ range: 'ytd' })).toEqual({ range: 'ytd' });
});

test('period schema accepts custom with from/to', () => {
  expect(parsePeriodSearch({ range: 'custom', from: '2026-01-01', to: '2026-01-31' })).toEqual({
    range: 'custom',
    from: '2026-01-01',
    to: '2026-01-31',
  });
});

test('period schema accepts month override', () => {
  expect(parsePeriodSearch({ range: 'this-month', month: '2025-12' })).toEqual({
    range: 'this-month',
    month: '2025-12',
  });
});

test('period schema rejects bad range and falls back to this-month', () => {
  expect(parsePeriodSearch({ range: 'forever' })).toEqual({ range: 'this-month' });
});

test('period schema rejects bad month format', () => {
  expect(parsePeriodSearch({ range: 'this-month', month: '2026-13' })).toEqual({
    range: 'this-month',
  });
});

test('as-of schema accepts a numeric as_of', () => {
  expect(parseAsOfSearch({ as_of: 1781697600 })).toEqual({ as_of: 1781697600 });
});

test('as-of schema coerces string', () => {
  expect(parseAsOfSearch({ as_of: '1781697600' })).toEqual({ as_of: 1781697600 });
});

test('as-of schema returns empty object when missing', () => {
  expect(parseAsOfSearch({})).toEqual({});
});

test('at schema accepts numeric at', () => {
  expect(parseAtSearch({ at: 1781697600 })).toEqual({ at: 1781697600 });
});

test('at schema returns empty object when missing', () => {
  expect(parseAtSearch({})).toEqual({});
});
