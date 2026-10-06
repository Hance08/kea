import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import type { SavingsTarget } from '../lib/types';
import { makeTestApp } from './test-app';

const ok = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

let items: SavingsTarget[];
let calls: { url: string; method: string; body?: unknown }[];
let putResponse: Response | null;

beforeEach(() => {
  calls = [];
  putResponse = null;
  items = [
    {
      id: 1,
      account_id: 1,
      account_name: 'Assets:Savings',
      effective_month: '2020-01',
      amount: 1500000,
      stopped: false,
    },
    {
      id: 2,
      account_id: 1,
      account_name: 'Assets:Savings',
      effective_month: '2020-06',
      amount: 2000000,
      stopped: false,
    },
    {
      id: 3,
      account_id: 2,
      account_name: 'Assets:Trip',
      effective_month: '2020-01',
      amount: 5000,
      stopped: false,
    },
    {
      id: 4,
      account_id: 2,
      account_name: 'Assets:Trip',
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
      if (url === '/api/accounts?include_hidden=true')
        return Promise.resolve(
          ok({
            items: [
              {
                id: 1,
                name: 'Assets:Savings',
                type: 'A',
                currency: 'TWD',
                description: '',
                is_hidden: true,
              },
              {
                id: 2,
                name: 'Assets:Trip',
                type: 'A',
                currency: '',
                description: '',
                is_hidden: false,
              },
            ],
            total: 2,
          }),
        );
      if (url.startsWith('/api/accounts?q='))
        return Promise.resolve(
          ok({
            items: [
              {
                id: 5,
                name: 'Assets:Emergency',
                type: 'A',
                currency: 'USD',
                description: '',
                is_hidden: false,
              },
              {
                id: 6,
                name: 'Expenses:Food',
                type: 'E',
                currency: 'USD',
                description: '',
                is_hidden: false,
              },
            ],
            total: 2,
          }),
        );
      if (url === '/api/savings-targets' && method === 'GET') return Promise.resolve(ok({ items }));
      if (url === '/api/savings-targets' && method === 'PUT')
        return Promise.resolve(putResponse ?? ok({ ...items[1], id: 9 }));
      if (url === '/api/savings-targets/stop')
        return Promise.resolve(ok({ ...items[1], stopped: true }));
      if (url.startsWith('/api/savings-targets/') && method === 'DELETE')
        return Promise.resolve(ok({ deleted: true, id: 1 }));
      throw new Error(`unexpected fetch: ${method} ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('lists targets effective in the selected month', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  const rows = await screen.findAllByTestId('savings-setting-row');
  expect(rows).toHaveLength(1); // Trip stopped in 2020-03
  expect(within(rows[0]).getByText('Assets:Savings')).toBeInTheDocument();
  expect(within(rows[0]).getByText('2020-06')).toBeInTheDocument();
  expect(within(rows[0]).getByTestId('savings-currency')).toHaveTextContent('TWD');
});

test('empty currency falls back to the default', async () => {
  render(makeTestApp('/savings?month=2020-02'));
  const rows = await screen.findAllByTestId('savings-setting-row');
  expect(within(rows[1]).getByTestId('savings-currency')).toHaveTextContent('USD');
});

test('edit creates a new version from the selected month', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  const row = (await screen.findAllByTestId('savings-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /edit/i }));
  const amount = screen.getByLabelText(/amount/i);
  await userEvent.clear(amount);
  await userEvent.type(amount, '25000');
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true));
  expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({
    account_name: 'Assets:Savings',
    effective_month: '2020-07',
    amount: 2500000,
  });
});

test('field errors from the API are shown next to the field', async () => {
  putResponse = ok(
    { error: 'validation_failed', message: 'savings target must not be negative', field: 'amount' },
    400,
  );
  render(makeTestApp('/savings?month=2020-07'));
  const row = (await screen.findAllByTestId('savings-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /edit/i }));
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  expect(await screen.findByText('savings target must not be negative')).toBeInTheDocument();
});

test('stop posts the selected month', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  const row = (await screen.findAllByTestId('savings-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /stop/i }));
  await userEvent.click(screen.getByRole('button', { name: /confirm stop/i }));
  await waitFor(() => expect(calls.some((c) => c.url === '/api/savings-targets/stop')).toBe(true));
  expect(calls.find((c) => c.url === '/api/savings-targets/stop')?.body).toEqual({
    account_name: 'Assets:Savings',
    effective_month: '2020-07',
  });
});

test('history shows all versions and deletes one after confirmation', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  const row = (await screen.findAllByTestId('savings-setting-row'))[0];
  await userEvent.click(within(row).getByRole('button', { name: /history/i }));
  const versions = screen.getAllByTestId('savings-version');
  expect(versions).toHaveLength(2);
  await userEvent.click(within(versions[0]).getByRole('button', { name: /delete/i }));
  await userEvent.click(screen.getByRole('button', { name: /confirm delete/i }));
  await waitFor(() =>
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/savings-targets/1')).toBe(
      true,
    ),
  );
});

test('add only offers Asset accounts and creates a target from the selected month', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  await screen.findAllByTestId('savings-setting-row');
  await userEvent.click(screen.getByRole('button', { name: /add target/i }));
  await userEvent.type(screen.getByPlaceholderText(/asset account/i), 'Emer');
  await userEvent.click(await screen.findByText('Assets:Emergency'));
  expect(screen.queryByText('Expenses:Food')).not.toBeInTheDocument();
  await userEvent.type(screen.getByLabelText(/amount/i), '0');
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true));
  expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({
    account_name: 'Assets:Emergency',
    effective_month: '2020-07',
    amount: 0,
  });
});

test('sidebar links to the savings page', async () => {
  render(makeTestApp('/savings?month=2020-07'));
  const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
  expect(within(nav).getByRole('link', { name: 'Savings' }).getAttribute('href')).toBe('/savings');
});
