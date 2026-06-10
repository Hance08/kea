import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

vi.mock('@/lib/accounts', async () => {
  const leaf = {
    id: 3,
    name: 'Assets:Bank:Checking',
    type: 'A' as const,
    parent_id: 2,
    currency: 'USD',
    description: '',
    is_hidden: false,
  };
  const parent = {
    id: 2,
    name: 'Assets:Bank',
    type: 'A' as const,
    parent_id: 1,
    currency: 'USD',
    description: '',
    is_hidden: false,
  };
  const system = {
    id: 99,
    name: 'Equity:OpeningBalances_USD',
    type: 'C' as const,
    currency: 'USD',
    description: '',
    is_hidden: false,
  };
  const tree = [
    { account: parent, children: [{ account: leaf, children: [] }] },
    { account: system, children: [] },
  ];

  return {
    ...(await vi.importActual<object>('@/lib/accounts')),
    getAccount: vi.fn((id: number) =>
      Promise.resolve(id === 3 ? leaf : id === 2 ? parent : system),
    ),
    getAccountBalance: vi.fn(() => Promise.resolve({ account_id: 0, amount: 0, currency: 'USD' })),
    getAccountTree: vi.fn().mockResolvedValue(tree),
  };
});

vi.mock('@/lib/transactions', async () => ({
  ...(await vi.importActual<object>('@/lib/transactions')),
  listTransactions: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 20,
    offset: 0,
  }),
}));

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi.fn().mockResolvedValue({ defaults: { currency: 'USD' } }),
  getBalances: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 0,
    offset: 0,
  }),
}));

describe('accounts detail page', () => {
  it('renders recent transactions card for a leaf', async () => {
    render(makeTestApp('/accounts/3'));
    await waitFor(() => expect(screen.getByText(/recent transactions/i)).toBeInTheDocument());
  });

  it('renders child accounts card for a parent', async () => {
    render(makeTestApp('/accounts/2'));
    await waitFor(() => expect(screen.getByText(/child accounts/i)).toBeInTheDocument());
  });

  it('renders the system banner and hides delete for a system account', async () => {
    render(makeTestApp('/accounts/99'));
    await waitFor(() => expect(screen.getByText(/system account/i)).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /^delete$/i })).not.toBeInTheDocument();
  });
});
