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
      // Tab-nav tests don't care about report data; return empty payloads.
      if (url === '/api/balances') {
        return Promise.resolve(okResponse({ items: [], total_count: 0, limit: 0, offset: 0 }));
      }
      if (url.startsWith('/api/reports/')) {
        return Promise.resolve(
          okResponse({
            period: '',
            total_income: {},
            total_expense: {},
            net_amount: {},
            net_worth: {},
            previous_net_worth: {},
            net_worth_growth_pct: {},
            income_rows: [],
            expense_rows: [],
            assets: [],
            liabilities: [],
            equity: [],
            total_assets: {},
            total_liabilities: {},
            total_equity: {},
            as_of: 0,
            at: 0,
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('renders all 4 report tabs', async () => {
  render(makeTestApp('/reports/income-statement'));
  await waitFor(() => {
    expect(screen.getByRole('link', { name: /Income Statement/i })).toBeInTheDocument();
  });
  expect(screen.getByRole('link', { name: /Income Breakdown/i })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: /Expense Breakdown/i })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: /Balance Sheet/i })).toBeInTheDocument();
  expect(screen.queryByRole('link', { name: /Net Worth/i })).toBeNull();
});

test('marks the active tab', async () => {
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    expect(screen.getByRole('link', { name: /Balance Sheet/i })).toBeInTheDocument();
  });
  const active = screen.getByRole('link', { name: /Balance Sheet/i });
  expect(active.getAttribute('aria-current')).toBe('page');
});
