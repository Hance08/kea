import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

vi.mock('@/lib/accounts', async () => {
  const leafWithTx = {
    id: 5,
    name: 'Assets:Bank:Spending',
    type: 'A' as const,
    parent_id: 2,
    currency: 'USD',
    description: '',
    is_hidden: false,
  };
  const tree = [{ account: leafWithTx, children: [] }];

  return {
    ...(await vi.importActual<object>('@/lib/accounts')),
    getAccount: vi.fn().mockResolvedValue(leafWithTx),
    getAccountBalance: vi.fn().mockResolvedValue({
      account_id: 5,
      amount: 1000,
      currency: 'USD',
    }),
    getAccountTree: vi.fn().mockResolvedValue(tree),
  };
});

vi.mock('@/lib/transactions', async () => ({
  ...(await vi.importActual<object>('@/lib/transactions')),
  listTransactions: vi.fn().mockResolvedValue({
    items: [],
    total_count: 7,
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

describe('delete button blocked states', () => {
  it('disables delete with tooltip when account has transactions', async () => {
    render(makeTestApp('/accounts/5'));
    const button = await waitFor(() => screen.getByRole('button', { name: /^delete$/i }));
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('title', expect.stringContaining('7 transactions'));
  });
});
