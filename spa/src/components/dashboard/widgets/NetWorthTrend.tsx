import { useQuery } from '@tanstack/react-query';
import { fetchNetWorthSeries } from '../../../lib/api/reports';
import { useAmountFormat, useServerConfig } from '../../../lib/server-config';
import { NetWorthChart } from '../../reports/NetWorthChart';

export type NetWorthTrendConfig = { range: '6m' | '12m' | '24m' | 'all' };

export const NET_WORTH_TREND_DEFAULT: NetWorthTrendConfig = { range: '12m' };

const RANGE_TO_DAYS: Record<NetWorthTrendConfig['range'], number | null> = {
  '6m': 183,
  '12m': 365,
  '24m': 730,
  all: null,
};

export function NetWorthTrend({ config }: { config: NetWorthTrendConfig }) {
  const currency = useServerConfig().defaults.currency;
  const { formatCents } = useAmountFormat();
  const q = useQuery({
    queryKey: ['dashboard', 'net-worth-series'],
    queryFn: fetchNetWorthSeries,
    staleTime: 5 * 60_000,
  });
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const series = q.data?.items.find((s) => s.currency === currency);
  if (!series || series.points.length < 2) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        Not enough data yet
      </div>
    );
  }
  const days = RANGE_TO_DAYS[config.range];
  const points = days === null ? series.points : series.points.slice(-days);

  return (
    <div className="flex h-full flex-col">
      <div className="mb-1 text-xs font-medium text-muted-foreground">Net Worth Trend</div>
      <div className="flex-1">
        <NetWorthChart
          points={points}
          currency={currency}
          formatCents={(c) => formatCents(c, currency)}
          className="h-full w-full"
        />
      </div>
    </div>
  );
}

export function NetWorthTrendConfigForm({
  config,
  onChange,
}: {
  config: NetWorthTrendConfig;
  onChange: (c: NetWorthTrendConfig) => void;
}) {
  return (
    <div className="flex flex-col gap-1 text-sm">
      <label htmlFor="net-worth-range" className="text-xs font-medium">
        Range
      </label>
      <select
        id="net-worth-range"
        value={config.range}
        onChange={(e) => onChange({ range: e.target.value as NetWorthTrendConfig['range'] })}
        className="rounded border px-2 py-1 text-sm"
      >
        <option value="6m">6 months</option>
        <option value="12m">12 months</option>
        <option value="24m">24 months</option>
        <option value="all">All</option>
      </select>
    </div>
  );
}
