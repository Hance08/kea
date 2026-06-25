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
  // Account-type prefix stripped in rendered text on report pages.
  expect(screen.getAllByText('Rent').length).toBeGreaterThan(0);
  expect(screen.queryByText('Expenses:Rent')).toBeNull();

  // Composition bar is rendered.
  const bar = document.querySelector('[data-testid="composition-bar"]');
  expect(bar).not.toBeNull();
  expect(document.querySelectorAll('[data-testid="composition-segment"]').length).toBe(2);

  // Table rows have colored swatches matching the bar segments.
  const swatches = document.querySelectorAll('[data-testid="row-swatch"]');
  expect(swatches.length).toBe(2);
  // Top expense (Rent, 180000) is the darkest red.
  expect(swatches[0].className).toContain('bg-red-700');
});

test('renders Regular/Irregular subLine on Total Expense KPI', async () => {
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
        return Promise.resolve(okResponse({ items: [], total_count: 0, limit: 0, offset: 0 }));
      if (url.startsWith('/api/reports/expense-breakdown'))
        return Promise.resolve(
          okResponse({
            period: 'June 2026',
            total_income: {},
            total_expense: { USD: 228000 },
            total_expense_regular: { USD: 180000 },
            total_expense_irregular: { USD: 48000 },
            net_amount: {},
            net_worth: {},
            previous_net_worth: {},
            net_worth_growth_pct: {},
            income_rows: [],
            expense_rows: [
              {
                account_name: 'Expenses:Rent',
                offset_account: '',
                amount: 228000,
                currency: 'USD',
                tx_count: 1,
              },
            ],
          }),
        );
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
  render(makeTestApp('/reports/expense-breakdown'));
  await waitFor(() => {
    expect(screen.getByText('Total Expense')).toBeInTheDocument();
  });
  const expenseLine = await screen.findByText(/Regular .*\$1,800\.00.*Irregular .*\$480\.00/);
  expect(expenseLine).toBeInTheDocument();
});
