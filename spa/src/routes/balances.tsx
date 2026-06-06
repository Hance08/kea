import { AccountListRow } from '@/components/AccountListRow';
import { NetWorthCard } from '@/components/NetWorthCard';
import { TypeTotalCard } from '@/components/TypeTotalCard';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getBalances } from '@/lib/api';
import { summarizeBalances } from '@/lib/balances';
import { useQuery } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';

const DEFAULT_CURRENCY = (import.meta.env.VITE_DEFAULT_CURRENCY as string) || 'USD';

export const Route = createFileRoute('/balances')({
  component: BalancesPage,
});

function BalancesPage() {
  const query = useQuery({ queryKey: ['balances'], queryFn: getBalances });

  if (query.isPending) {
    return (
      <div>
        <Skeleton className="mb-6 h-32 w-full" />
        <div className="mb-6 grid grid-cols-2 gap-4">
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
        <div className="space-y-2">
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
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

  const rows = query.data.items;

  if (rows.length === 0) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <p className="max-w-md text-center text-sm text-muted-foreground">
          No accounts yet — run <code className="font-mono">kea ledger add</code> then create one
          via the CLI.
        </p>
      </div>
    );
  }

  const summary = summarizeBalances(rows, DEFAULT_CURRENCY);

  return (
    <div>
      <NetWorthCard
        netWorth={summary.netWorth}
        assetsTotal={summary.assetsTotal}
        liabilitiesTotal={summary.liabilitiesTotal}
        currency={DEFAULT_CURRENCY}
        excludedCount={summary.excluded.length}
      />

      <div className="mb-6 grid grid-cols-2 gap-4">
        <TypeTotalCard label="Assets" amount={summary.assetsTotal} currency={DEFAULT_CURRENCY} />
        <TypeTotalCard
          label="Liabilities"
          amount={summary.liabilitiesTotal}
          currency={DEFAULT_CURRENCY}
          negative
        />
      </div>

      <div className="rounded-md border bg-card">
        {summary.included.map((row) => (
          <AccountListRow key={row.account_id} row={row} />
        ))}
      </div>
    </div>
  );
}
