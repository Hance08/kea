import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

vi.mock('@/lib/accounts', async () => {
  const actual = await vi.importActual<object>('@/lib/accounts');
  const tree = [
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
          children: [
            {
              account: {
                id: 3,
                name: 'Assets:Bank:Checking',
                type: 'A',
                parent_id: 2,
                currency: 'USD',
                description: '',
                is_hidden: false,
              },
              children: [],
            },
          ],
        },
      ],
    },
  ];
  return {
    ...actual,
    getAccountTree: vi.fn().mockResolvedValue(tree),
  };
});

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getBalances: vi.fn().mockResolvedValue({
    items: [
      {
        account_id: 1,
        name: 'Assets',
        type: 'A',
        currency: 'USD',
        amount: 12340,
        is_hidden: false,
      },
      {
        account_id: 2,
        name: 'Assets:Bank',
        type: 'A',
        currency: 'USD',
        amount: 8200,
        is_hidden: false,
      },
      {
        account_id: 3,
        name: 'Assets:Bank:Checking',
        type: 'A',
        currency: 'USD',
        amount: 3200,
        is_hidden: false,
      },
    ],
    total_count: 3,
    limit: 0,
    offset: 0,
  }),
  getConfig: vi.fn().mockResolvedValue({ defaults: { currency: 'USD' } }),
}));

describe('accounts list — tree mode', () => {
  it('renders root accounts and expands on chevron click', async () => {
    render(makeTestApp('/accounts'));
    await waitFor(() => expect(screen.getByText('Assets')).toBeInTheDocument());

    // 'Bank' is hidden until the root is expanded.
    expect(screen.queryByText('Bank')).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /expand/i }));
    await waitFor(() => expect(screen.getByText('Bank')).toBeInTheDocument());
  });
});
