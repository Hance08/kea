import { describe, expect, test } from 'vitest';
import { formatCents } from './format';

describe('formatCents', () => {
  test('formats positive cents as USD', () => {
    expect(formatCents(125000, 'USD')).toBe('$1,250.00');
  });

  test('formats zero as $0.00 in USD', () => {
    expect(formatCents(0, 'USD')).toBe('$0.00');
  });

  test('formats negative cents with minus prefix', () => {
    expect(formatCents(-42000, 'USD')).toBe('-$420.00');
  });

  test('formats TWD with currency symbol', () => {
    // Intl formats TWD with NT$ symbol; fraction digits vary by runtime ICU data
    const result = formatCents(125000, 'TWD');
    expect(result).toMatch(/NT\$/);
    expect(result).toContain('1,250');
  });

  test('falls back gracefully on unknown currency code', () => {
    // Intl will throw on bogus codes; helper must handle.
    expect(() => formatCents(100, 'ZZZ')).not.toThrow();
  });
});
