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

const PREV_REPORT_PAYLOAD = {
  ...REPORT_PAYLOAD,
  period: 'May 2026',
  total_income: { USD: 400000 }, // prev income = $4,000.00; current = $5,248.00 → +$1,248.00 (+31.2%)
  total_expense: { USD: 300000 }, // prev expense = $3,000.00; current = $2,530.00 → -$470.00 (-15.7%)
  net_amount: { USD: 100000 }, // prev net = $1,000.00; current = $2,718.00 → +$1,718.00 (+171.8%)
};

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date('2026-06-17T00:00:00Z'));
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
        if (url.includes('month=2026-05')) {
          return Promise.resolve(okResponse(PREV_REPORT_PAYLOAD));
        }
        return Promise.resolve(okResponse(REPORT_PAYLOAD));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
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
  // Row labels appear in the mix bars; the account-type prefix is
  // stripped in the visible text. `Income:Salary` is a non-canonical
  // fixture name whose prefix isn't in the strip set, so it renders
  // unchanged.
  expect(screen.getByText('Rent')).toBeInTheDocument();
  expect(screen.queryByText('Expenses:Rent')).toBeNull();
  expect(screen.getByText('Income:Salary')).toBeInTheDocument();
  // No detail-table headings.
  expect(screen.queryByRole('heading', { name: /Income detail/i })).toBeNull();
  expect(screen.queryByRole('heading', { name: /Expense detail/i })).toBeNull();
  // No drill-down links to /transactions.
  expect(screen.queryByRole('link', { name: /Rent/ })).toBeNull();

  // Diff lines (current minus previous) appear on each KPI card.
  await waitFor(() => {
    const diffs = document.querySelectorAll('[data-testid="kpi-diff"]');
    expect(diffs.length).toBe(3);
  });
  const diffTexts = Array.from(document.querySelectorAll('[data-testid="kpi-diff"]')).map(
    (el) => el.textContent ?? '',
  );
  // Income: +$1,248.00 vs prev $4,000.00
  expect(diffTexts.some((t) => t.includes('+$1,248.00'))).toBe(true);
  // Expense: -$470.00 vs prev $3,000.00
  expect(diffTexts.some((t) => t.includes('-$470.00'))).toBe(true);
  // Net: +$1,718.00 vs prev $1,000.00
  expect(diffTexts.some((t) => t.includes('+$1,718.00'))).toBe(true);
});

test('renders empty state when there are no rows', async () => {
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

test('renders Regular/Irregular subLine on Income and Expense KPIs', async () => {
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
        return Promise.resolve(okResponse({ items: [], total_count: 0, limit: 0, offset: 0 }));
      }
      if (url.startsWith('/api/reports/income-statement')) {
        return Promise.resolve(
          okResponse({
            ...REPORT_PAYLOAD,
            total_income: { USD: 530000 },
            total_income_regular: { USD: 500000 },
            total_income_irregular: { USD: 30000 },
            total_expense: { USD: 228000 },
            total_expense_regular: { USD: 180000 },
            total_expense_irregular: { USD: 48000 },
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
  render(makeTestApp('/reports/income-statement'));
  await waitFor(() => {
    expect(screen.getByText('Income')).toBeInTheDocument();
  });
  const incomeLine = await screen.findByText(/Regular .*\$5,000\.00.*Irregular .*\$300\.00/);
  const expenseLine = screen.getByText(/Regular .*\$1,800\.00.*Irregular .*\$480\.00/);
  expect(incomeLine).toBeInTheDocument();
  expect(expenseLine).toBeInTheDocument();
});

test('omits diff lines when previous-period query is errored', async () => {
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
        return Promise.resolve(okResponse({ items: [], total_count: 0, limit: 0, offset: 0 }));
      }
      if (url.startsWith('/api/reports/income-statement')) {
        if (url.includes('month=2026-05')) {
          return Promise.resolve(new Response('boom', { status: 500 }));
        }
        return Promise.resolve(okResponse(REPORT_PAYLOAD));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
  render(makeTestApp('/reports/income-statement'));
  await waitFor(() => {
    expect(screen.getByText('Income')).toBeInTheDocument();
  });
  // Give the previous-period query time to settle.
  await waitFor(() => {
    expect(document.querySelectorAll('[data-testid="kpi-diff"]').length).toBe(0);
  });
});
