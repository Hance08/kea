import { render, screen } from '@testing-library/react';
import { afterEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const RECONCILED_TX = {
  id: 99,
  timestamp: 1733184000,
  description: 'Old groceries',
  status: 'Reconciled',
  type: 'Expense',
  splits: [
    {
      id: 100,
      account_id: 6,
      account_name: 'Assets:Cash',
      account_type: 'A',
      amount: -8320,
      currency: 'USD',
      memo: '',
    },
    {
      id: 101,
      account_id: 2,
      account_name: 'Expenses:Groceries',
      account_type: 'E',
      amount: 8320,
      currency: 'USD',
      memo: '',
    },
  ],
};

const CLEARED_TX = { ...RECONCILED_TX, id: 100, status: 'Cleared' };

function setupFetch(tx: typeof RECONCILED_TX) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p', active: true }] }),
        );
      }
      if (url === `/api/transactions/${tx.id}`) {
        return Promise.resolve(okResponse(tx));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

test('reconciled detail shows ReconciledBanner and hides Edit/Delete', async () => {
  setupFetch(RECONCILED_TX);
  render(makeTestApp(`/transactions/${RECONCILED_TX.id}`));

  expect(await screen.findByText(/This transaction is reconciled/i)).toBeInTheDocument();
  expect(screen.queryByRole('link', { name: /edit/i })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
});

test('non-reconciled detail shows Edit and Delete', async () => {
  setupFetch(CLEARED_TX);
  render(makeTestApp(`/transactions/${CLEARED_TX.id}`));

  expect(await screen.findByText('Old groceries')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: /edit/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
  expect(screen.queryByText(/This transaction is reconciled/i)).not.toBeInTheDocument();
});
