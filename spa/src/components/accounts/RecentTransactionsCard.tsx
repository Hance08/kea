import { StatusText } from '@/components/transactions/StatusText';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { Skeleton } from '@/components/ui/skeleton';
import { formatCents } from '@/lib/format';
import { displayAmount } from '@/lib/transactionDisplay';
import { listTransactions } from '@/lib/transactions';
import { DEFAULT_TRANSACTIONS_LIMIT } from '@/lib/transactions-search-params';
import type { TransactionDetail } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';

interface Props {
  accountId: number;
}

function formatDate(unix: number): string {
  const d = new Date(unix * 1000);
  return d.toISOString().slice(0, 10);
}

export function RecentTransactionsCard({ accountId }: Props) {
  const query = useQuery({
    queryKey: ['transactions', { account_id: accountId, limit: 20 }],
    queryFn: () => listTransactions({ account_id: accountId }, { limit: 20 }),
  });

  return (
    <div className="rounded-md border bg-card">
      <div className="flex items-center justify-between border-b bg-muted/40 px-3 py-2 text-xs font-semibold uppercase text-muted-foreground">
        <span>Recent transactions</span>
        <Link
          to="/transactions"
          search={{ account_id: accountId, limit: DEFAULT_TRANSACTIONS_LIMIT, offset: 0 }}
          className="font-normal lowercase hover:underline"
        >
          View all →
        </Link>
      </div>
      {query.isPending && (
        <div className="space-y-2 p-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-6 w-full" />
          ))}
        </div>
      )}
      {query.isSuccess && query.data.items.length === 0 && (
        <div className="px-3 py-4 text-sm text-muted-foreground">No transactions yet.</div>
      )}
      {query.isSuccess &&
        query.data.items.map((tx: TransactionDetail) => {
          const { amount, currency } = displayAmount(tx.splits, tx.type);
          return (
            <Link
              key={tx.id}
              to="/transactions/$id"
              params={{ id: String(tx.id) }}
              search={{ limit: DEFAULT_TRANSACTIONS_LIMIT, offset: 0 }}
              className="grid grid-cols-[6rem_5rem_1fr_6rem_5rem] items-center gap-3 border-b border-border/60 px-3 py-1.5 text-sm last:border-b-0 hover:bg-muted/40"
            >
              <span className="tabular-nums">{formatDate(tx.timestamp)}</span>
              <TypeBadge type={tx.type} />
              <span className="truncate">{tx.description}</span>
              <span className="text-right tabular-nums">{formatCents(amount, currency)}</span>
              <StatusText status={tx.status} />
            </Link>
          );
        })}
    </div>
  );
}
