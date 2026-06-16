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
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      if (url === '/api/ledgers')
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      if (url === '/api/balances')
        return Promise.resolve(okResponse({ items: [], total_count: 0, limit: 0, offset: 0 }));
      if (url.startsWith('/api/reports/expense-breakdown'))
        return Promise.resolve(
          okResponse({
            period: 'June 2026',
            total_income: {},
            total_expense: { USD: 253000 },
            net_amount: {},
            net_worth: {},
            previous_net_worth: {},
            net_worth_growth_pct: {},
            income_rows: [],
            expense_rows: [
              {
                account_name: 'Expenses:Rent',
                offset_account: '',
                amount: 180000,
                currency: 'USD',
                tx_count: 1,
              },
              {
                account_name: 'Expenses:Food',
                offset_account: '',
                amount: 73000,
                currency: 'USD',
                tx_count: 5,
              },
            ],
          }),
        );
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('renders total expense KPI and rows', async () => {
  render(makeTestApp('/reports/expense-breakdown'));
  await waitFor(() => {
    expect(screen.getByText('Total Expense')).toBeInTheDocument();
  });
  expect(screen.getByText('$2,530.00')).toBeInTheDocument();
  expect(screen.getAllByText('Expenses:Rent').length).toBeGreaterThan(0);
});
