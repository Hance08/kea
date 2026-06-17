import { BalanceCard } from '@/components/balances/BalanceCard';
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
import { describe, expect, it } from 'vitest';
import { withServerConfig } from '../test-app';

function renderWithRouter(ui: React.ReactNode) {
  const rootRoute = new RootRoute({ component: () => <>{ui}</> });
  const dummy = new Route({ getParentRoute: () => rootRoute, path: '/', component: () => null });
  const router = new Router({
    routeTree: rootRoute.addChildren([dummy]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return render(
    withServerConfig(
      <QueryClientProvider client={qc}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    ),
  );
}

const assetRow: AccountBalance = {
  account_id: 1,
  name: 'Assets:Investments:00878',
  type: 'A',
  currency: 'USD',
  amount: 1629_79_00,
  is_hidden: false,
};

const liabilityRow: AccountBalance = {
  account_id: 2,
  name: 'Liabilities:CreditCard:Visa',
  type: 'L',
  currency: 'USD',
  amount: -2140_00,
  is_hidden: false,
};

describe('BalanceCard', () => {
  it('strips the Assets: prefix in the Assets column', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={75} />);
    expect(await screen.findByText('Investments:00878')).toBeInTheDocument();
    expect(screen.queryByText(/^Assets:/)).not.toBeInTheDocument();
  });

  it('strips the Liabilities: prefix in the Liabilities column', async () => {
    renderWithRouter(<BalanceCard row={liabilityRow} columnLabel="Liabilities" share={56} />);
    expect(await screen.findByText('CreditCard:Visa')).toBeInTheDocument();
    expect(screen.queryByText(/^Liabilities:/)).not.toBeInTheDocument();
  });

  it('leaves non-canonical names unchanged', async () => {
    const oddRow = { ...assetRow, name: 'Bank:Checking' };
    renderWithRouter(<BalanceCard row={oddRow} columnLabel="Assets" share={10} />);
    expect(await screen.findByText('Bank:Checking')).toBeInTheDocument();
  });

  it('renders the currency badge and the balance amount without sign', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={75} />);
    expect(await screen.findByText('USD')).toBeInTheDocument();
    expect(screen.getByText('$162,979.00')).toBeInTheDocument();
  });

  it('renders the share line when share is a number', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={75} />);
    expect(await screen.findByText('75% of assets')).toBeInTheDocument();
  });

  it('renders the liabilities-side share wording', async () => {
    renderWithRouter(<BalanceCard row={liabilityRow} columnLabel="Liabilities" share={56} />);
    expect(await screen.findByText('56% of liabilities')).toBeInTheDocument();
  });

  it('hides the share line when share is null', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={null} />);
    expect(await screen.findByText('Investments:00878')).toBeInTheDocument();
    expect(screen.queryByText(/% of assets/)).not.toBeInTheDocument();
  });

  it('puts the original un-stripped name in a title tooltip', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={75} />);
    const nameSpan = await screen.findByText('Investments:00878');
    expect(nameSpan).toHaveAttribute('title', 'Assets:Investments:00878');
  });

  it('applies red color to a negative balance', async () => {
    renderWithRouter(<BalanceCard row={liabilityRow} columnLabel="Liabilities" share={56} />);
    expect(await screen.findByText('$2,140.00')).toHaveClass('text-red-600');
  });

  it('applies green color to a positive balance', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={75} />);
    expect(await screen.findByText('$162,979.00')).toHaveClass('text-green-600');
  });
});
