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
    JSON.stringify({ as_of: 1700000000 }),
  );
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    const calls = (fetch as unknown as { mock: { calls: [string][] } }).mock.calls;
    expect(calls.some(([u]) => u.includes('as_of=1700000000'))).toBe(true);
  });
});
