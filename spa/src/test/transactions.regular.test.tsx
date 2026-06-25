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
      name: 'Assets:Savings',
      type: 'A',
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
    {
      id: 3,
      name: 'Expenses:Coffee',
      type: 'E',
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
  ],
  total_count: 3,
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
      throw new Error(`unexpected fetch: ${url} (method=${init?.method ?? 'GET'})`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('shows Regular checkbox when type derives to Expense', async () => {
  const user = userEvent.setup();
  render(makeTestApp('/transactions/new'));

  await user.type(await screen.findByLabelText('From account'), 'Assets:Bank');
  await new Promise((r) => setTimeout(r, 250));
  await user.type(screen.getByLabelText('To account'), 'Expenses:Coffee');
  await new Promise((r) => setTimeout(r, 250));
  await user.type(screen.getByLabelText('Amount'), '12.50');

  expect(await screen.findByLabelText(/Regular/i)).toBeInTheDocument();
});

test('hides Regular checkbox when type derives to Transfer', async () => {
  const user = userEvent.setup();
  render(makeTestApp('/transactions/new'));

  await user.type(await screen.findByLabelText('From account'), 'Assets:Bank');
  await new Promise((r) => setTimeout(r, 250));
  await user.type(screen.getByLabelText('To account'), 'Assets:Savings');
  await new Promise((r) => setTimeout(r, 250));
  await user.type(screen.getByLabelText('Amount'), '12.50');

  // Give the form a beat to (not) render the checkbox.
  await new Promise((r) => setTimeout(r, 100));
  expect(screen.queryByLabelText(/Regular/i)).not.toBeInTheDocument();
});

test('submits regular: true by default for an Expense transaction', async () => {
  const user = userEvent.setup();
  render(makeTestApp('/transactions/new'));

  await user.type(await screen.findByLabelText('Description'), 'Coffee');
  await user.type(await screen.findByLabelText('From account'), 'Assets:Bank');
  await new Promise((r) => setTimeout(r, 250));
  await user.type(screen.getByLabelText('To account'), 'Expenses:Coffee');
  await new Promise((r) => setTimeout(r, 250));
  await user.type(screen.getByLabelText('Amount'), '12.50');

  await screen.findByLabelText(/Regular/i);

  await user.click(screen.getByRole('button', { name: /create/i }));

  await waitFor(() => expect(postedBody).not.toBeNull());
  expect(postedBody).toMatchObject({
    type: 'Expense',
    regular: true,
  });
});
