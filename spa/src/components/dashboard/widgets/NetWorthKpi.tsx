import { useQuery } from '@tanstack/react-query';
import { getNetWorth } from '../../../lib/api';
import { useAmountFormat, useServerConfig } from '../../../lib/server-config';

function endOfPrevMonthUnix(): number {
  const d = new Date();
  d.setUTCDate(1);
  d.setUTCHours(0, 0, 0, 0);
  return Math.floor(d.getTime() / 1000) - 1;
}

function useNetWorthQueries() {
  const cur = useQuery({
    queryKey: ['dashboard', 'net-worth', 'now'],
    queryFn: () => getNetWorth(),
    staleTime: 5 * 60_000,
  });
  const prev = useQuery({
    queryKey: ['dashboard', 'net-worth', 'prev-month-end'],
    queryFn: () => getNetWorth(endOfPrevMonthUnix()),
    staleTime: 5 * 60_000,
  });
  return { cur, prev };
}

export function NetWorthKpi() {
  const currency = useServerConfig().defaults.currency;
  const { formatCents } = useAmountFormat();
  const { cur } = useNetWorthQueries();

  if (cur.isLoading) return <Skeleton />;
  if (cur.isError) return <ErrorTile onRetry={() => cur.refetch()} />;

  const now = cur.data?.net_worth[currency] ?? 0;

  return (
    <div className="flex h-full flex-col justify-center">
      <div className="text-2xl font-semibold tabular-nums">{formatCents(now, currency)}</div>
    </div>
  );
}

export function NetWorthKpiHeader() {
  const currency = useServerConfig().defaults.currency;
  const { formatCents } = useAmountFormat();
  const { cur, prev } = useNetWorthQueries();
  if (cur.isLoading || prev.isLoading || cur.isError || prev.isError) return null;
  const now = cur.data?.net_worth[currency] ?? 0;
  const before = prev.data?.net_worth[currency] ?? 0;
  const delta = now - before;
  const pct = before === 0 ? null : (delta / Math.abs(before)) * 100;
  return (
    <span className={delta >= 0 ? 'text-emerald-600' : 'text-rose-600'}>
      {delta >= 0 ? '▲' : '▼'} {formatCents(Math.abs(delta), currency)}
      {pct !== null && ` (${pct.toFixed(1)}%)`}
    </span>
  );
}

function Skeleton() {
  return (
    <div className="flex h-full flex-col justify-center gap-2">
      <div className="h-3 w-16 animate-pulse rounded bg-muted" />
      <div className="h-7 w-32 animate-pulse rounded bg-muted" />
      <div className="h-3 w-24 animate-pulse rounded bg-muted" />
    </div>
  );
}

function ErrorTile({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-1 text-xs">
      <span className="text-muted-foreground">Failed to load</span>
      <button type="button" onClick={onRetry} className="rounded border px-2 py-0.5 hover:bg-muted">
        Retry
      </button>
    </div>
  );
}
