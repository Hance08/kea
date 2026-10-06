import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRouter,
} from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { SavingsProgress } from '../components/dashboard/widgets/SavingsProgress';
import { currentMonth } from '../lib/budgets';
import { DEFAULT_STATE } from '../lib/dashboard/defaults';
import { ALL_WIDGET_IDS } from '../lib/dashboard/registry';
import { withServerConfig } from './test-app';

const ok = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const row = (name: string, target: number, saved: number, ytdRemaining: number) => ({
  account_id: name.length,
  account_name: name,
  currency: 'USD',
  effective_month: '2026-01',
  target,
  saved,
  remaining: target - saved,
  ytd_target: 0,
  ytd_saved: 0,
  ytd_remaining: ytdRemaining,
  months: [],
  excluded_accounts: [],
});

let urls: string[];
let rows: ReturnType<typeof row>[];

beforeEach(() => {
  urls = [];
  rows = [
    row('Assets:Savings', 1500000, 1500000, -650000),
    row('Assets:Trip', 20000, -5000, 25000),
  ];
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      urls.push(url);
      return Promise.resolve(
        ok({
          month: currentMonth(),
          rows,
          total_target: {},
          total_saved: {},
          total_ytd_target: {},
          total_ytd_saved: {},
        }),
      );
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

function renderWidget(node: ReactNode) {
  const rootRoute = createRootRoute({ component: () => <>{node}</> });
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return render(
    <QueryClientProvider client={qc}>
      {withServerConfig(<RouterProvider router={router} />)}
    </QueryClientProvider>,
  );
}

test('lets the server choose the month (no UTC month on the client)', async () => {
  renderWidget(<SavingsProgress />);
  await waitFor(() => expect(urls).toContain('/api/reports/savings'));
});

test('shows one line per target with status and ytd remaining', async () => {
  renderWidget(<SavingsProgress />);
  const items = await screen.findAllByTestId('savings-progress-item');
  expect(items.map((i) => i.getAttribute('data-account'))).toEqual([
    'Assets:Savings',
    'Assets:Trip',
  ]);
  expect(within(items[0]).getByTestId('savings-bar').getAttribute('data-status')).toBe('achieved');
  expect(within(items[1]).getByTestId('savings-bar').getAttribute('data-status')).toBe('negative');
  expect(within(items[0]).getByText(/over target/)).toBeInTheDocument();
  expect(within(items[1]).getByText(/to go/)).toBeInTheDocument();
});

test('links to the savings report', async () => {
  renderWidget(<SavingsProgress />);
  await screen.findAllByTestId('savings-progress-item');
  expect(screen.getByRole('link').getAttribute('href')).toBe('/reports/savings');
});

test('empty state links to /savings', async () => {
  rows = [];
  renderWidget(<SavingsProgress />);
  const link = await screen.findByRole('link', { name: /set up savings targets/i });
  expect(link.getAttribute('href')).toBe('/savings');
});

test('widget is registered and in the default layout', () => {
  expect(ALL_WIDGET_IDS).toContain('savings-progress');
  expect(DEFAULT_STATE.layout.some((g) => g.i === 'savings-progress')).toBe(true);
});
