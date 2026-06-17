import { CurrencyFooter } from '@/components/reports/CurrencyFooter';
import { KpiCard } from '@/components/reports/KpiCard';
import { PeriodPicker } from '@/components/reports/PeriodPicker';
import { ProportionBar } from '@/components/reports/ProportionBar';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { fetchIncomeStatement } from '@/lib/api/reports';
import { useIncomeStatement } from '@/lib/hooks/useReport';
import { previousPeriod, resolvePeriod } from '@/lib/period';
import { type PeriodSearchParams, parsePeriodSearch } from '@/lib/reports-search-params';
import { useServerConfig } from '@/lib/server-config';
import { useQuery } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useMemo } from 'react';

export const Route = createFileRoute('/reports/income-statement')({
  validateSearch: (s): PeriodSearchParams => parsePeriodSearch(s),
  component: IncomeStatementPage,
});

function IncomeStatementPage() {
  const { defaults } = useServerConfig();
  const currency = defaults.currency;
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/reports/income-statement' });
  const period = useMemo(() => resolvePeriod(search), [search]);
  const query = useIncomeStatement(period.apiParams);
  const previous = useMemo(() => previousPeriod(search, period), [search, period]);
  const previousQuery = useQuery({
    queryKey: ['reports', 'income-statement', 'previous', previous?.apiParams],
    queryFn: () => fetchIncomeStatement(previous!.apiParams),
    enabled: previous !== null,
  });

  const setSearch = (next: Partial<PeriodSearchParams>) => {
    navigate({ search: () => ({ ...search, ...next }) });
  };

  if (query.isPending) {
    return (
      <div className="space-y-4">
        <PeriodPicker value={search} onChange={setSearch} label={period.label} />
        <div className="grid grid-cols-3 gap-3">
          <Skeleton className="h-20" />
          <Skeleton className="h-20" />
          <Skeleton className="h-20" />
        </div>
        <Skeleton className="h-48" />
      </div>
    );
  }

  if (query.isError) {
    return (
      <div className="space-y-3">
        <PeriodPicker value={search} onChange={setSearch} label={period.label} />
        <Alert variant="destructive">
          <AlertTitle>Failed to load income statement</AlertTitle>
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
  const incomeRows = (result.income_rows ?? []).filter((r) => r.currency === currency);
  const expenseRows = (result.expense_rows ?? []).filter((r) => r.currency === currency);
  const isEmpty = incomeRows.length === 0 && expenseRows.length === 0;

  const income = result.total_income[currency] ?? 0;
  const expense = result.total_expense[currency] ?? 0;
  const net = result.net_amount[currency] ?? 0;
  const growth = result.net_worth_growth_pct[currency];

  const netSubLine =
    growth === undefined || Number.isNaN(growth)
      ? undefined
      : `${growth >= 0 ? '▲' : '▼'} ${Math.abs(growth).toFixed(1)}% net worth`;

  return (
    <div className="space-y-6">
      <PeriodPicker value={search} onChange={setSearch} label={period.label} />

      {isEmpty ? (
        <p className="text-sm text-muted-foreground">
          No income or expenses in {period.label}. Try a wider range.
        </p>
      ) : (
        <>
          <div className="grid grid-cols-3 gap-3">
            <KpiCard
              label="Income"
              amount={income}
              currency={currency}
              variant="green"
              diff={
                previousQuery.isSuccess
                  ? {
                      delta: income - (previousQuery.data.total_income[currency] ?? 0),
                      prevAmount: previousQuery.data.total_income[currency] ?? 0,
                      goodWhen: 'up',
                    }
                  : undefined
              }
            />
            <KpiCard
              label="Expense"
              amount={expense}
              currency={currency}
              variant="red"
              diff={
                previousQuery.isSuccess
                  ? {
                      delta: expense - (previousQuery.data.total_expense[currency] ?? 0),
                      prevAmount: previousQuery.data.total_expense[currency] ?? 0,
                      goodWhen: 'down',
                    }
                  : undefined
              }
            />
            <KpiCard
              label="Net"
              amount={net}
              currency={currency}
              variant="neutral"
              subLine={netSubLine}
              diff={
                previousQuery.isSuccess
                  ? {
                      delta: net - (previousQuery.data.net_amount[currency] ?? 0),
                      prevAmount: previousQuery.data.net_amount[currency] ?? 0,
                      goodWhen: 'up',
                    }
                  : undefined
              }
            />
          </div>

          <div className="grid grid-cols-2 gap-6">
            <section>
              <h2 className="mb-2 text-sm font-semibold">Income mix</h2>
              <ProportionBar
                rows={incomeRows}
                total={income}
                currency={currency}
                limit={5}
                variant="income"
              />
            </section>
            <section>
              <h2 className="mb-2 text-sm font-semibold">Expense mix</h2>
              <ProportionBar
                rows={expenseRows}
                total={expense}
                currency={currency}
                limit={5}
                variant="expense"
              />
            </section>
          </div>

          <CurrencyFooter
            defaultCurrency={currency}
            entries={[
              { label: 'Income', byCurrency: result.total_income },
              { label: 'Expense', byCurrency: result.total_expense },
              { label: 'Net', byCurrency: result.net_amount },
            ]}
          />
        </>
      )}
    </div>
  );
}
