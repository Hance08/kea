import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { useAmountFormat } from '../../../lib/server-config';
import { listTransactions } from '../../../lib/transactions';
import { DEFAULT_TRANSACTIONS_LIMIT } from '../../../lib/transactions-search-params';

export type RecentTxnConfig = { limit: 5 | 10 | 20 };
export const RECENT_TXN_DEFAULT: RecentTxnConfig = { limit: 10 };

export function RecentTransactions({ config }: { config: RecentTxnConfig }) {
  const { formatAmount } = useAmountFormat();
  const q = useQuery({
    queryKey: ['dashboard', 'recent-transactions', config.limit],
    queryFn: () => listTransactions({}, { limit: config.limit, include_count: false }),
    staleTime: 30_000,
  });
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;
  const items = q.data?.items ?? [];
  if (items.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        No transactions yet
      </div>
    );
  }
  return (
    <div className="flex h-full flex-col">
      <ul className="flex flex-col gap-1 overflow-auto">
        {items.map((t) => {
          const expenseAmt = t.splits
            .filter((s) => s.account_type === 'E')
            .reduce((sum, s) => sum + s.amount, 0);
          return (
            <li key={t.id} className="text-xs">
              <Link
                to="/transactions/$id"
                params={{ id: String(t.id) }}
                search={{ limit: DEFAULT_TRANSACTIONS_LIMIT, offset: 0 }}
                className="flex items-baseline justify-between hover:underline"
              >
                <span className="truncate">{t.description || '(no description)'}</span>
                <span className="tabular-nums text-muted-foreground">
                  {formatAmount(expenseAmt)}
                </span>
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export function RecentTxnConfigForm({
  config,
  onChange,
}: { config: RecentTxnConfig; onChange: (c: RecentTxnConfig) => void }) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-xs font-medium">Number of transactions</span>
      <select
        value={config.limit}
        onChange={(e) => onChange({ limit: Number(e.target.value) as RecentTxnConfig['limit'] })}
        className="rounded border px-2 py-1 text-sm"
      >
        <option value={5}>5</option>
        <option value={10}>10</option>
        <option value={20}>20</option>
      </select>
    </label>
  );
}
