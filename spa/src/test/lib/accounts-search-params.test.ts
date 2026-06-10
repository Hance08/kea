import { parseAccountsSearch } from '@/lib/accounts-search-params';
import { describe, expect, it } from 'vitest';

describe('parseAccountsSearch', () => {
  it('returns defaults when nothing is set', () => {
    expect(parseAccountsSearch({})).toEqual({ include_hidden: false });
  });

  it('coerces include_hidden from string', () => {
    expect(parseAccountsSearch({ include_hidden: 'true' }).include_hidden).toBe(true);
    expect(parseAccountsSearch({ include_hidden: 'false' }).include_hidden).toBe(false);
  });

  it('keeps q and type when valid', () => {
    expect(parseAccountsSearch({ q: 'bank', type: 'A' })).toEqual({
      q: 'bank',
      type: 'A',
      include_hidden: false,
    });
  });

  it('drops unknown fields and rejects invalid type', () => {
    expect(() => parseAccountsSearch({ type: 'X' })).toThrow();
  });
});
