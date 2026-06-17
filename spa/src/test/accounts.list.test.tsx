import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

vi.mock('@/lib/accounts', async () => {
  const accounts = [
    {
      id: 1,
      name: 'Assets',
      type: 'A' as const,
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
    {
      id: 2,
      name: 'Assets:Bank',
      type: 'A' as const,
      parent_id: 1,
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
    {
      id: 3,
      name: 'Assets:Bank:Checking',
      type: 'A' as const,
      parent_id: 2,
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
  ];
  return {
    ...(await vi.importActual<object>('@/lib/accounts')),
    listAccounts: vi.fn().mockResolvedValue({
      items: accounts,
      total_count: 3,
      limit: 0,
      offset: 0,
    }),
  };
});

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi
    .fn()
    .mockResolvedValue({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
  getBalances: vi.fn().mockResolvedValue({
    items: [
      { account_id: 1, name: 'Assets', type: 'A', currency: 'USD', amount: 0, is_hidden: false },
      {
        account_id: 2,
        name: 'Assets:Bank',
        type: 'A',
        parent_id: 1,
        currency: 'USD',
        amount: 0,
        is_hidden: false,
      },
      {
        account_id: 3,
        name: 'Assets:Bank:Checking',
        type: 'A',
        parent_id: 2,
        currency: 'USD',
        amount: 3200,
        is_hidden: false,
      },
    ],
    total_count: 3,
    limit: 0,
    offset: 0,
  }),
}));

describe('accounts list', () => {
  it('hides parent accounts by default — leaf only', async () => {
    render(makeTestApp('/accounts'));
    await waitFor(() => expect(screen.getByText('Assets:Bank:Checking')).toBeInTheDocument());
    expect(screen.queryByText('Assets:Bank')).not.toBeInTheDocument();
    expect(screen.queryByText(/^Assets$/)).not.toBeInTheDocument();
  });

  it('shows parents when show_parents=true', async () => {
    render(makeTestApp('/accounts?show_parents=true'));
    await waitFor(() => expect(screen.getByText('Assets:Bank:Checking')).toBeInTheDocument());
    expect(screen.getByText('Assets:Bank')).toBeInTheDocument();
    expect(screen.getByText(/^Assets$/)).toBeInTheDocument();
  });
});
