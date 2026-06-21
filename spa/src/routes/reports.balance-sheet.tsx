import { AsOfPicker } from '@/components/reports/AsOfPicker';
import { CurrencyFooter } from '@/components/reports/CurrencyFooter';
import { KpiCard } from '@/components/reports/KpiCard';
import { NetWorthChart } from '@/components/reports/NetWorthChart';
import { ReportRowTable } from '@/components/reports/ReportRowTable';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getBalances } from '@/lib/api';
import { useBalanceSheet, useNetWorthSeries } from '@/lib/hooks/useReport';
import { type AsOfSearchParams, parseAsOfSearch } from '@/lib/reports-search-params';
import { useAmountFormat, useServerConfig } from '@/lib/server-config';
import { useQuery } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useMemo } from 'react';

export const Route = createFileRoute('/reports/balance-sheet')({
  validateSearch: (s): AsOfSearchParams => parseAsOfSearch(s),
  component: BalanceSheetPage,
});

function BalanceSheetPage() {
  const { defaults } = useServerConfig();
  const { formatCents } = useAmountFormat();
  const currency = defaults.currency;
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/reports/balance-sheet' });
  const query = useBalanceSheet(search);
  const seriesQuery = useNetWorthSeries();

  const balancesQuery = useQuery({ queryKey: ['balances'], queryFn: getBalances });
  const nameToId = useMemo(() => {
    const m = new Map<string, number>();
    if (balancesQuery.data)
      for (const row of balancesQuery.data.items) m.set(row.name, row.account_id);
    return m;
  }, [balancesQuery.data]);

  const setAsOf = (v: number | undefined) =>
    navigate({ search: () => (v === undefined ? {} : { as_of: v }) });

  if (query.isPending) {
    return (
      <div className="space-y-4">
        <AsOfPicker label="As of" value={search.as_of} onChange={setAsOf} />
        <Skeleton className="h-20" />
        <Skeleton className="h-48" />
      </div>
    );
  }
  if (query.isError) {
    return (
      <div className="space-y-3">
        <AsOfPicker label="As of" value={search.as_of} onChange={setAsOf} />
        <Alert variant="destructive">
          <AlertTitle>Failed to load balance sheet</AlertTitle>
          <AlertDescription className="mt-2 space-y-3">
            <div>{query.error instanceof Error ? query.error.message : 'Unknown error'}</div>
            <Button onClick={() => query.refetch()} size="sm">
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  const result = query.data;
  const ta = result.total_assets[currency] ?? 0;
  const tl = result.total_liabilities[currency] ?? 0;
  const te = result.total_equity[currency] ?? 0;
  const nw = result.net_worth[currency] ?? 0;
  const liabilities = (result.liabilities ?? []).filter((r) => r.currency === currency);

  const assetsCount = (result.assets ?? []).filter((r) => r.currency === currency).length;
  const equityCount = (result.equity ?? []).filter((r) => r.currency === currency).length;

  const seriesForCurrency = seriesQuery.data?.items.find((s) => s.currency === currency);
  const asOfDate =
    search.as_of !== undefined
      ? new Date(search.as_of * 1000).toISOString().slice(0, 10)
      : undefined;
  const chartPoints = seriesForCurrency
    ? asOfDate
      ? seriesForCurrency.points.filter((p) => p.date <= asOfDate)
      : seriesForCurrency.points
    : [];

  if (assetsCount === 0 && liabilities.length === 0 && equityCount === 0) {
    return (
      <div className="space-y-4">
        <AsOfPicker label="As of" value={search.as_of} onChange={setAsOf} />
        <p className="text-sm text-muted-foreground">No balance-sheet activity at this date.</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <AsOfPicker label="As of" value={search.as_of} onChange={setAsOf} />

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {assetsCount > 0 && (
          <KpiCard label="Total Assets" amount={ta} currency={currency} variant="green" />
        )}
        {liabilities.length > 0 && (
          <KpiCard label="Total Liabilities" amount={tl} currency={currency} variant="red" />
        )}
        {equityCount > 0 && (
          <KpiCard label="Total Equity" amount={te} currency={currency} variant="neutral" />
        )}
        <KpiCard label="Net Worth" amount={nw} currency={currency} variant="neutral" />
      </div>

      <section>
        <h2 className="mb-2 text-sm font-semibold">Net worth over time</h2>
        {seriesQuery.isPending ? (
          <Skeleton className="h-48 w-full" />
        ) : seriesQuery.isError ? (
          <Alert variant="destructive">
            <AlertTitle>Failed to load net worth history</AlertTitle>
            <AlertDescription className="mt-2 space-y-3">
              <div>
                {seriesQuery.error instanceof Error ? seriesQuery.error.message : 'Unknown error'}
              </div>
              <Button onClick={() => seriesQuery.refetch()} size="sm">
                Retry
              </Button>
            </AlertDescription>
          </Alert>
        ) : chartPoints.length < 2 ? (
          <p className="text-sm text-muted-foreground">Not enough history to chart net worth.</p>
        ) : (
          <NetWorthChart
            points={chartPoints}
            currency={currency}
            formatCents={(c) => formatCents(c, currency)}
            asOfDate={asOfDate}
          />
        )}
      </section>

      {liabilities.length > 0 && (
        <section>
          <h2 className="mb-2 text-sm font-semibold">Liabilities</h2>
          <ReportRowTable
            rows={liabilities}
            currency={currency}
            nameToId={nameToId}
            period={null}
          />
        </section>
      )}

      <CurrencyFooter
        defaultCurrency={currency}
        entries={[
          { label: 'Assets', byCurrency: result.total_assets },
          { label: 'Liabilities', byCurrency: result.total_liabilities },
          { label: 'Equity', byCurrency: result.total_equity },
          { label: 'Net worth', byCurrency: result.net_worth },
        ]}
      />
    </div>
  );
}
