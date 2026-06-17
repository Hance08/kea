import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

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
        amount: 3200,
        is_hidden: false,
      },
    ],
    total_count: 1,
    limit: 0,
    offset: 0,
  }),
}));

describe('balances row link', () => {
  it('renders each row as a link to its account detail', async () => {
    render(makeTestApp('/balances'));
    const link = await waitFor(() => screen.getByRole('link', { name: /Assets:Bank:Checking/ }));
    expect(link.getAttribute('href')).toMatch(/^\/accounts\/3(\?|$)/);
  });
});
