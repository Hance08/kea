import { describe, expect, test } from 'vitest';
import {
  parseTransactionsSearch,
  searchToFilter,
  searchToListOptions,
  transactionsSearchSchema,
} from '@/lib/transactions-search-params';

describe('transactionsSearchSchema', () => {
  test('all empty → defaults to limit=50 offset=0', () => {
    expect(transactionsSearchSchema.parse({})).toEqual({ limit: 50, offset: 0 });
  });

  test('valid full search', () => {
    expect(
      transactionsSearchSchema.parse({
        account_id: '3',
        type: 'Expense',
        status: 'Cleared',
        start_time: '1700000000',
        end_time: '1733000000',
        description: 'coffee',
        limit: '20',
        offset: '40',
      }),
    ).toEqual({
      account_id: 3,
      type: 'Expense',
      status: 'Cleared',
      start_time: 1700000000,
      end_time: 1733000000,
      description: 'coffee',
      limit: 20,
      offset: 40,
    });
  });

  test('invalid type rejected', () => {
    expect(() => transactionsSearchSchema.parse({ type: 'Bogus' })).toThrow();
  });

  test('negative offset rejected', () => {
    expect(() => transactionsSearchSchema.parse({ offset: '-1' })).toThrow();
  });

  test('zero limit rejected (must be positive)', () => {
    expect(() => transactionsSearchSchema.parse({ limit: '0' })).toThrow();
  });
});

describe('searchToFilter / searchToListOptions', () => {
  test('extracts filter fields only', () => {
    const search = transactionsSearchSchema.parse({
      account_id: '3',
      type: 'Income',
      limit: '10',
      offset: '20',
    });
    expect(searchToFilter(search)).toEqual({ account_id: 3, type: 'Income' });
    expect(searchToListOptions(search)).toEqual({
      limit: 10,
      offset: 20,
      include_count: true,
    });
  });
});

describe('parseTransactionsSearch (lenient wrapper)', () => {
  test('returns defaults when input fails validation entirely', () => {
    expect(parseTransactionsSearch({ type: 'Bogus' })).toEqual({ limit: 50, offset: 0 });
  });
});
