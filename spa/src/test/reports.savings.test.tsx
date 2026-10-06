import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { currentMonth } from '../lib/budgets';
import type { SavingsReport, SavingsReportRow } from '../lib/types';
import { makeTestApp } from './test-app';

const ok = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

function row(name: string, target: number, saved: number, extra: Partial<SavingsReportRow> = {}) {
  return {
    account_id: name.length,
    account_name: name,
    currency: 'USD',
    effective_month: '2026-01',
    target,
    saved,
    remaining: target - saved,
    ytd_target: target * 3,
    ytd_saved: saved * 3,
    ytd_remaining: (target - saved) * 3,
    months: [],
    excluded_accounts: [],
    ...extra,
  };
}

let report: SavingsReport;
let reportUrls: string[];

beforeEach(() => {
  localStorage.clear();
  reportUrls = [];
  report = {
    month: '2020-03',
    rows: [
      row('Assets:Savings', 1500000, 1200000, {
        excluded_accounts: ['Assets:Savings:Japan'],
        months: [
          { month: '2020-01', target: 1500000, saved: 1600000 },
          { month: '2020-02', target: 1500000, saved: 1000000 },
          { month: '2020-03', target: 1500000, saved: 1200000 },
        ],
      }),
      row('Assets:Savings:Travel', 200000, 250000),
      row('Assets:Emergency', 100000, -30000),
    ],
    total_target: { USD: 1600000 },
    total_saved: { USD: 1170000 },
    total_ytd_target: { USD: 4800000 },
    total_ytd_saved: { USD: 3510000 },
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
      if (url.startsWith('/api/reports/savings')) {
        reportUrls.push(url);
        return Promise.resolve(ok(report));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('renders statuses, remaining wording, ytd and month breakdown', async () => {
  render(makeTestApp('/reports/savings?month=2020-03'));
  await waitFor(() => expect(screen.getAllByTestId('savings-bar').length).toBe(3));
  expect(reportUrls).toContain('/api/reports/savings?month=2020-03');

  // Sorted as the server returns them; a past month counts as complete.
  const bars = screen.getAllByTestId('savings-bar');
  expect(bars.map((b) => b.getAttribute('data-status'))).toEqual([
    'behind',
    'achieved',
    'negative',
  ]);

  // "Savings" also appears in the report tabs and the sidebar, so address rows by position.
  const [savings, travel, emergency] = screen.getAllByTestId('savings-row');
  expect(within(savings).getByText('Savings')).toBeInTheDocument();
  expect(within(savings).getByTestId('savings-remaining')).toHaveTextContent(/3,000\.00 to go/);
  expect(within(savings).getByTestId('savings-ytd')).toHaveTextContent(/9,000\.00 to go/);
  const months = within(savings).getAllByTestId('savings-month');
  expect(months.map((m) => m.getAttribute('data-met'))).toEqual(['true', 'false', 'false']);

  expect(within(travel).getByText('Savings:Travel')).toBeInTheDocument();
  expect(travel.getAttribute('data-depth')).toBe('1');
  expect(within(travel).getByTestId('savings-remaining')).toHaveTextContent(/500\.00 over target/);

  expect(within(emergency).getByTestId('savings-remaining')).toHaveTextContent(/1,300\.00 to go/);

  expect(screen.getByText('Total (USD)')).toBeInTheDocument();
});

test('excluded accounts produce a warning with their names', async () => {
  render(makeTestApp('/reports/savings?month=2020-03'));
  const warn = await screen.findByTestId('excluded-warning');
  expect(warn.getAttribute('title')).toContain('Assets:Savings:Japan');
});

test('current month omits the month param and shows the time marker', async () => {
  report = { ...report, month: currentMonth() };
  render(makeTestApp('/reports/savings'));
  await waitFor(() => expect(screen.getAllByTestId('savings-bar').length).toBe(3));
  expect(reportUrls[0]).toBe('/api/reports/savings');
  expect(screen.getAllByTestId('time-marker').length).toBe(3);
});

test('empty month links to the savings page', async () => {
  report = {
    month: currentMonth(),
    rows: [],
    total_target: {},
    total_saved: {},
    total_ytd_target: {},
    total_ytd_saved: {},
  };
  render(makeTestApp('/reports/savings'));
  const link = await screen.findByRole('link', { name: /set up savings targets/i });
  expect(link.getAttribute('href')).toBe('/savings');
});

test('previous-month button navigates with month param', async () => {
  report = { ...report, month: currentMonth() };
  render(makeTestApp('/reports/savings'));
  await waitFor(() => expect(screen.getAllByTestId('savings-bar').length).toBe(3));
  await userEvent.click(screen.getByRole('button', { name: /previous month/i }));
  await waitFor(() => expect(reportUrls.some((u) => u.includes('month='))).toBe(true));
});

test('report tabs include Savings', async () => {
  render(makeTestApp('/reports/savings?month=2020-03'));
  const tabs = await screen.findByRole('navigation', { name: 'Report types' });
  expect(within(tabs).getByRole('link', { name: 'Savings' }).getAttribute('href')).toBe(
    '/reports/savings',
  );
});
