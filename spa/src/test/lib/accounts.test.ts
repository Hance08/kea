import {
  balanceColor,
  getAccountTree,
  isOpeningBalancesAccount,
  listAccounts,
  naturalAmount,
} from '@/lib/accounts';
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

describe('naturalAmount', () => {
  it('returns the raw stored amount for Asset / Equity / Expense', () => {
    expect(naturalAmount('A', 5000)).toBe(5000);
    expect(naturalAmount('C', 2000)).toBe(2000);
    expect(naturalAmount('E', 1500)).toBe(1500);
  });

  it('negates the stored amount for Liability so descending sort puts biggest debt on top', () => {
    expect(naturalAmount('L', -100000)).toBe(100000);
    expect(naturalAmount('L', -1000)).toBe(1000);
  });

  it('negates the stored amount for Revenue so descending sort puts biggest income on top', () => {
    expect(naturalAmount('R', -50000)).toBe(50000);
    expect(naturalAmount('R', -100)).toBe(100);
  });
});

describe('balanceColor', () => {
  it('uses raw sign for Asset / Liability / Equity', () => {
    expect(balanceColor('A', 5000)).toBe('text-green-600');
    expect(balanceColor('A', -100)).toBe('text-red-600');
    expect(balanceColor('L', -5000)).toBe('text-red-600');
    expect(balanceColor('C', 1000)).toBe('text-green-600');
  });

  it('inverts the sign mapping for Revenue and Expense', () => {
    // Revenue stored negative when earned → should display green.
    expect(balanceColor('R', -5000)).toBe('text-green-600');
    // Revenue stored positive (rare — refund) → red.
    expect(balanceColor('R', 100)).toBe('text-red-600');
    // Expense stored positive when spent → red.
    expect(balanceColor('E', 2000)).toBe('text-red-600');
    // Expense stored negative (refund) → green.
    expect(balanceColor('E', -50)).toBe('text-green-600');
  });

  it('returns empty string for zero', () => {
    expect(balanceColor('A', 0)).toBe('');
    expect(balanceColor('R', 0)).toBe('');
  });
});
