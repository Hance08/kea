import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const ACCOUNTS = {
  items: [
    {
      id: 1,
      name: 'Assets:Bank',
      type: 'A',
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
    {
      id: 2,
      name: 'Expenses:Coffee',
      type: 'E',
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
  ],
  total_count: 2,
  limit: 0,
  offset: 0,
};

let postedBody: unknown = null;

beforeEach(() => {
  postedBody = null;
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/config') {
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p', active: true }] }),
        );
      }
      if (url.startsWith('/api/accounts')) {
        return Promise.resolve(okResponse(ACCOUNTS));
      }
      if (url === '/api/transactions' && init?.method === 'POST') {
        postedBody = init.body ? JSON.parse(init.body as string) : null;
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: 42,
              timestamp: Math.floor(Date.now() / 1000),
              description: 'Coffee',
              status: 'Cleared',
              type: 'Expense',
              splits: [],
            }),
            { status: 201, headers: { 'Content-Type': 'application/json' } },
          ),
        );
      }
      if (url === '/api/transactions/42') {
        return Promise.resolve(
          okResponse({
            id: 42,
            timestamp: Math.floor(Date.now() / 1000),
            description: 'Coffee',
            status: 'Cleared',
            type: 'Expense',
            splits: [],
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url} (method=${init?.method ?? 'GET'})`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('Simple mode submits a 2-split transaction', async () => {
  const user = userEvent.setup();
  render(makeTestApp('/transactions/new'));

  await user.type(await screen.findByLabelText('Description'), 'Coffee');

  await user.type(screen.getByLabelText('From account'), 'Assets:Bank');
  await new Promise((r) => setTimeout(r, 250));
  await user.type(screen.getByLabelText('To account'), 'Expenses:Coffee');
  await new Promise((r) => setTimeout(r, 250));
  await user.type(screen.getByLabelText('Amount'), '12.50');

  await user.click(screen.getByRole('button', { name: /create/i }));

  await waitFor(() => expect(postedBody).not.toBeNull());
  expect(postedBody).toMatchObject({
    description: 'Coffee',
    status: 'Cleared',
    // type must be present — the server rejects creates with empty type.
    // SimpleFields derives it client-side from the picked accounts.
    type: 'Expense',
    splits: [
      { account_name: 'Assets:Bank', amount: -1250, currency: 'USD' },
      { account_name: 'Expenses:Coffee', amount: 1250, currency: 'USD' },
    ],
  });
});

test('Advanced toggle reveals the splits editor with balance indicator', async () => {
  const user = userEvent.setup();
  render(makeTestApp('/transactions/new'));

  await user.click(await screen.findByLabelText(/Advanced \(edit splits\)/));

  expect(await screen.findByText(/Balance:/)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /add split/i })).toBeInTheDocument();
});
