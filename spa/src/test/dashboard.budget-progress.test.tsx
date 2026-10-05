import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRouter,
} from '@tanstack/react-router';
import { render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import {
  BUDGET_PROGRESS_DEFAULT,
  BudgetProgress,
} from '../components/dashboard/widgets/BudgetProgress';
import { currentMonth } from '../lib/budgets';
import { ALL_WIDGET_IDS } from '../lib/dashboard/registry';
import { withServerConfig } from './test-app';

const ok = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const row = (name: string, budget: number, actual: number) => ({
  account_id: name.length,
  account_name: name,
  currency: 'USD',
  effective_month: '2026-01',
  budget,
  actual,
  actual_regular: 0,
  actual_irregular: actual,
  remaining: budget - actual,
  excluded_accounts: [],
});

let urls: string[];
let rows: ReturnType<typeof row>[];

beforeEach(() => {
  urls = [];
  rows = [
    row('Expenses:A', 1000, 100), // 10%
    row('Expenses:B', 1000, 900), // 90%
    row('Expenses:C', 1000, 1200), // 120%
    row('Expenses:D', 1000, 500), // 50%
    row('Expenses:E', 1000, 0),
    row('Expenses:F', 1000, 850), // 85%
  ];
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      urls.push(url);
      return Promise.resolve(
        ok({ month: currentMonth(), rows, total_budget: {}, total_actual: {} }),
      );
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

// The widget renders <Link>, so it needs a router context.
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
  renderWidget(<BudgetProgress config={BUDGET_PROGRESS_DEFAULT} />);
  await waitFor(() => expect(urls).toContain('/api/reports/budget'));
});

test('sorts by percent used and limits to N', async () => {
  renderWidget(<BudgetProgress config={{ limit: 5, onlyWarnings: false }} />);
  const items = await screen.findAllByTestId('budget-progress-item');
  expect(items).toHaveLength(5);
  expect(items.map((i) => i.getAttribute('data-account'))).toEqual([
    'Expenses:C',
    'Expenses:B',
    'Expenses:F',
    'Expenses:D',
    'Expenses:A',
  ]);
});

test('onlyWarnings keeps rows at or above 80%', async () => {
  renderWidget(<BudgetProgress config={{ limit: 10, onlyWarnings: true }} />);
  const items = await screen.findAllByTestId('budget-progress-item');
  expect(items.map((i) => i.getAttribute('data-account'))).toEqual([
    'Expenses:C',
    'Expenses:B',
    'Expenses:F',
  ]);
});

test('empty state links to /budgets', async () => {
  rows = [];
  renderWidget(<BudgetProgress config={BUDGET_PROGRESS_DEFAULT} />);
  const link = await screen.findByRole('link', { name: /set up budgets/i });
  expect(link.getAttribute('href')).toBe('/budgets');
});

test('widget is registered', () => {
  expect(ALL_WIDGET_IDS).toContain('budget-progress');
});
