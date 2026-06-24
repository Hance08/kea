import { CompositionBar, partitionForComposition } from '@/components/reports/CompositionBar';
import { CurrencyFooter } from '@/components/reports/CurrencyFooter';
import { KpiCard } from '@/components/reports/KpiCard';
import { PeriodPicker } from '@/components/reports/PeriodPicker';
import { ReportRowTable } from '@/components/reports/ReportRowTable';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getBalances } from '@/lib/api';
import { useIncomeBreakdown } from '@/lib/hooks/useReport';
import { resolvePeriod } from '@/lib/period';
import { type PeriodSearchParams, parsePeriodSearch } from '@/lib/reports-search-params';
import { useServerConfig } from '@/lib/server-config';
import { useQuery } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useMemo } from 'react';

export const Route = createFileRoute('/reports/income-breakdown')({
  validateSearch: (s): PeriodSearchParams => parsePeriodSearch(s),
  component: IncomeBreakdownPage,
});

function IncomeBreakdownPage() {
  const { defaults } = useServerConfig();
  const currency = defaults.currency;
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/reports/income-breakdown' });
  const period = useMemo(() => resolvePeriod(search), [search]);
  const query = useIncomeBreakdown(period.apiParams);

  const balancesQuery = useQuery({ queryKey: ['balances'], queryFn: getBalances });
  const nameToId = useMemo(() => {
    const m = new Map<string, number>();
    if (balancesQuery.data)
      for (const row of balancesQuery.data.items) m.set(row.name, row.account_id);
    return m;
  }, [balancesQuery.data]);

  const setSearch = (next: Partial<PeriodSearchParams>) =>
    navigate({ search: () => ({ ...search, ...next }) });

  if (query.isPending) {
    return (
      <div className="space-y-4">
        <PeriodPicker value={search} onChange={setSearch} label={period.label} />
        <Skeleton className="h-20" />
        <Skeleton className="h-48" />
      </div>
    );
  }
  if (query.isError) {
    return (
      <div className="space-y-3">
        <PeriodPicker value={search} onChange={setSearch} label={period.label} />
        <Alert variant="destructive">
          <AlertTitle>Failed to load income breakdown</AlertTitle>
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
  const income = result.total_income[currency] ?? 0;
  const rows = (result.income_rows ?? []).filter((r) => r.currency === currency);

  const { swatchColors } = partitionForComposition(rows, income, 'income');

  return (
    <div className="space-y-6">
      <PeriodPicker value={search} onChange={setSearch} label={period.label} />
      {rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">No income in {period.label}.</p>
      ) : (
        <>
          <div className="max-w-xs">
            <KpiCard label="Total Income" amount={income} currency={currency} variant="green" />
          </div>
          <section>
            <h2 className="mb-2 text-sm font-semibold">Composition</h2>
            <CompositionBar rows={rows} total={income} currency={currency} variant="income" />
          </section>
          <ReportRowTable
            rows={rows}
            currency={currency}
            nameToId={nameToId}
            period={{ startUnix: period.startUnix, endUnix: period.endUnix }}
            swatchColors={swatchColors}
          />
          <CurrencyFooter
            defaultCurrency={currency}
            entries={[{ label: 'Income', byCurrency: result.total_income }]}
          />
        </>
      )}
    </div>
  );
}
