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
          okResponse({ active: 'personal', items: [{ name: 'personal', path: '/p', active: true }] }),
        );
      }
      if (url === '/api/balances/history') {
        return Promise.resolve(okResponse({ items: [] }));
      }
      if (url.startsWith('/api/balances')) {
        return Promise.resolve(
          okResponse({
            items: [
              {
                account_id: 1,
                name: 'Assets:Bank',
                type: 'A',
                currency: 'USD',
                amount: 100000,
                is_hidden: false,
              },
            ],
            total_count: 1,
            limit: 0,
            offset: 0,
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

test('with memory, /balances redirects to remembered view', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.balances',
    JSON.stringify({
      a_offset: 0,
      a_sort: 'balance_desc',
      l_offset: 0,
      l_sort: 'balance_desc',
      view: 'list',
    }),
  );
  const { container } = render(makeTestApp('/balances'));
  await waitFor(() => {
    const listBtn = container.querySelector('button[aria-label="List view"]');
    expect(listBtn?.getAttribute('aria-pressed')).toBe('true');
  });
});

test('non-default URL saves to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/balances?view=list'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.balances');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).view).toBe('list');
  });
});
