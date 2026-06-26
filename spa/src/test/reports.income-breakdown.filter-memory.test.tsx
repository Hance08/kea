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
      if (url.startsWith('/api/reports/income-breakdown')) {
        return Promise.resolve(okResponse({ rows: [], total: 0 }));
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
  render(makeTestApp('/reports/income-breakdown?range=ytd'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.reports/income-breakdown');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).range).toBe('ytd');
  });
});

test('with memory, redirects to remembered range', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.reports/income-breakdown',
    JSON.stringify({ range: 'last-12mo' }),
  );
  render(makeTestApp('/reports/income-breakdown'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.reports/income-breakdown');
    expect(JSON.parse(raw as string).range).toBe('last-12mo');
  });
});
