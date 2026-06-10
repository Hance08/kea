import { AccountFilters } from '@/components/accounts/AccountFilters';
import { AccountSearchResults } from '@/components/accounts/AccountSearchResults';
import { AccountTree } from '@/components/accounts/AccountTree';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getAccountTree, listAccounts } from '@/lib/accounts';
import type { AccountsSearch } from '@/lib/accounts-search-params';
import { getBalances } from '@/lib/api';
import { useQuery } from '@tanstack/react-query';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';

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
    navigate({ search: { include_hidden: false } });
  };

  const treeQuery = useQuery({
    queryKey: ['accounts', 'tree', { include_hidden: search.include_hidden }],
    queryFn: () => getAccountTree({ include_hidden: search.include_hidden }),
    enabled: !search.q,
  });

  const balancesQuery = useQuery({
    queryKey: ['balances'],
    queryFn: getBalances,
  });

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Accounts</h1>
        <Button asChild size="sm">
          <Link to="/accounts/new" search={{ include_hidden: false }}>
            + New account
          </Link>
        </Button>
      </div>

      <AccountFilters search={search} onChange={setSearch} onClear={clear} />

      {search.q && <SearchMode search={search} />}

      {!search.q && treeQuery.isPending && (
        <div className="space-y-2">
          {[0, 1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      )}

      {!search.q && treeQuery.isError && (
        <Alert variant="destructive">
          <AlertTitle>Failed to load accounts</AlertTitle>
          <AlertDescription className="mt-2 space-y-3">
            <div>
              {treeQuery.error instanceof Error ? treeQuery.error.message : 'Unknown error'}
            </div>
            <Button onClick={() => treeQuery.refetch()} size="sm">
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {!search.q && treeQuery.isSuccess && treeQuery.data.length === 0 && (
        <div className="rounded-md border bg-card p-8 text-center text-sm text-muted-foreground">
          No accounts yet — click + New account to create your first.
        </div>
      )}

      {!search.q && treeQuery.isSuccess && treeQuery.data.length > 0 && (
        <AccountTree nodes={treeQuery.data} balances={balancesQuery.data?.items} />
      )}
    </div>
  );
}

function SearchMode({ search }: { search: AccountsSearch }) {
  const balancesQuery = useQuery({
    queryKey: ['balances'],
    queryFn: getBalances,
  });
  const query = useQuery({
    queryKey: [
      'accounts',
      'search',
      { q: search.q, type: search.type, include_hidden: search.include_hidden },
    ],
    queryFn: () =>
      listAccounts({
        q: search.q,
        type: search.type,
        include_hidden: search.include_hidden,
        limit: 100,
      }),
  });

  if (query.isPending) {
    return (
      <div className="space-y-2">
        {[0, 1, 2, 3, 4].map((i) => (
          <Skeleton key={i} className="h-8 w-full" />
        ))}
      </div>
    );
  }

  if (query.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Search failed</AlertTitle>
        <AlertDescription className="mt-2 space-y-3">
          <div>{query.error instanceof Error ? query.error.message : 'Unknown error'}</div>
          <Button onClick={() => query.refetch()} size="sm">
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  if (query.data.items.length === 0) {
    return (
      <div className="rounded-md border bg-card p-8 text-center text-sm text-muted-foreground">
        No accounts match "{search.q}".
      </div>
    );
  }

  return (
    <AccountSearchResults
      accounts={query.data.items}
      balances={balancesQuery.data?.items}
      totalCount={query.data.total_count}
    />
  );
}
