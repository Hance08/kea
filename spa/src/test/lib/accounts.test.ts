import { getAccountTree, isOpeningBalancesAccount } from '@/lib/accounts';
import { describe, expect, it, vi } from 'vitest';

describe('isOpeningBalancesAccount', () => {
  it('matches per-currency system accounts', () => {
    expect(isOpeningBalancesAccount('Equity:OpeningBalances_USD')).toBe(true);
    expect(isOpeningBalancesAccount('Equity:OpeningBalances_EUR')).toBe(true);
  });

  it('does not match the legacy single-name system account', () => {
    expect(isOpeningBalancesAccount('Equity:OpeningBalances')).toBe(false);
  });

  it('does not match unrelated accounts', () => {
    expect(isOpeningBalancesAccount('Equity:RetainedEarnings')).toBe(false);
    expect(isOpeningBalancesAccount('Assets:Bank:Checking')).toBe(false);
    expect(isOpeningBalancesAccount('')).toBe(false);
  });
});

describe('getAccountTree', () => {
  it('normalizes null children to empty arrays at every depth', async () => {
    const originalFetch = global.fetch;
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve([
          {
            account: {
              id: 1,
              name: 'Assets',
              type: 'A',
              currency: 'USD',
              description: '',
              is_hidden: false,
            },
            children: [
              {
                account: {
                  id: 2,
                  name: 'Assets:Bank',
                  type: 'A',
                  parent_id: 1,
                  currency: 'USD',
                  description: '',
                  is_hidden: false,
                },
                children: null, // simulates Go nil slice
              },
            ],
          },
          {
            account: {
              id: 3,
              name: 'Equity',
              type: 'C',
              currency: 'USD',
              description: '',
              is_hidden: false,
            },
            children: null,
          },
        ]),
    } as unknown as Response);

    const tree = await getAccountTree();
    global.fetch = originalFetch;

    expect(tree[0].children).toEqual([
      expect.objectContaining({
        account: expect.objectContaining({ id: 2 }),
        children: [],
      }),
    ]);
    expect(tree[1].children).toEqual([]);
  });
});
