import { BalanceColumn } from '@/components/balances/BalanceColumn';
import type { AccountBalance } from '@/lib/types';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  RootRoute,
  Route,
  Router,
  RouterProvider,
  createMemoryHistory,
} from '@tanstack/react-router';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as React from 'react';
import { describe, expect, it, vi } from 'vitest';

// BalanceColumn renders <Link>, so it needs a router context. Build a
// minimal one with a single dummy route so the link has somewhere to go.
// RouterProvider resolves its initial match inside a Suspense boundary, so
// the render must be flushed with act() before assertions can find content.
//
// `ui` is held in an external store read via useSyncExternalStore by the
// route's component, so that `rerenderUi` can swap the rendered content
// without remounting the RouterProvider tree.
function makeUiStore(initial: React.ReactNode) {
  let current = initial;
  const listeners = new Set<() => void>();
  return {
    get: () => current,
    set: (next: React.ReactNode) => {
      current = next;
      for (const listener of listeners) listener();
    },
    subscribe: (listener: () => void) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

function UiSlot({ store }: { store: ReturnType<typeof makeUiStore> }) {
  const ui = React.useSyncExternalStore(store.subscribe, store.get, store.get);
  return <>{ui}</>;
}

async function renderWithRouter(ui: React.ReactNode) {
  const store = makeUiStore(ui);
  const rootRoute = new RootRoute({ component: () => <UiSlot store={store} /> });
  const dummy = new Route({
    getParentRoute: () => rootRoute,
    path: '/',
    component: () => null,
  });
  const router = new Router({
    routeTree: rootRoute.addChildren([dummy]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <QueryClientProvider client={qc}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
  });
  return {
    ...result,
    rerenderUi: async (nextUi: React.ReactNode) => {
      await act(async () => {
        store.set(nextUi);
      });
    },
  };
}

function makeRows(count: number, type: 'A' | 'L'): AccountBalance[] {
  return Array.from({ length: count }, (_, i) => ({
    account_id: i + 1,
    name: `${type === 'A' ? 'Assets' : 'Liab'}:Acct${i + 1}`,
    type,
    currency: 'USD',
    amount: type === 'A' ? (i + 1) * 1000 : -(i + 1) * 1000,
    is_hidden: false,
  }));
}

describe('BalanceColumn', () => {
  const baseProps = {
    label: 'Assets' as const,
    total: 24310_00,
    currency: 'USD',
    sortDir: 'desc' as const,
    onToggleSort: vi.fn(),
    offset: 0,
    onOffsetChange: vi.fn(),
    emptyText: 'No assets',
  };

  it('renders the type label and total in the header bar', async () => {
    await renderWithRouter(
      <BalanceColumn {...baseProps} rows={makeRows(3, 'A')} totalRowCount={3} />,
    );
    expect(screen.getByText('Assets')).toBeInTheDocument();
    expect(screen.getByText('$24,310.00')).toBeInTheDocument();
  });

  it('renders the sort arrow matching sortDir', async () => {
    const { rerenderUi } = await renderWithRouter(
      <BalanceColumn {...baseProps} rows={makeRows(1, 'A')} totalRowCount={1} sortDir="desc" />,
    );
    expect(screen.getByRole('button', { name: /Balance/i })).toHaveTextContent('▼');
    await rerenderUi(
      <BalanceColumn {...baseProps} rows={makeRows(1, 'A')} totalRowCount={1} sortDir="asc" />,
    );
    expect(screen.getByRole('button', { name: /Balance/i })).toHaveTextContent('▲');
  });

  it('calls onToggleSort when the Balance header is clicked', async () => {
    const onToggleSort = vi.fn();
    await renderWithRouter(
      <BalanceColumn
        {...baseProps}
        rows={makeRows(1, 'A')}
        totalRowCount={1}
        onToggleSort={onToggleSort}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /Balance/i }));
    expect(onToggleSort).toHaveBeenCalledTimes(1);
  });

  it('hides pagination when totalRowCount is 8 or fewer', async () => {
    await renderWithRouter(
      <BalanceColumn {...baseProps} rows={makeRows(8, 'A')} totalRowCount={8} />,
    );
    expect(screen.queryByText(/Page \d+ of/)).not.toBeInTheDocument();
  });

  it('shows pagination when totalRowCount exceeds 8', async () => {
    await renderWithRouter(
      <BalanceColumn {...baseProps} rows={makeRows(8, 'A')} totalRowCount={15} />,
    );
    expect(screen.getByText(/Page 1 of 2/)).toBeInTheDocument();
  });

  it('shows the empty text and hides sort/pagination when rows is empty', async () => {
    await renderWithRouter(<BalanceColumn {...baseProps} rows={[]} totalRowCount={0} />);
    expect(screen.getByText('No assets')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Balance/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/Page \d+ of/)).not.toBeInTheDocument();
  });
});
