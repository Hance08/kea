import { AccountFilters } from '@/components/accounts/AccountFilters';
import { AccountSearchResults } from '@/components/accounts/AccountSearchResults';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { listAccounts } from '@/lib/accounts';
import type { AccountsSearch } from '@/lib/accounts-search-params';
import { getBalances } from '@/lib/api';
import type { Account } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';
import { useMemo } from 'react';

export const Route = createFileRoute('/accounts/')({
  component: AccountsListPage,
});

function AccountsListPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/accounts' });

  const setSearch = (partial: Partial<AccountsSearch>) => {
    navigate({ search: (prev) => ({ ...prev, ...partial }) });
  };
  const clear = () => {
    navigate({ search: { include_hidden: false, show_parents: false } });
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

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Accounts</h1>
        <Button asChild size="sm">
          <Link to="/accounts/new" search={{ include_hidden: false, show_parents: false }}>
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
        <AccountSearchResults
          accounts={filteredItems}
          balances={balancesQuery.data?.items}
          totalCount={accountsQuery.data.total_count}
        />
      )}
    </div>
  );
}
