import { describe, test, expect } from 'vitest';
import { regularSubLine } from '@/lib/reportSubLine';

const fakeFormat = (cents: number, currency: string) =>
  `${currency} ${(cents / 100).toFixed(2)}`;

describe('regularSubLine', () => {
  test('returns formatted line with both values', () => {
    expect(regularSubLine(500_000, 30_000, 'USD', fakeFormat)).toBe(
      'Regular USD 5000.00 · Irregular USD 300.00',
    );
  });

  test('returns the line when only regular is non-zero', () => {
    expect(regularSubLine(500_000, 0, 'USD', fakeFormat)).toBe(
      'Regular USD 5000.00 · Irregular USD 0.00',
    );
  });

  test('returns the line when only irregular is non-zero', () => {
    expect(regularSubLine(0, 30_000, 'USD', fakeFormat)).toBe(
      'Regular USD 0.00 · Irregular USD 300.00',
    );
  });

  test('returns undefined when both are zero', () => {
    expect(regularSubLine(0, 0, 'USD', fakeFormat)).toBeUndefined();
  });
});
