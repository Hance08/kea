import { describe, expect, test } from 'vitest';
import { formatBalanceAbs, formatCents } from './format';

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

describe('formatCents with hideDecimals: true', () => {
  test('drops decimals for whole values', () => {
    expect(formatCents(260000, 'USD', { hideDecimals: true })).toBe('$2,600');
  });

  test('rounds half away from zero (positive)', () => {
    expect(formatCents(260050, 'USD', { hideDecimals: true })).toBe('$2,601');
  });

  test('rounds half away from zero (negative)', () => {
    expect(formatCents(-260050, 'USD', { hideDecimals: true })).toBe('-$2,601');
  });

  test('does not round below half', () => {
    expect(formatCents(260049, 'USD', { hideDecimals: true })).toBe('$2,600');
  });

  test('falls back gracefully on unknown currency', () => {
    expect(() => formatCents(260050, 'ZZZ', { hideDecimals: true })).not.toThrow();
  });
});

describe('formatBalanceAbs', () => {
  test('renders two decimals by default', () => {
    expect(formatBalanceAbs(260000)).toBe('$2,600.00');
    expect(formatBalanceAbs(-42050)).toBe('$420.50');
  });

  test('drops decimals when hideDecimals is true', () => {
    expect(formatBalanceAbs(260000, { hideDecimals: true })).toBe('$2,600');
  });

  test('rounds and strips sign when hideDecimals is true', () => {
    expect(formatBalanceAbs(-260050, { hideDecimals: true })).toBe('$2,601');
  });
});
