import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const REPORT_PAYLOAD = {
  period: 'June 2026',
  total_income: { USD: 524800 },
  total_expense: { USD: 253000 },
  net_amount: { USD: 271800 },
  net_worth: { USD: 5000000 },
  previous_net_worth: { USD: 4700000 },
  net_worth_growth_pct: { USD: 6.2 },
  income_rows: [
    {
      account_name: 'Income:Salary',
      offset_account: 'Assets:Bank',
      amount: 520000,
      currency: 'USD',
      tx_count: 1,
    },
  ],
  expense_rows: [
    {
      account_name: 'Expenses:Rent',
      offset_account: 'Assets:Bank',
      amount: 180000,
      currency: 'USD',
      tx_count: 1,
    },
    {
      account_name: 'Expenses:Food',
      offset_account: 'Assets:Bank',
      amount: 73000,
      currency: 'USD',
      tx_count: 4,
    },
  ],
};

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      }
      if (url === '/api/balances') {
        return Promise.resolve(
          okResponse({
            items: [
              {
                account_id: 11,
                name: 'Expenses:Rent',
                type: 'E',
                currency: 'USD',
                amount: 180000,
                is_hidden: false,
              },
              {
                account_id: 12,
                name: 'Expenses:Food',
                type: 'E',
                currency: 'USD',
                amount: 73000,
                is_hidden: false,
              },
              {
                account_id: 13,
                name: 'Income:Salary',
                type: 'R',
                currency: 'USD',
                amount: -520000,
                is_hidden: false,
              },
            ],
            total_count: 3,
            limit: 0,
            offset: 0,
          }),
        );
      }
      if (url.startsWith('/api/reports/income-statement')) {
        return Promise.resolve(okResponse(REPORT_PAYLOAD));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('renders KPI cards, charts, and tables for income statement', async () => {
  render(makeTestApp('/reports/income-statement'));
  await waitFor(() => {
    expect(screen.getByText('Income')).toBeInTheDocument();
  });
  expect(screen.getByText('Expense')).toBeInTheDocument();
  expect(screen.getByText('Net')).toBeInTheDocument();
  expect(screen.getByText('$5,248.00')).toBeInTheDocument();
  expect(screen.getByText('$2,530.00')).toBeInTheDocument();
  expect(screen.getByText('$2,718.00')).toBeInTheDocument();
  expect(screen.getByText(/6\.2% net worth/)).toBeInTheDocument();
  expect(screen.getAllByText('Expenses:Rent').length).toBeGreaterThan(0);
  expect(screen.getAllByText('Income:Salary').length).toBeGreaterThan(0);
});

test('drill-down row links to /transactions with account_id and time bounds', async () => {
  render(makeTestApp('/reports/income-statement'));
  await waitFor(() => {
    expect(screen.getAllByText('Expenses:Rent').length).toBeGreaterThan(0);
  });
  const link = screen.getByRole('link', { name: /Expenses:Rent/ });
  const href = link.getAttribute('href') ?? '';
  expect(href).toContain('/transactions');
  expect(href).toContain('account_id=11');
});

test('renders empty state when there are no rows', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      }
      if (url === '/api/balances') {
        return Promise.resolve(okResponse({ items: [], total_count: 0, limit: 0, offset: 0 }));
      }
      if (url.startsWith('/api/reports/income-statement')) {
        return Promise.resolve(
          okResponse({
            ...REPORT_PAYLOAD,
            total_income: {},
            total_expense: {},
            net_amount: {},
            income_rows: [],
            expense_rows: [],
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
  render(makeTestApp('/reports/income-statement'));
  await waitFor(() => {
    expect(screen.getByText(/no income or expenses/i)).toBeInTheDocument();
  });
});
