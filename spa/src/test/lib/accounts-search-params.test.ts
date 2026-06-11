import { parseAccountsSearch } from '@/lib/accounts-search-params';
import { describe, expect, it } from 'vitest';

describe('parseAccountsSearch', () => {
  it('returns defaults when nothing is set', () => {
    expect(parseAccountsSearch({})).toEqual({ include_hidden: false, show_parents: false });
  });

  it('coerces include_hidden from string', () => {
    expect(parseAccountsSearch({ include_hidden: 'true' }).include_hidden).toBe(true);
    expect(parseAccountsSearch({ include_hidden: 'false' }).include_hidden).toBe(false);
  });

  it('coerces show_parents from string and defaults to false', () => {
    expect(parseAccountsSearch({}).show_parents).toBe(false);
    expect(parseAccountsSearch({ show_parents: 'true' }).show_parents).toBe(true);
    expect(parseAccountsSearch({ show_parents: 'false' }).show_parents).toBe(false);
  });

  it('keeps q and type when valid', () => {
    expect(parseAccountsSearch({ q: 'bank', type: 'A' })).toEqual({
      q: 'bank',
      type: 'A',
      include_hidden: false,
      show_parents: false,
    });
  });

  it('drops unknown fields and rejects invalid type', () => {
    expect(() => parseAccountsSearch({ type: 'X' })).toThrow();
  });
});
