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
      if (url === '/api/config') {
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
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
                account_id: 13,
                name: 'Income:Salary',
                type: 'R',
                currency: 'USD',
                amount: -520000,
                is_hidden: false,
              },
            ],
            total_count: 1,
            limit: 0,
            offset: 0,
          }),
        );
      }
      if (url.startsWith('/api/reports/income-breakdown')) {
        return Promise.resolve(
          okResponse({
            period: 'June 2026',
            total_income: { USD: 524800 },
            total_expense: {},
            net_amount: {},
            net_worth: {},
            previous_net_worth: {},
            net_worth_growth_pct: {},
            income_rows: [
              {
                account_name: 'Income:Salary',
                offset_account: 'Assets:Bank',
                amount: 520000,
                currency: 'USD',
                tx_count: 1,
              },
              {
                account_name: 'Income:Interest',
                offset_account: 'Assets:Bank',
                amount: 4800,
                currency: 'USD',
                tx_count: 1,
              },
            ],
            expense_rows: [],
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('renders total income KPI and rows', async () => {
  render(makeTestApp('/reports/income-breakdown'));
  await waitFor(() => {
    expect(screen.getByText('Total Income')).toBeInTheDocument();
  });
  expect(screen.getByText('$5,248.00')).toBeInTheDocument();
  expect(screen.getAllByText('Income:Salary').length).toBeGreaterThan(0);
  expect(screen.getAllByText('Income:Interest').length).toBeGreaterThan(0);

  // Composition bar is rendered.
  const bar = document.querySelector('[data-testid="composition-bar"]');
  expect(bar).not.toBeNull();
  expect(document.querySelectorAll('[data-testid="composition-segment"]').length).toBe(2);

  // Table rows have colored swatches matching the bar segments.
  const swatches = document.querySelectorAll('[data-testid="row-swatch"]');
  expect(swatches.length).toBe(2);
  // Top income (Salary, 520000) is the darkest emerald.
  expect(swatches[0].className).toContain('bg-emerald-700');
});
