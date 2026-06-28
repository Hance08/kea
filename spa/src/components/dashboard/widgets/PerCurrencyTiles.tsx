import { useQuery } from '@tanstack/react-query';
import { getNetWorth } from '../../../lib/api';
import { useAmountFormat } from '../../../lib/server-config';

export function PerCurrencyTiles() {
  const { formatCents } = useAmountFormat();
  const q = useQuery({
    queryKey: ['dashboard', 'net-worth', 'now'],
    queryFn: () => getNetWorth(),
    staleTime: 5 * 60_000,
  });
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;
  const entries = Object.entries(q.data?.net_worth ?? {});
  if (entries.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        No balances yet
      </div>
    );
  }
  return (
    <div className="flex h-full flex-col">
      <div className="mb-1 text-xs text-muted-foreground">Per-Currency Net Worth</div>
      <ul className="flex flex-col gap-1 overflow-auto">
        {entries.map(([ccy, amount]) => (
          <li key={ccy} className="flex items-baseline justify-between text-sm">
            <span className="font-medium text-muted-foreground">{ccy}</span>
            <span className="tabular-nums">{formatCents(amount, ccy)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
