import {
  RootRoute,
  Route,
  Router,
  RouterProvider,
  createMemoryHistory,
} from '@tanstack/react-router';
import { render, screen } from '@testing-library/react';
import { expect, test } from 'vitest';
import { ReportRowTable } from '../components/reports/ReportRowTable';
import type { ReportRow } from '../lib/types';

function harness(node: React.ReactNode) {
  const rootRoute = new RootRoute({ component: () => <>{node}</> });
  const dummy = new Route({ getParentRoute: () => rootRoute, path: '/', component: () => null });
  const router = new Router({
    routeTree: rootRoute.addChildren([dummy]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
  return <RouterProvider router={router} />;
}

const rows: ReportRow[] = [
  {
    account_name: 'Expenses:Food:Groceries',
    offset_account: 'Assets:Bank',
    amount: 52000,
    currency: 'USD',
    tx_count: 3,
  },
  {
    account_name: 'Expenses:Rent',
    offset_account: 'Assets:Bank',
    amount: 180000,
    currency: 'USD',
    tx_count: 1,
  },
];

const nameToId = new Map<string, number>([
  ['Expenses:Food:Groceries', 11],
  ['Expenses:Rent', 12],
]);

test('renders rows with account name, amount, tx count', async () => {
  render(
    harness(
      <ReportRowTable
        rows={rows}
        currency="USD"
        nameToId={nameToId}
        period={{ startUnix: 100, endUnix: 200 }}
      />,
    ),
  );
  // Type prefix is stripped in the rendered cell; the original full name
  // stays on the wrapping span's `title` for hover.
  expect(await screen.findByText('Food:Groceries')).toBeInTheDocument();
  expect(screen.getByText('Rent')).toBeInTheDocument();
  expect(screen.queryByText('Expenses:Food:Groceries')).toBeNull();
  expect(screen.queryByText('Expenses:Rent')).toBeNull();
  expect(screen.getByText('$520.00')).toBeInTheDocument();
  expect(screen.getByText('$1,800.00')).toBeInTheDocument();
});

test('rows link to /transactions with account_id and bounds', async () => {
  render(
    harness(
      <ReportRowTable
        rows={rows}
        currency="USD"
        nameToId={nameToId}
        period={{ startUnix: 100, endUnix: 200 }}
      />,
    ),
  );
  const link = await screen.findByRole('link', { name: /Food:Groceries/ });
  const href = link.getAttribute('href') ?? '';
  expect(href).toContain('/transactions');
  expect(href).toContain('account_id=11');
  expect(href).toContain('start_time=100');
  expect(href).toContain('end_time=200');
});

test('rows without a resolvable id omit account_id', async () => {
  render(
    harness(
      <ReportRowTable
        rows={rows}
        currency="USD"
        nameToId={new Map()}
        period={{ startUnix: 100, endUnix: 200 }}
      />,
    ),
  );
  const link = await screen.findByRole('link', { name: /Food:Groceries/ });
  const href = link.getAttribute('href') ?? '';
  expect(href).not.toContain('account_id=');
  expect(href).toContain('start_time=100');
});

test('with period=null, rows link to /accounts/{id}', async () => {
  render(harness(<ReportRowTable rows={rows} currency="USD" nameToId={nameToId} period={null} />));
  const link = await screen.findByRole('link', { name: /Food:Groceries/ });
  expect(link.getAttribute('href')).toMatch(/\/accounts\/11/);
});

test('renders empty-state message when rows is empty', async () => {
  render(harness(<ReportRowTable rows={[]} currency="USD" nameToId={nameToId} period={null} />));
  expect(await screen.findByText(/no rows/i)).toBeInTheDocument();
});
