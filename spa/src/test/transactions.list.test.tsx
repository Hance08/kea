import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const SEEDED_LIST = {
  items: [
    {
      id: 1,
      timestamp: 1733184000,
      description: 'Coffee with team',
      status: 'Cleared',
      type: 'Expense',
      splits: [
        {
          id: 10,
          account_id: 1,
          account_name: 'Assets:Bank',
          account_type: 'A',
          amount: -1250,
          currency: 'USD',
          memo: '',
        },
        {
          id: 11,
          account_id: 2,
          account_name: 'Expenses:Coffee',
          account_type: 'E',
          amount: 1250,
          currency: 'USD',
          memo: '',
        },
      ],
    },
    {
      id: 2,
      timestamp: 1733097600,
      description: 'June salary',
      status: 'Cleared',
      type: 'Income',
      splits: [
        {
          id: 20,
          account_id: 1,
          account_name: 'Assets:Bank',
          account_type: 'A',
          amount: 420000,
          currency: 'USD',
          memo: '',
        },
        {
          id: 21,
          account_id: 3,
          account_name: 'Revenue:Salary',
          account_type: 'R',
          amount: -420000,
          currency: 'USD',
          memo: '',
        },
      ],
    },
  ],
  total_count: 2,
  limit: 50,
  offset: 0,
};

beforeEach(() => {
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
      if (url.startsWith('/api/transactions?')) {
        return Promise.resolve(okResponse(SEEDED_LIST));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('renders the transactions table with seeded rows', async () => {
  render(makeTestApp('/transactions'));

  expect(await screen.findByText('Coffee with team')).toBeInTheDocument();
  expect(await screen.findByText('June salary')).toBeInTheDocument();
});

test('Status column renders as plain text without color or emoji', async () => {
  render(makeTestApp('/transactions'));

  await screen.findByText('Coffee with team');

  const allCleared = await screen.findAllByText('Cleared');
  const clearedSpans = allCleared.filter((el) => el.tagName === 'SPAN');
  expect(clearedSpans.length).toBeGreaterThan(0);
  for (const el of clearedSpans) {
    expect(el).not.toHaveAttribute('role', 'img');
    const cls = el.className;
    expect(cls).not.toMatch(/text-(green|red|amber|blue)-/);
  }
});

test('pagination is hidden when total <= limit', async () => {
  render(makeTestApp('/transactions'));
  await screen.findByText('Coffee with team');
  expect(screen.queryByRole('button', { name: /next/i })).not.toBeInTheDocument();
});
