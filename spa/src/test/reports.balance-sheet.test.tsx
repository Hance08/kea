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
      if (url === '/api/reports/net-worth-series')
        return Promise.resolve(
          okResponse({
            items: [
              {
                currency: 'USD',
                points: [
                  { date: '2026-01-01', balance: 50000 },
                  { date: '2026-01-02', balance: 100000 },
                  { date: '2026-01-03', balance: 125000 },
                ],
              },
            ],
          }),
        );
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('renders 4-column KPI grid with Net Worth and hides empty Liabilities card', async () => {
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    expect(screen.getByText('Total Assets')).toBeInTheDocument();
  });
  expect(screen.getByText('Total Equity')).toBeInTheDocument();
  expect(screen.getByText('Net Worth')).toBeInTheDocument();
  expect(screen.queryByText('Total Liabilities')).not.toBeInTheDocument();
});

test('removes Asset mix, Assets, and Equity sections', async () => {
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    expect(screen.getByText('Total Assets')).toBeInTheDocument();
  });
  expect(screen.queryByRole('heading', { name: 'Asset mix' })).toBeNull();
  expect(screen.queryByRole('heading', { name: 'Assets' })).toBeNull();
  expect(screen.queryByRole('heading', { name: 'Equity' })).toBeNull();
});

test('renders the Net worth over time chart heading', async () => {
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /net worth over time/i })).toBeInTheDocument();
  });
});
