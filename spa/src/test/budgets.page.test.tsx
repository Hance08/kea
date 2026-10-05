import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import type { Budget } from '../lib/types';
import { makeTestApp } from './test-app';

const ok = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

let items: Budget[];
let calls: { url: string; method: string; body?: unknown }[];
let putResponse: Response | null;

beforeEach(() => {
  calls = [];
  putResponse = null;
  items = [
    {
      id: 1,
      account_id: 1,
      account_name: 'Expenses:Food',
      effective_month: '2020-01',
      amount: 800000,
      stopped: false,
    },
    {
      id: 2,
      account_id: 1,
      account_name: 'Expenses:Food',
      effective_month: '2020-06',
      amount: 900000,
      stopped: false,
    },
    {
      id: 3,
      account_id: 2,
      account_name: 'Expenses:Gym',
      effective_month: '2020-01',
      amount: 5000,
      stopped: false,
    },
    {
      id: 4,
      account_id: 2,
      account_name: 'Expenses:Gym',
      effective_month: '2020-03',
      amount: 0,
      stopped: true,
    },
  ];
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      const method = init?.method ?? 'GET';
      calls.push({ url, method, body: init?.body ? JSON.parse(String(init.body)) : undefined });
      if (url === '/api/config')
        return Promise.resolve(
          ok({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      if (url === '/api/ledgers')
        return Promise.resolve(
          ok({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      if (url === '/api/budgets' && method === 'GET') return Promise.resolve(ok({ items }));
      if (url === '/api/budgets' && method === 'PUT')
        return Promise.resolve(putResponse ?? ok({ ...items[1], id: 9 }));
      if (url === '/api/budgets/stop') return Promise.resolve(ok({ ...items[1], stopped: true }));
      if (url.startsWith('/api/budgets/') && method === 'DELETE')
        return Promise.resolve(ok({ deleted: true, id: 1 }));
      throw new Error(`unexpected fetch: ${method} ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('lists budgets effective in the selected month', async () => {
  render(makeTestApp('/budgets?month=2020-07'));
  const rows = await screen.findAllByTestId('budget-setting-row');
  expect(rows).toHaveLength(1); // Gym stopped in 2020-03
  expect(within(rows[0]).getByText('Expenses:Food')).toBeInTheDocument();
  expect(within(rows[0]).getByText('2020-06')).toBeInTheDocument();
});

test('edit creates a new version from the selected month', async () => {
  render(makeTestApp('/budgets?month=2020-07'));
  const row = (await screen.findAllByTestId('budget-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /edit/i }));
  const amount = screen.getByLabelText(/amount/i);
  await userEvent.clear(amount);
  await userEvent.type(amount, '9500');
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true));
  const put = calls.find((c) => c.method === 'PUT');
  expect(put?.body).toEqual({
    account_name: 'Expenses:Food',
    effective_month: '2020-07',
    amount: 950000,
  });
});

test('field errors from the API are shown next to the field', async () => {
  putResponse = ok(
    { error: 'validation_failed', message: 'budget amount must not be negative', field: 'amount' },
    400,
  );
  render(makeTestApp('/budgets?month=2020-07'));
  const row = (await screen.findAllByTestId('budget-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /edit/i }));
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  expect(await screen.findByText('budget amount must not be negative')).toBeInTheDocument();
});

test('stop posts the selected month', async () => {
  render(makeTestApp('/budgets?month=2020-07'));
  const row = (await screen.findAllByTestId('budget-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /stop/i }));
  await userEvent.click(screen.getByRole('button', { name: /confirm stop/i }));
  await waitFor(() => expect(calls.some((c) => c.url === '/api/budgets/stop')).toBe(true));
  expect(calls.find((c) => c.url === '/api/budgets/stop')?.body).toEqual({
    account_name: 'Expenses:Food',
    effective_month: '2020-07',
  });
});

test('history shows all versions and deletes one after confirmation', async () => {
  render(makeTestApp('/budgets?month=2020-07'));
  const row = (await screen.findAllByTestId('budget-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /history/i }));
  const versions = screen.getAllByTestId('budget-version');
  expect(versions).toHaveLength(2);
  await userEvent.click(within(versions[0]).getByRole('button', { name: /delete/i }));
  await userEvent.click(screen.getByRole('button', { name: /confirm delete/i }));
  await waitFor(() =>
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/budgets/1')).toBe(true),
  );
});
