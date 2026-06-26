import { NetWorthCard } from '@/components/NetWorthCard';
import { BalanceColumn, balanceColumnPageSize } from '@/components/balances/BalanceColumn';
import { ViewToggle } from '@/components/balances/ViewToggle';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { naturalAmount } from '@/lib/accounts';
import { getBalanceHistory, getBalances } from '@/lib/api';
import { summarizeBalances } from '@/lib/balances';
import { type BalancesSearch, parseBalancesSearch } from '@/lib/balances-search-params';
import { makeFilterMemoryLoader } from '@/lib/filter-memory';
import { useServerConfig } from '@/lib/server-config';
import type { AccountBalance, BalanceHistoryPoint } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useMemo } from 'react';

const BALANCES_DEFAULTS = parseBalancesSearch({});

export const Route = createFileRoute('/balances')({
  validateSearch: (s): BalancesSearch => parseBalancesSearch(s),
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<BalancesSearch>({
    pageId: 'balances',
    defaults: BALANCES_DEFAULTS,
    redirectTo: '/balances',
  }),
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

// Pre-slice shares: each row's |amount| / |total| as a whole percent.
// Returns nulls when total is zero (avoids 0/0 and the "0%" line on every card).
function computeShares(rows: AccountBalance[], total: number): (number | null)[] {
  if (total === 0) return rows.map(() => null);
  const denom = Math.abs(total);
  return rows.map((r) => Math.round((Math.abs(r.amount) / denom) * 100));
}

function BalancesPage() {
  const { defaults } = useServerConfig();
  const DEFAULT_CURRENCY = defaults.currency;
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/balances' });
  const query = useQuery({ queryKey: ['balances'], queryFn: getBalances });

  const historyQuery = useQuery({
    queryKey: ['balance-history'],
    queryFn: getBalanceHistory,
  });

  const historyByAccount = useMemo(() => {
    const m = new Map<number, BalanceHistoryPoint[]>();
    if (historyQuery.data) {
      for (const s of historyQuery.data.items) {
        m.set(s.account_id, s.points);
      }
    }
    return m;
  }, [historyQuery.data]);

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

  const setView = (next: 'list' | 'cards') => {
    navigate({ search: (prev) => ({ ...prev, view: next }) });
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

  const assetsSharesAll = useMemo(
    () => computeShares(assetsSorted, summary?.assetsTotal ?? 0),
    [assetsSorted, summary?.assetsTotal],
  );
  const liabilitiesSharesAll = useMemo(
    () => computeShares(liabilitiesSorted, summary?.liabilitiesTotal ?? 0),
    [liabilitiesSorted, summary?.liabilitiesTotal],
  );

  const pageSize = balanceColumnPageSize(search.view);

  const assetsPaged = useMemo(
    () => assetsSorted.slice(search.a_offset, search.a_offset + pageSize),
    [assetsSorted, search.a_offset, pageSize],
  );
  const liabilitiesPaged = useMemo(
    () => liabilitiesSorted.slice(search.l_offset, search.l_offset + pageSize),
    [liabilitiesSorted, search.l_offset, pageSize],
  );

  const assetsPagedShares = useMemo(
    () => assetsSharesAll.slice(search.a_offset, search.a_offset + pageSize),
    [assetsSharesAll, search.a_offset, pageSize],
  );
  const liabilitiesPagedShares = useMemo(
    () => liabilitiesSharesAll.slice(search.l_offset, search.l_offset + pageSize),
    [liabilitiesSharesAll, search.l_offset, pageSize],
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
        currency={DEFAULT_CURRENCY}
        excludedCount={summary.excludedByCurrency.length}
      />

      <div className="mb-3 flex justify-end">
        <ViewToggle value={search.view} onChange={setView} />
      </div>

      <div className="grid grid-cols-2 items-start gap-4">
        <BalanceColumn
          label="Assets"
          total={summary.assetsTotal}
          rows={assetsPaged}
          shares={assetsPagedShares}
          totalRowCount={assetsSorted.length}
          sortDir={assetsSortDir}
          onToggleSort={() => toggleSort('a')}
          offset={search.a_offset}
          onOffsetChange={(off) => setOffset('a', off)}
          emptyText="No assets"
          view={search.view}
          historyByAccount={historyByAccount}
        />
        <BalanceColumn
          label="Liabilities"
          total={summary.liabilitiesTotal}
          rows={liabilitiesPaged}
          shares={liabilitiesPagedShares}
          totalRowCount={liabilitiesSorted.length}
          sortDir={liabilitiesSortDir}
          onToggleSort={() => toggleSort('l')}
          offset={search.l_offset}
          onOffsetChange={(off) => setOffset('l', off)}
          emptyText="No liabilities"
          view={search.view}
          historyByAccount={historyByAccount}
        />
      </div>
    </div>
  );
}
