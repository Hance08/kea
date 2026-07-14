import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

vi.mock('@/lib/accounts', async () => {
  const assetLeaf = {
    id: 3,
    name: 'Assets:Bank:Checking',
    type: 'A' as const,
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
  const assetParent = {
    id: 2,
    name: 'Assets:Bank',
    type: 'A' as const,
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
  // 10 asset leaves (ids 10..19) used by the pagination tests.
  const manyAssetLeaves = Array.from({ length: 10 }, (_, i) => ({
    id: 10 + i,
    name: `Assets:Many:${i + 1}`,
    type: 'A' as const,
    currency: 'USD',
    is_hidden: false,
    description: '',
  }));
  const tree = [
    { account: assetParent, children: [{ account: assetLeaf, children: [] }] },
    { account: liabilityLeaf, children: [] },
    { account: expenseLeaf, children: [] },
    ...manyAssetLeaves.map((a) => ({ account: a, children: [] })),
  ];
  return {
    ...(await vi.importActual<object>('@/lib/accounts')),
    getAccountTree: vi.fn().mockResolvedValue(tree),
  };
});

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi
    .fn()
    .mockResolvedValue({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
  getBalances: vi.fn().mockResolvedValue({
    items: [
      {
        account_id: 3,
        name: 'Assets:Bank:Checking',
        type: 'A',
        currency: 'USD',
        amount: 250000,
        is_hidden: false,
      },
      {
        account_id: 4,
        name: 'Liabilities:CreditCard',
        type: 'L',
        currency: 'USD',
        amount: -50000,
        is_hidden: false,
      },
    ],
    total_count: 2,
    limit: 0,
    offset: 0,
  }),
}));

describe('/reconcile chooser', () => {
  it('lists asset and liability leaves, hides parents and other types', async () => {
    render(makeTestApp('/reconcile'));
    await waitFor(() => expect(screen.getByText('Assets:Bank:Checking')).toBeInTheDocument());
    expect(screen.getByText('Liabilities:CreditCard')).toBeInTheDocument();
    expect(screen.queryByText('Assets:Bank')).not.toBeInTheDocument(); // parent
    expect(screen.queryByText('Expenses:Coffee')).not.toBeInTheDocument(); // expense leaf
  });

  it('renders a link to /reconcile/$id for each account', async () => {
    render(makeTestApp('/reconcile'));
    await waitFor(() => expect(screen.getByText('Assets:Bank:Checking')).toBeInTheDocument());
    const link = screen.getByText('Assets:Bank:Checking').closest('a');
    expect(link).toHaveAttribute('href', '/reconcile/3');
    const link2 = screen.getByText('Liabilities:CreditCard').closest('a');
    expect(link2).toHaveAttribute('href', '/reconcile/4');
  });

  it('paginates to 8 rows per page and advances on Next', async () => {
    render(makeTestApp('/reconcile'));
    await waitFor(() => expect(screen.getByText('Assets:Bank:Checking')).toBeInTheDocument());
    // 12 total leaves (1 + 1 + 10) shown 8 at a time.
    const visibleLinks = () =>
      Array.from(document.querySelectorAll('a[href^="/reconcile/"]'));
    expect(visibleLinks()).toHaveLength(8);
    expect(screen.getByText(/Page 1 of 2/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /next/i }));
    await waitFor(() => expect(screen.getByText(/Page 2 of 2/)).toBeInTheDocument());
    expect(visibleLinks()).toHaveLength(4); // 12 total − 8 on page 1 = 4 on page 2
  });
});
