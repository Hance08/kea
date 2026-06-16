import { CurrencyFooter } from '@/components/reports/CurrencyFooter';
import { KpiCard } from '@/components/reports/KpiCard';
import { PeriodPicker } from '@/components/reports/PeriodPicker';
import { ProportionBar } from '@/components/reports/ProportionBar';
import { ReportRowTable } from '@/components/reports/ReportRowTable';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getBalances } from '@/lib/api';
import { useIncomeStatement } from '@/lib/hooks/useReport';
import { resolvePeriod } from '@/lib/period';
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

  const balancesQuery = useQuery({ queryKey: ['balances'], queryFn: getBalances });
  const nameToId = useMemo(() => {
    const m = new Map<string, number>();
    if (balancesQuery.data) {
      for (const row of balancesQuery.data.items) m.set(row.name, row.account_id);
    }
    return m;
  }, [balancesQuery.data]);

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
            <Button onClick={() => query.refetch()} size="sm">Retry</Button>
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  const result = query.data;
  const isEmpty =
    result.income_rows.length === 0 && result.expense_rows.length === 0;

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
            <KpiCard label="Income" amount={income} currency={currency} variant="green" />
            <KpiCard label="Expense" amount={expense} currency={currency} variant="red" />
            <KpiCard
              label="Net"
              amount={net}
              currency={currency}
              variant="neutral"
              subLine={netSubLine}
            />
          </div>

          <div className="grid grid-cols-2 gap-6">
            <section>
              <h2 className="mb-2 text-sm font-semibold">Income mix</h2>
              <ProportionBar
                rows={result.income_rows.filter((r) => r.currency === currency)}
                total={income}
                currency={currency}
                limit={8}
                variant="income"
              />
            </section>
            <section>
              <h2 className="mb-2 text-sm font-semibold">Expense mix</h2>
              <ProportionBar
                rows={result.expense_rows.filter((r) => r.currency === currency)}
                total={expense}
                currency={currency}
                limit={8}
                variant="expense"
              />
            </section>
          </div>

          <div className="grid grid-cols-2 gap-6">
            <section>
              <h2 className="mb-2 text-sm font-semibold">Income detail</h2>
              <ReportRowTable
                rows={result.income_rows.filter((r) => r.currency === currency)}
                currency={currency}
                nameToId={nameToId}
                period={{ startUnix: period.startUnix, endUnix: period.endUnix }}
              />
            </section>
            <section>
              <h2 className="mb-2 text-sm font-semibold">Expense detail</h2>
              <ReportRowTable
                rows={result.expense_rows.filter((r) => r.currency === currency)}
                currency={currency}
                nameToId={nameToId}
                period={{ startUnix: period.startUnix, endUnix: period.endUnix }}
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
