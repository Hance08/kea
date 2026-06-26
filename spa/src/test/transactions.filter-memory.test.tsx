import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const EMPTY_LIST = { items: [], total_count: 0, limit: 10, offset: 0 };

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
      if (url.startsWith('/api/transactions?')) {
        return Promise.resolve(okResponse(EMPTY_LIST));
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

test('with no memory, /transactions renders with defaults (no redirect)', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/transactions'));
  await screen.findByText(/no transactions match these filters/i);
  expect(localStorage.getItem('kea.filters.personal.transactions')).toBeNull();
});

test('with memory, /transactions redirects to remembered search on entry', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.transactions',
    JSON.stringify({ limit: 10, offset: 0, type: 'Expense' }),
  );
  render(makeTestApp('/transactions'));
  await waitFor(() => {
    const calls = (fetch as unknown as { mock: { calls: [string][] } }).mock.calls;
    const txCall = calls.find(([u]) => u.startsWith('/api/transactions?'));
    expect(txCall?.[0]).toContain('type=Expense');
  });
});

test('non-default URL saves the search to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/transactions?type=Income'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.transactions');
    expect(raw).not.toBeNull();
    const stored = JSON.parse(raw as string);
    expect(stored.type).toBe('Income');
  });
});

test('clicking Clear wipes memory and stays at defaults', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/transactions?type=Income'));
  await waitFor(() => {
    expect(localStorage.getItem('kea.filters.personal.transactions')).not.toBeNull();
  });
  const clearBtn = await screen.findByRole('button', { name: /clear/i });
  await userEvent.click(clearBtn);
  await waitFor(() => {
    expect(localStorage.getItem('kea.filters.personal.transactions')).toBeNull();
  });
});

test('memory absent when no ledger mirrored (race on first load)', async () => {
  // no kea.activeLedger seeded
  render(makeTestApp('/transactions?type=Income'));
  await screen.findByText(/no transactions match these filters/i);
  expect(localStorage.getItem('kea.filters.personal.transactions')).toBeNull();
});
