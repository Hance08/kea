import { AccountFilters } from '@/components/accounts/AccountFilters';
import { AccountSearchResults } from '@/components/accounts/AccountSearchResults';
import { Pagination } from '@/components/transactions/Pagination';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { listAccounts, naturalAmount } from '@/lib/accounts';
import { clearFilters, makeFilterMemoryLoader } from '@/lib/filter-memory';
import { type AccountsSearch, parseAccountsSearch } from '@/lib/accounts-search-params';
import { getBalances } from '@/lib/api';
import type { Account } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';
import { useMemo } from 'react';

const ACCOUNTS_DEFAULTS = parseAccountsSearch({});

export const Route = createFileRoute('/accounts/')({
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<AccountsSearch>({
    pageId: 'accounts',
    defaults: ACCOUNTS_DEFAULTS,
    redirectTo: '/accounts',
  }),
  component: AccountsListPage,
});

function AccountsListPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/accounts' });

  // Filter changes reset offset to 0 — without this, narrowing a result
  // set could land the user on a page that no longer exists.
  const setSearch = (partial: Partial<AccountsSearch>) => {
    navigate({ search: (prev) => ({ ...prev, ...partial, offset: 0 }) });
  };
  const clear = () => {
    clearFilters('accounts');
    navigate({
      search: { include_hidden: false, show_parents: false, limit: search.limit, offset: 0 },
    });
  };
  const setOffset = (offset: number) => {
    navigate({ search: (prev) => ({ ...prev, offset }) });
  };

  const balancesQuery = useQuery({
    queryKey: ['balances'],
    queryFn: getBalances,
  });

  const accountsQuery = useQuery({
    queryKey: [
      'accounts',
      'list',
      {
        q: search.q,
        type: search.type,
        include_hidden: search.include_hidden,
      },
    ],
    queryFn: () =>
      listAccounts({
        q: search.q,
        type: search.type,
        include_hidden: search.include_hidden,
        limit: search.q ? 100 : undefined,
      }),
  });

  const parentIds = useMemo(() => {
    const s = new Set<number>();
    for (const b of balancesQuery.data?.items ?? []) {
      if (b.parent_id != null) s.add(b.parent_id);
    }
    return s;
  }, [balancesQuery.data]);

  const filteredItems: Account[] = useMemo(() => {
    const items = accountsQuery.data?.items ?? [];
    if (search.show_parents) return items;
    return items.filter((acc) => !parentIds.has(acc.id));
  }, [accountsQuery.data, parentIds, search.show_parents]);

  // Sort by balance — accounts with no balance entry (still loading, or
  // never had any transactions) sort to the end in either direction so
  // they don't crowd out the meaningful rows on the first page.
  const balanceById = useMemo(() => {
    const m = new Map<number, number>();
    for (const b of balancesQuery.data?.items ?? []) m.set(b.account_id, b.amount);
    return m;
  }, [balancesQuery.data]);

  const sortDir: 'asc' | 'desc' = search.sort === 'balance_asc' ? 'asc' : 'desc';

  const sortedItems = useMemo(() => {
    const items = [...filteredItems];
    items.sort((a, b) => {
      const av = balanceById.get(a.id);
      const bv = balanceById.get(b.id);
      // Missing balances sort to the end regardless of direction.
      if (av === undefined && bv === undefined) return 0;
      if (av === undefined) return 1;
      if (bv === undefined) return -1;
      // Sort by the natural-direction amount so descending = "biggest of
      // this thing" regardless of the stored sign convention for the type.
      const an = naturalAmount(a.type, av);
      const bn = naturalAmount(b.type, bv);
      return sortDir === 'asc' ? an - bn : bn - an;
    });
    return items;
  }, [filteredItems, balanceById, sortDir]);

  // Paginate client-side. The leaf-vs-parent filter happens above, so the
  // total count and slice both apply to the post-filter list.
  const pagedItems = useMemo(
    () => sortedItems.slice(search.offset, search.offset + search.limit),
    [sortedItems, search.offset, search.limit],
  );

  const toggleSort = () => {
    navigate({
      search: (prev) => ({
        ...prev,
        sort: sortDir === 'desc' ? 'balance_asc' : 'balance_desc',
        offset: 0,
      }),
    });
  };

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Accounts</h1>
        <Button asChild size="sm">
          <Link
            to="/accounts/new"
            search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
          >
            + New account
          </Link>
        </Button>
      </div>

      <AccountFilters search={search} onChange={setSearch} onClear={clear} />

      {accountsQuery.isPending && (
        <div className="space-y-2">
          {[0, 1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      )}

      {accountsQuery.isError && (
        <Alert variant="destructive">
          <AlertTitle>Failed to load accounts</AlertTitle>
          <AlertDescription className="mt-2 space-y-3">
            <div>
              {accountsQuery.error instanceof Error ? accountsQuery.error.message : 'Unknown error'}
            </div>
            <Button onClick={() => accountsQuery.refetch()} size="sm">
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {accountsQuery.isSuccess && filteredItems.length === 0 && (
        <div className="rounded-md border bg-card p-8 text-center text-sm text-muted-foreground">
          {accountsQuery.data.items.length > 0 && !search.show_parents
            ? 'Only parent accounts match these filters. Toggle "Show parents" to include them.'
            : 'No accounts match these filters.'}
        </div>
      )}

      {accountsQuery.isSuccess && filteredItems.length > 0 && (
        <>
          <AccountSearchResults
            accounts={pagedItems}
            balances={balancesQuery.data?.items}
            sortDir={sortDir}
            onToggleSort={toggleSort}
          />
          <Pagination
            total={filteredItems.length}
            limit={search.limit}
            offset={search.offset}
            onChange={setOffset}
          />
        </>
      )}
    </div>
  );
}
