import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

vi.mock('@/lib/accounts', async () => {
  const matches = [
    {
      id: 3,
      name: 'Assets:Bank:Checking',
      type: 'A' as const,
      parent_id: 2,
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
    {
      id: 4,
      name: 'Assets:Bank:Savings',
      type: 'A' as const,
      parent_id: 2,
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
  ];
  return {
    ...(await vi.importActual<object>('@/lib/accounts')),
    getAccountTree: vi.fn().mockResolvedValue([]),
    listAccounts: vi.fn().mockResolvedValue({
      items: matches,
      total_count: 2,
      limit: 100,
      offset: 0,
    }),
  };
});

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getBalances: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 0,
    offset: 0,
  }),
  getConfig: vi.fn().mockResolvedValue({ defaults: { currency: 'USD' } }),
}));

describe('accounts list — search mode', () => {
  it('renders flat results when q is typed', async () => {
    render(makeTestApp('/accounts'));
    const input = await screen.findByPlaceholderText(/search accounts/i);
    await userEvent.type(input, 'bank');

    await waitFor(() => expect(screen.getByText('Assets:Bank:Checking')).toBeInTheDocument());
    expect(screen.getByText('Assets:Bank:Savings')).toBeInTheDocument();
  });

  it('uses flat mode when only a type filter is set (no q)', async () => {
    render(makeTestApp('/accounts?type=A'));
    // The flat-mode SearchMode component issues listAccounts. If we land here
    // without typing into the search box, the matches mock above resolves and
    // the flat table renders.
    await waitFor(() => expect(screen.getByText('Assets:Bank:Checking')).toBeInTheDocument());
  });
});
