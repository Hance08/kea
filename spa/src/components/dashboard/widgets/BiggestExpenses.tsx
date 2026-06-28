import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { useAmountFormat, useServerConfig } from '../../../lib/server-config';
import { listTransactions } from '../../../lib/transactions';
import { DEFAULT_TRANSACTIONS_LIMIT } from '../../../lib/transactions-search-params';

export type BiggestExpensesConfig = { limit: 5 | 10 };
export const BIGGEST_EXPENSES_DEFAULT: BiggestExpensesConfig = { limit: 5 };

function startOfMonthUnix(): number {
  const d = new Date();
  d.setUTCDate(1);
  d.setUTCHours(0, 0, 0, 0);
  return Math.floor(d.getTime() / 1000);
}

function endOfMonthUnix(): number {
  const d = new Date();
  d.setUTCMonth(d.getUTCMonth() + 1, 1);
  d.setUTCHours(0, 0, 0, 0);
  return Math.floor(d.getTime() / 1000) - 1;
}

export function BiggestExpenses({ config }: { config: BiggestExpensesConfig }) {
  const currency = useServerConfig().defaults.currency;
  const { formatCents } = useAmountFormat();
  const start = startOfMonthUnix();
  const end = endOfMonthUnix();
  const q = useQuery({
    queryKey: ['dashboard', 'biggest-expenses', start, end],
    queryFn: () =>
      listTransactions(
        { type: 'Expense', start_time: start, end_time: end },
        { limit: 500, include_count: false },
      ),
    staleTime: 60_000,
  });

  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const ranked = (q.data?.items ?? [])
    .map((t) => {
      const amount = t.splits
        .filter((s) => s.account_type === 'E' && s.currency === currency)
        .reduce((sum, s) => sum + s.amount, 0);
      return { id: t.id, description: t.description, amount };
    })
    .filter((r) => r.amount > 0)
    .sort((a, b) => b.amount - a.amount)
    .slice(0, config.limit);

  if (ranked.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        No expenses this month
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="mb-1 text-xs text-muted-foreground">Biggest Expenses This Month</div>
      <ul className="flex flex-col gap-1 overflow-auto">
        {ranked.map((r) => (
          <li key={r.id} className="text-xs">
            <Link
              to="/transactions/$id"
              params={{ id: String(r.id) }}
              search={{ limit: DEFAULT_TRANSACTIONS_LIMIT, offset: 0 }}
              className="flex items-baseline justify-between hover:underline"
            >
              <span className="truncate">{r.description || '(no description)'}</span>
              <span className="tabular-nums">{formatCents(r.amount, currency)}</span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function BiggestExpensesConfigForm({
  config,
  onChange,
}: { config: BiggestExpensesConfig; onChange: (c: BiggestExpensesConfig) => void }) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-xs font-medium">Number to show</span>
      <select
        value={config.limit}
        onChange={(e) =>
          onChange({ limit: Number(e.target.value) as BiggestExpensesConfig['limit'] })
        }
        className="rounded border px-2 py-1 text-sm"
      >
        <option value={5}>5</option>
        <option value={10}>10</option>
      </select>
    </label>
  );
}
