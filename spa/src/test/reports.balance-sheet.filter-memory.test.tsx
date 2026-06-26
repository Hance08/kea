import { render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  localStorage.clear();
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
          okResponse({
            active: 'personal',
            items: [{ name: 'personal', path: '/p', active: true }],
          }),
        );
      }
      if (url.startsWith('/api/reports/balance-sheet')) {
        return Promise.resolve(okResponse({ as_of: 0, assets: [], liabilities: [], equity: [] }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

test('non-default URL saves to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/reports/balance-sheet?as_of=1700000000'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.reports/balance-sheet');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).as_of).toBe(1700000000);
  });
});

test('with memory, redirects to remembered as_of', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.reports/balance-sheet',
    JSON.stringify({ as_of: 1700000000, chart_range: '1Y' }),
  );
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    const calls = (fetch as unknown as { mock: { calls: [string][] } }).mock.calls;
    expect(calls.some(([u]) => u.includes('as_of=1700000000'))).toBe(true);
  });
});

test('non-default chart_range URL saves to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/reports/balance-sheet?chart_range=3M'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.reports/balance-sheet');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).chart_range).toBe('3M');
  });
});

test('with remembered non-default chart_range, redirect restores it', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.reports/balance-sheet',
    JSON.stringify({ chart_range: 'YTD' }),
  );
  // Override fetch so the page renders the full body (with ChartRangeSelector)
  // instead of the empty state.
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
          okResponse({
            active: 'personal',
            items: [{ name: 'personal', path: '/p', active: true }],
          }),
        );
      }
      if (url.startsWith('/api/reports/balance-sheet')) {
        return Promise.resolve(
          okResponse({
            as_of: 1700000000,
            assets: [
              {
                account_name: 'Bank',
                offset_account: '',
                amount: 50000,
                currency: 'USD',
                tx_count: 1,
              },
            ],
            liabilities: [],
            equity: [],
            total_assets: { USD: 50000 },
            total_liabilities: { USD: 0 },
            total_equity: { USD: 0 },
            net_worth: { USD: 50000 },
          }),
        );
      }
      if (url.startsWith('/api/reports/net-worth-series')) {
        return Promise.resolve(
          okResponse({
            items: [
              {
                currency: 'USD',
                points: [
                  { date: '2026-06-01', balance: 40000 },
                  { date: '2026-06-15', balance: 45000 },
                  { date: '2026-06-26', balance: 50000 },
                ],
              },
            ],
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    const active = document.querySelector('button[role="radio"][aria-checked="true"]');
    expect(active?.textContent).toBe('YTD');
  });
});
