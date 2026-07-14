import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

vi.mock('@/lib/accounts', async () => {
  const assetLeaf = {
    id: 3,
    name: 'Assets:Bank:Checking',
    type: 'A' as const,
    parent_id: 2,
    currency: 'USD',
    is_hidden: false,
    description: '',
  };
  const liabilityLeaf = {
    id: 4,
    name: 'Liabilities:CreditCard',
    type: 'L' as const,
    currency: 'USD',
    is_hidden: false,
    description: '',
  };
  const expenseLeaf = {
    id: 7,
    name: 'Expenses:Coffee',
    type: 'E' as const,
    currency: 'USD',
    is_hidden: false,
    description: '',
  };
  const assetParent = {
    id: 2,
    name: 'Assets:Bank',
    type: 'A' as const,
    currency: 'USD',
    is_hidden: false,
    description: '',
  };
  const tree = [
    { account: assetParent, children: [{ account: assetLeaf, children: [] }] },
    { account: liabilityLeaf, children: [] },
    { account: expenseLeaf, children: [] },
  ];

  return {
    ...(await vi.importActual<object>('@/lib/accounts')),
    getAccount: vi.fn(async (id: number) => {
      if (id === 3) return assetLeaf;
      if (id === 4) return liabilityLeaf;
      if (id === 7) return expenseLeaf;
      return assetParent;
    }),
    getAccountBalance: vi.fn().mockResolvedValue({ account_id: 0, amount: 0, currency: 'USD' }),
    getAccountTree: vi.fn().mockResolvedValue(tree),
  };
});

vi.mock('@/lib/transactions', async () => ({
  ...(await vi.importActual<object>('@/lib/transactions')),
  listTransactions: vi.fn().mockResolvedValue({ items: [], total_count: 0, limit: 20, offset: 0 }),
}));

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi
    .fn()
    .mockResolvedValue({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
  getBalances: vi.fn().mockResolvedValue({ items: [], total_count: 0, limit: 0, offset: 0 }),
}));

// The sidebar always renders a "Reconcile" link pointing at /reconcile.
// These helpers filter that out so we assert specifically on the in-header
// deep-link button.
function reconcileDeepLinks() {
  return screen
    .queryAllByRole('link', { name: /^reconcile$/i })
    .filter((el) => el.getAttribute('href') !== '/reconcile');
}

describe('AccountDetailHeader Reconcile button', () => {
  it('shows Reconcile for an Asset leaf account', async () => {
    render(makeTestApp('/accounts/3'));
    await waitFor(() => {
      const links = reconcileDeepLinks();
      expect(links).toHaveLength(1);
      expect(links[0]).toHaveAttribute('href', '/reconcile/3');
    });
  });

  it('shows Reconcile for a Liability leaf account', async () => {
    render(makeTestApp('/accounts/4'));
    await waitFor(() => {
      const links = reconcileDeepLinks();
      expect(links).toHaveLength(1);
      expect(links[0]).toHaveAttribute('href', '/reconcile/4');
    });
  });

  it('hides Reconcile for an Expense leaf account', async () => {
    render(makeTestApp('/accounts/7'));
    await waitFor(() => expect(screen.getByText('Expenses:Coffee')).toBeInTheDocument());
    expect(reconcileDeepLinks()).toHaveLength(0);
  });

  it('hides Reconcile for a parent Asset account', async () => {
    render(makeTestApp('/accounts/2'));
    await waitFor(() => expect(screen.getByText('Assets:Bank')).toBeInTheDocument());
    expect(reconcileDeepLinks()).toHaveLength(0);
  });
});
