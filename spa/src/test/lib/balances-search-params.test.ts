import { parseBalancesSearch } from '@/lib/balances-search-params';
import { describe, expect, it } from 'vitest';

describe('parseBalancesSearch', () => {
  it('returns defaults when nothing is set', () => {
    expect(parseBalancesSearch({})).toEqual({
      a_offset: 0,
      a_sort: 'balance_desc',
      l_offset: 0,
      l_sort: 'balance_desc',
    });
  });

  it('coerces offsets from query strings', () => {
    expect(parseBalancesSearch({ a_offset: '16', l_offset: '8' })).toEqual({
      a_offset: 16,
      a_sort: 'balance_desc',
      l_offset: 8,
      l_sort: 'balance_desc',
    });
  });

  it('accepts both sort values per side', () => {
    expect(parseBalancesSearch({ a_sort: 'balance_asc', l_sort: 'balance_desc' })).toEqual({
      a_offset: 0,
      a_sort: 'balance_asc',
      l_offset: 0,
      l_sort: 'balance_desc',
    });
  });

  it('rejects negative offsets', () => {
    expect(() => parseBalancesSearch({ a_offset: '-1' })).toThrow();
  });

  it('rejects unknown sort values', () => {
    expect(() => parseBalancesSearch({ a_sort: 'name_asc' })).toThrow();
  });
});
