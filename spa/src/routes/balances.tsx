import { NetWorthCard } from '@/components/NetWorthCard';
import { BALANCE_COLUMN_PAGE_SIZE, BalanceColumn } from '@/components/balances/BalanceColumn';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { naturalAmount } from '@/lib/accounts';
import { getBalances } from '@/lib/api';
import { summarizeBalances } from '@/lib/balances';
import { type BalancesSearch, parseBalancesSearch } from '@/lib/balances-search-params';
import { useServerConfig } from '@/lib/server-config';
import type { AccountBalance } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useMemo } from 'react';

export const Route = createFileRoute('/balances')({
  validateSearch: (s): BalancesSearch => parseBalancesSearch(s),
  component: BalancesPage,
});

function sortByNatural(rows: AccountBalance[], dir: 'asc' | 'desc'): AccountBalance[] {
  const out = [...rows];
  out.sort((a, b) => {
    const an = naturalAmount(a.type, a.amount);
    const bn = naturalAmount(b.type, b.amount);
    return dir === 'asc' ? an - bn : bn - an;
  });
  return out;
}

function BalancesPage() {
  const { defaults } = useServerConfig();
  const DEFAULT_CURRENCY = defaults.currency;
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/balances' });
  const query = useQuery({ queryKey: ['balances'], queryFn: getBalances });

  const toggleSort = (side: 'a' | 'l') => {
    const key = side === 'a' ? 'a_sort' : 'l_sort';
    const offsetKey = side === 'a' ? 'a_offset' : 'l_offset';
    navigate({
      search: (prev) => ({
        ...prev,
        [key]: prev[key] === 'balance_desc' ? 'balance_asc' : 'balance_desc',
        [offsetKey]: 0,
      }),
    });
  };

  const setOffset = (side: 'a' | 'l', offset: number) => {
    const key = side === 'a' ? 'a_offset' : 'l_offset';
    navigate({ search: (prev) => ({ ...prev, [key]: offset }) });
  };

  const summary = useMemo(
    () => (query.data ? summarizeBalances(query.data.items, DEFAULT_CURRENCY) : null),
    [query.data, DEFAULT_CURRENCY],
  );

  const assetsAll = useMemo(
    () => (summary ? summary.included.filter((r) => r.type === 'A') : []),
    [summary],
  );
  const liabilitiesAll = useMemo(
    () => (summary ? summary.included.filter((r) => r.type === 'L') : []),
    [summary],
  );

  const assetsSortDir: 'asc' | 'desc' = search.a_sort === 'balance_asc' ? 'asc' : 'desc';
  const liabilitiesSortDir: 'asc' | 'desc' = search.l_sort === 'balance_asc' ? 'asc' : 'desc';

  const assetsSorted = useMemo(
    () => sortByNatural(assetsAll, assetsSortDir),
    [assetsAll, assetsSortDir],
  );
  const liabilitiesSorted = useMemo(
    () => sortByNatural(liabilitiesAll, liabilitiesSortDir),
    [liabilitiesAll, liabilitiesSortDir],
  );

  const assetsPaged = useMemo(
    () => assetsSorted.slice(search.a_offset, search.a_offset + BALANCE_COLUMN_PAGE_SIZE),
    [assetsSorted, search.a_offset],
  );
  const liabilitiesPaged = useMemo(
    () => liabilitiesSorted.slice(search.l_offset, search.l_offset + BALANCE_COLUMN_PAGE_SIZE),
    [liabilitiesSorted, search.l_offset],
  );

  if (query.isPending) {
    return (
      <div>
        <Skeleton className="mb-6 h-32 w-full" />
        <div className="grid grid-cols-2 items-start gap-4">
          <Skeleton className="h-72 w-full" />
          <Skeleton className="h-72 w-full" />
        </div>
      </div>
    );
  }

  if (query.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load balances</AlertTitle>
        <AlertDescription className="mt-2 space-y-3">
          <div>{query.error instanceof Error ? query.error.message : 'Unknown error'}</div>
          <Button onClick={() => query.refetch()} size="sm">
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  if (!summary || query.data.items.length === 0) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <p className="max-w-md text-center text-sm text-muted-foreground">
          No accounts yet — run <code className="font-mono">kea ledger add</code> then create one
          via the CLI.
        </p>
      </div>
    );
  }

  return (
    <div>
      <NetWorthCard
        netWorth={summary.netWorth}
        assetsTotal={summary.assetsTotal}
        liabilitiesTotal={summary.liabilitiesTotal}
        currency={DEFAULT_CURRENCY}
        excludedCount={summary.excludedByCurrency.length}
      />

      <div className="grid grid-cols-2 items-start gap-4">
        <BalanceColumn
          label="Assets"
          total={summary.assetsTotal}
          rows={assetsPaged}
          totalRowCount={assetsSorted.length}
          sortDir={assetsSortDir}
          onToggleSort={() => toggleSort('a')}
          offset={search.a_offset}
          onOffsetChange={(off) => setOffset('a', off)}
          emptyText="No assets"
        />
        <BalanceColumn
          label="Liabilities"
          total={summary.liabilitiesTotal}
          rows={liabilitiesPaged}
          totalRowCount={liabilitiesSorted.length}
          sortDir={liabilitiesSortDir}
          onToggleSort={() => toggleSort('l')}
          offset={search.l_offset}
          onOffsetChange={(off) => setOffset('l', off)}
          emptyText="No liabilities"
        />
      </div>
    </div>
  );
}
