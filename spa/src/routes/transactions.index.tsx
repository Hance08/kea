import { FilterBar } from "@/components/transactions/FilterBar";
import { Pagination } from "@/components/transactions/Pagination";
import { TransactionsTable } from "@/components/transactions/TransactionsTable";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { listTransactions } from "@/lib/transactions";
import {
  type TransactionsSearch,
  searchToFilter,
  searchToListOptions,
} from "@/lib/transactions-search-params";
import { useQuery } from "@tanstack/react-query";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";

export const Route = createFileRoute("/transactions/")({
  component: TransactionsListPage,
});

function TransactionsListPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: "/transactions" });

  const filter = searchToFilter(search);
  const opts = searchToListOptions(search);

  const query = useQuery({
    queryKey: ["transactions", { ...filter, ...opts }],
    queryFn: () => listTransactions(filter, opts),
  });

  const setSearch = (partial: Partial<TransactionsSearch>) => {
    navigate({
      search: (prev) => ({ ...prev, ...partial, offset: 0 }),
    });
  };
  const clear = () => {
    navigate({ search: { limit: search.limit, offset: 0 } });
  };
  const setOffset = (offset: number) => {
    navigate({ search: (prev) => ({ ...prev, offset }) });
  };

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Transactions</h1>
        <Button asChild size="sm">
          <Link to="/transactions/new" search={search}>
            + New transaction
          </Link>
        </Button>
      </div>

      <FilterBar search={search} onChange={setSearch} onClear={clear} />

      {query.isPending && (
        <div className="space-y-2">
          {[0, 1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-9 w-full" />
          ))}
        </div>
      )}

      {query.isError && (
        <Alert variant="destructive">
          <AlertTitle>Failed to load transactions</AlertTitle>
          <AlertDescription className="mt-2 space-y-3">
            <div>
              {query.error instanceof Error
                ? query.error.message
                : "Unknown error"}
            </div>
            <Button onClick={() => query.refetch()} size="sm">
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {query.isSuccess && query.data.items.length === 0 && (
        <div className="rounded-md border bg-card p-8 text-center text-sm text-muted-foreground">
          No transactions match these filters.
        </div>
      )}

      {query.isSuccess && query.data.items.length > 0 && (
        <>
          <TransactionsTable items={query.data.items} search={search} />
          <Pagination
            total={query.data.total_count}
            limit={search.limit}
            offset={search.offset}
            onChange={setOffset}
          />
        </>
      )}
    </div>
  );
}
