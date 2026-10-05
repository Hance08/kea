import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { currentMonth } from '../lib/budgets';
import type { BudgetReport } from '../lib/types';
import { makeTestApp } from './test-app';

const ok = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

function row(
  name: string,
  budget: number,
  actual: number,
  extra: Partial<BudgetReport['rows'][0]> = {},
) {
  return {
    account_id: 1,
    account_name: name,
    currency: 'USD',
    effective_month: '2026-01',
    budget,
    actual,
    actual_regular: 0,
    actual_irregular: actual,
    remaining: budget - actual,
    excluded_accounts: [],
    ...extra,
  };
}

let report: BudgetReport;
let reportUrls: string[];

beforeEach(() => {
  // The page remembers its month per ledger; start every test from a clean slate.
  localStorage.clear();
  reportUrls = [];
  report = {
    month: currentMonth(),
    rows: [
      row('Expenses:Food', 100000, 50000, { excluded_accounts: ['Expenses:Food:Japan'] }),
      row('Expenses:Food:Dining', 60000, 64100),
      row('Expenses:Rent', 200000, 170000),
      row('Expenses:Gifts', 0, 1000),
    ],
    total_budget: { USD: 300000, TWD: 500000 },
    total_actual: { USD: 221000, TWD: 0 },
  };
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config')
        return Promise.resolve(
          ok({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      if (url === '/api/ledgers')
        return Promise.resolve(
          ok({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      if (url.startsWith('/api/reports/budget')) {
        reportUrls.push(url);
        return Promise.resolve(ok(report));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('renders rows with status colors, indentation and per-currency totals', async () => {
  render(makeTestApp('/reports/budget'));
  await waitFor(() => expect(screen.getAllByTestId('budget-bar').length).toBe(4));

  const bars = screen.getAllByTestId('budget-bar');
  expect(bars.map((b) => b.getAttribute('data-status'))).toEqual(['ok', 'over', 'warning', 'over']);

  const dining = screen
    .getByText('Food:Dining')
    .closest('[data-testid="budget-row"]') as HTMLElement;
  expect(dining.getAttribute('data-depth')).toBe('1');

  expect(screen.getByText('Total (USD)')).toBeInTheDocument();
  expect(screen.getByText('Total (TWD)')).toBeInTheDocument();
});

test('shows the time marker for the current month', async () => {
  render(makeTestApp('/reports/budget'));
  await waitFor(() => expect(screen.getAllByTestId('budget-bar').length).toBe(4));
  expect(screen.getAllByTestId('time-marker').length).toBe(4);
  expect(reportUrls[0]).toBe('/api/reports/budget');
});

test('hides the time marker for a past month', async () => {
  report = { ...report, month: '2020-01' };
  render(makeTestApp('/reports/budget?month=2020-01'));
  await waitFor(() => expect(screen.getAllByTestId('budget-bar').length).toBe(4));
  expect(reportUrls).toContain('/api/reports/budget?month=2020-01');
  expect(screen.queryAllByTestId('time-marker').length).toBe(0);
});

test('excluded accounts produce a warning with their names', async () => {
  render(makeTestApp('/reports/budget'));
  const warn = await screen.findByTestId('excluded-warning');
  expect(warn.getAttribute('title')).toContain('Expenses:Food:Japan');
});

test('zero budget with spending reads as over', async () => {
  render(makeTestApp('/reports/budget'));
  const gifts = (await screen.findByText('Gifts')).closest(
    '[data-testid="budget-row"]',
  ) as HTMLElement;
  expect(within(gifts).getByText('over')).toBeInTheDocument();
});

test('empty month links to the budgets page', async () => {
  report = { month: currentMonth(), rows: [], total_budget: {}, total_actual: {} };
  render(makeTestApp('/reports/budget'));
  const link = await screen.findByRole('link', { name: /set up budgets/i });
  expect(link.getAttribute('href')).toBe('/budgets');
});

test('previous-month button navigates with month param', async () => {
  render(makeTestApp('/reports/budget'));
  await waitFor(() => expect(screen.getAllByTestId('budget-bar').length).toBe(4));
  await userEvent.click(screen.getByRole('button', { name: /previous month/i }));
  await waitFor(() => expect(reportUrls.some((u) => u.includes('month='))).toBe(true));
});
