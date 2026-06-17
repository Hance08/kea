import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config')
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      if (url === '/api/ledgers')
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      if (url === '/api/balances')
        return Promise.resolve(
          okResponse({
            items: [
              {
                account_id: 1,
                name: 'Assets:Bank',
                type: 'A',
                currency: 'USD',
                amount: 125000,
                is_hidden: false,
              },
            ],
            total_count: 1,
            limit: 0,
            offset: 0,
          }),
        );
      if (url.startsWith('/api/reports/balance-sheet'))
        return Promise.resolve(
          okResponse({
            assets: [
              {
                account_name: 'Assets:Bank',
                offset_account: '',
                amount: 125000,
                currency: 'USD',
                tx_count: 4,
              },
            ],
            liabilities: [],
            equity: [
              {
                account_name: 'Equity:OpeningBalances_USD',
                offset_account: '',
                amount: 125000,
                currency: 'USD',
                tx_count: 1,
              },
            ],
            total_assets: { USD: 125000 },
            total_liabilities: {},
            total_equity: { USD: 125000 },
            net_worth: { USD: 125000 },
            as_of: 1781697600,
          }),
        );
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('renders Assets and Equity sections; hides empty Liabilities', async () => {
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    expect(screen.getByText('Total Assets')).toBeInTheDocument();
  });
  expect(screen.getByText('Total Equity')).toBeInTheDocument();
  expect(screen.queryByText('Total Liabilities')).not.toBeInTheDocument();
  // Account-type prefix stripped in rendered text on report pages.
  expect(screen.getAllByText('Bank').length).toBeGreaterThan(0);
  expect(screen.queryByText('Assets:Bank')).toBeNull();
});

test('asset rows link to /accounts/{id}', async () => {
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    expect(screen.getAllByText('Bank').length).toBeGreaterThan(0);
  });
  const link = screen.getByRole('link', { name: /Bank/ });
  expect(link.getAttribute('href')).toMatch(/\/accounts\/1/);
});
