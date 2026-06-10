import { getAccountTree, isOpeningBalancesAccount, listAccounts } from '@/lib/accounts';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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

describe('listAccounts URL routing', () => {
  const captured: string[] = [];
  let originalFetch: typeof global.fetch;

  beforeEach(() => {
    captured.length = 0;
    originalFetch = global.fetch;
    global.fetch = vi.fn().mockImplementation((url: string) => {
      captured.push(url);
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ items: [], total_count: 0, limit: 0, offset: 0 }),
      } as unknown as Response);
    });
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it('omits include_count when no q is given (routes to ListAccounts path)', async () => {
    await listAccounts({ type: 'C', include_hidden: false });
    expect(captured[0]).not.toContain('include_count');
    expect(captured[0]).not.toContain('limit=');
    expect(captured[0]).toContain('type=C');
  });

  it('sends include_count and limit when q is given (routes to SearchAccounts path)', async () => {
    await listAccounts({ q: 'bank', type: 'A', limit: 100 });
    expect(captured[0]).toContain('include_count=true');
    expect(captured[0]).toContain('limit=100');
    expect(captured[0]).toContain('q=bank');
  });
});
