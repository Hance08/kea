import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
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
      if (url.startsWith('/api/balances')) {
        return Promise.resolve(okResponse({ items: [] }));
      }
      if (url.startsWith('/api/accounts')) {
        return Promise.resolve(okResponse({ items: [], total_count: 0 }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

test('with memory, /accounts redirects to remembered search', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.accounts',
    JSON.stringify({
      q: 'bank',
      include_hidden: false,
      show_parents: false,
      limit: 10,
      offset: 0,
    }),
  );
  render(makeTestApp('/accounts'));
  await waitFor(() => {
    const calls = (fetch as unknown as { mock: { calls: [string][] } }).mock.calls;
    const accountsCall = calls.find(([u]) => u.includes('q=bank'));
    expect(accountsCall).toBeDefined();
  });
});

test('non-default URL saves to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/accounts?q=cash'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.accounts');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).q).toBe('cash');
  });
});

test('clicking Clear wipes memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/accounts?q=cash'));
  await waitFor(() => {
    expect(localStorage.getItem('kea.filters.personal.accounts')).not.toBeNull();
  });
  const clearBtn = await screen.findByRole('button', { name: /clear/i });
  await userEvent.click(clearBtn);
  await waitFor(() => {
    expect(localStorage.getItem('kea.filters.personal.accounts')).toBeNull();
  });
});
