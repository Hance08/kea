import { afterEach, beforeEach, describe, expect, test } from 'vitest';
import {
  clearFilters,
  getActiveLedger,
  loadFilters,
  saveFilters,
  searchEquals,
  setActiveLedger,
} from '../../lib/filter-memory';

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  localStorage.clear();
});

describe('active ledger mirror', () => {
  test('getActiveLedger returns null when unset', () => {
    expect(getActiveLedger()).toBeNull();
  });

  test('setActiveLedger then getActiveLedger round-trips', () => {
    setActiveLedger('personal');
    expect(getActiveLedger()).toBe('personal');
  });

  test('setActiveLedger overwrites previous value', () => {
    setActiveLedger('personal');
    setActiveLedger('work');
    expect(getActiveLedger()).toBe('work');
  });
});

describe('per-page filter storage', () => {
  test('loadFilters returns null when no ledger active', () => {
    expect(loadFilters('transactions')).toBeNull();
  });

  test('loadFilters returns null when key absent', () => {
    setActiveLedger('personal');
    expect(loadFilters('transactions')).toBeNull();
  });

  test('saveFilters then loadFilters round-trips', () => {
    setActiveLedger('personal');
    saveFilters('transactions', { limit: 10, offset: 20, type: 'Expense' });
    expect(loadFilters('transactions')).toEqual({
      limit: 10,
      offset: 20,
      type: 'Expense',
    });
  });

  test('per-ledger isolation: filters under ledger A do not surface under ledger B', () => {
    setActiveLedger('personal');
    saveFilters('transactions', { offset: 10 });
    setActiveLedger('work');
    expect(loadFilters('transactions')).toBeNull();
    setActiveLedger('personal');
    expect(loadFilters('transactions')).toEqual({ offset: 10 });
  });

  test('clearFilters removes only the (ledger, page) key', () => {
    setActiveLedger('personal');
    saveFilters('transactions', { offset: 10 });
    saveFilters('accounts', { q: 'bank' });
    clearFilters('transactions');
    expect(loadFilters('transactions')).toBeNull();
    expect(loadFilters('accounts')).toEqual({ q: 'bank' });
  });

  test('loadFilters returns null on malformed JSON', () => {
    setActiveLedger('personal');
    localStorage.setItem('kea.filters.personal.transactions', 'not json');
    expect(loadFilters('transactions')).toBeNull();
  });

  test('saveFilters is a no-op when no ledger active', () => {
    saveFilters('transactions', { offset: 10 });
    setActiveLedger('personal');
    expect(loadFilters('transactions')).toBeNull();
  });
});

describe('searchEquals', () => {
  test('returns true for identical objects', () => {
    expect(searchEquals({ limit: 10, offset: 0 }, { limit: 10, offset: 0 })).toBe(true);
  });

  test('returns false when keys differ', () => {
    expect(searchEquals({ limit: 10 }, { limit: 10, offset: 0 })).toBe(false);
  });

  test('returns false when values differ', () => {
    expect(searchEquals({ limit: 10 }, { limit: 20 })).toBe(false);
  });

  test('treats undefined values as absent', () => {
    expect(searchEquals({ limit: 10, type: undefined }, { limit: 10 })).toBe(true);
  });

  test('ignores key order', () => {
    expect(searchEquals({ a: 1, b: 2 }, { b: 2, a: 1 })).toBe(true);
  });
});
