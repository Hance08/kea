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
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

// BalanceColumn renders <Link>, so it needs a router context. Build a
// minimal one with a single dummy route so the link has somewhere to go.
function renderWithRouter(ui: React.ReactNode) {
  const rootRoute = new RootRoute({ component: () => <>{ui}</> });
  const dummy = new Route({ getParentRoute: () => rootRoute, path: '/', component: () => null });
  const router = new Router({
    routeTree: rootRoute.addChildren([dummy]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return render(
    <QueryClientProvider client={qc}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
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
    renderWithRouter(<BalanceColumn {...baseProps} rows={makeRows(3, 'A')} totalRowCount={3} />);
    expect(await screen.findByText('Assets')).toBeInTheDocument();
    expect(screen.getByText('$24,310.00')).toBeInTheDocument();
  });

  it('renders a descending sort arrow when sortDir is desc', async () => {
    renderWithRouter(
      <BalanceColumn {...baseProps} rows={makeRows(1, 'A')} totalRowCount={1} sortDir="desc" />,
    );
    expect(await screen.findByRole('button', { name: /Balance/i })).toHaveTextContent('▼');
  });

  it('renders an ascending sort arrow when sortDir is asc', async () => {
    renderWithRouter(
      <BalanceColumn {...baseProps} rows={makeRows(1, 'A')} totalRowCount={1} sortDir="asc" />,
    );
    expect(await screen.findByRole('button', { name: /Balance/i })).toHaveTextContent('▲');
  });

  it('calls onToggleSort when the Balance header is clicked', async () => {
    const onToggleSort = vi.fn();
    renderWithRouter(
      <BalanceColumn
        {...baseProps}
        rows={makeRows(1, 'A')}
        totalRowCount={1}
        onToggleSort={onToggleSort}
      />,
    );
    await userEvent.click(await screen.findByRole('button', { name: /Balance/i }));
    expect(onToggleSort).toHaveBeenCalledTimes(1);
  });

  it('hides pagination when totalRowCount is 8 or fewer', async () => {
    renderWithRouter(<BalanceColumn {...baseProps} rows={makeRows(8, 'A')} totalRowCount={8} />);
    expect(await screen.findByText('Assets')).toBeInTheDocument();
    expect(screen.queryByText(/Page \d+ of/)).not.toBeInTheDocument();
  });

  it('shows pagination when totalRowCount exceeds 8', async () => {
    renderWithRouter(<BalanceColumn {...baseProps} rows={makeRows(8, 'A')} totalRowCount={15} />);
    expect(await screen.findByText(/Page 1 of 2/)).toBeInTheDocument();
  });

  it('shows the empty text and hides sort/pagination when rows is empty', async () => {
    renderWithRouter(<BalanceColumn {...baseProps} rows={[]} totalRowCount={0} />);
    expect(await screen.findByText('No assets')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Balance/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/Page \d+ of/)).not.toBeInTheDocument();
  });
});
