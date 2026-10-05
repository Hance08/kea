import { BudgetProgressBar } from '@/components/budgets/BudgetProgressBar';
import { MonthPicker } from '@/components/budgets/MonthPicker';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { depthIn, elapsedFraction, usedPct } from '@/lib/budgets';
import { makeFilterMemoryLoader } from '@/lib/filter-memory';
import { useBudgetReport } from '@/lib/hooks/useBudgets';
import { regularSubLine } from '@/lib/reportSubLine';
import { type MonthSearchParams, parseMonthSearch } from '@/lib/reports-search-params';
import { useAmountFormat } from '@/lib/server-config';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/reports/budget')({
  validateSearch: (s): MonthSearchParams => parseMonthSearch(s),
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<MonthSearchParams>({
    pageId: 'reports/budget',
    defaults: {},
    redirectTo: '/reports/budget',
  }),
  component: BudgetReportPage,
});

const shortName = (name: string) => name.split(':').slice(1).join(':') || name;

function usedLabel(budget: number, actual: number): string {
  const pct = usedPct(budget, actual);
  if (pct === null) return actual > 0 ? 'over' : '';
  return `${Math.round(pct)}%`;
}

function BudgetReportPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/reports/budget' });
  const query = useBudgetReport(search.month);
  const { formatCents } = useAmountFormat();
  const setMonth = (month: string | undefined) =>
    navigate({ search: () => (month ? { month } : {}) });

  const picker = <MonthPicker value={search.month} onChange={setMonth} />;

  if (query.isPending) {
    return (
      <div className="space-y-4">
        {picker}
        <Skeleton className="h-48" />
      </div>
    );
  }
  if (query.isError) {
    return (
      <div className="space-y-3">
        {picker}
        <Alert variant="destructive">
          <AlertTitle>Failed to load budget report</AlertTitle>
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

  const report = query.data;
  const elapsed = elapsedFraction(report.month);
  const names = report.rows.map((r) => r.account_name);

  return (
    <div className="space-y-4" data-testid="budget-report">
      {picker}
      {report.rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No budgets in {report.month}.{' '}
          <Link to={'/budgets' as string} className="underline">
            Set up budgets
          </Link>
        </p>
      ) : (
        <div className="space-y-3">
          {report.rows.map((r) => {
            const depth = depthIn(r.account_name, names);
            return (
              <div
                key={r.account_id}
                data-testid="budget-row"
                data-depth={depth}
                className="space-y-1"
                style={{ paddingLeft: `${depth * 1.25}rem` }}
              >
                <div className="flex items-baseline justify-between gap-3 text-sm">
                  <span className="flex items-center gap-1 truncate font-medium">
                    {shortName(r.account_name)}
                    {r.excluded_accounts.length > 0 && (
                      <span
                        data-testid="excluded-warning"
                        className="text-amber-600"
                        title={`Not counted (different currency): ${r.excluded_accounts.join(', ')}`}
                      >
                        ⚠
                      </span>
                    )}
                  </span>
                  <span className="tabular-nums text-muted-foreground">
                    {formatCents(r.actual, r.currency)} / {formatCents(r.budget, r.currency)}
                    <span className="ml-2 font-medium text-foreground">
                      {usedLabel(r.budget, r.actual)}
                    </span>
                  </span>
                </div>
                <BudgetProgressBar budget={r.budget} actual={r.actual} elapsed={elapsed} />
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>
                    {regularSubLine(r.actual_regular, r.actual_irregular, r.currency, formatCents)}
                  </span>
                  <span className={r.remaining < 0 ? 'text-red-600' : undefined}>
                    Remaining {formatCents(r.remaining, r.currency)}
                  </span>
                </div>
              </div>
            );
          })}
          <div className="border-t pt-3 text-sm">
            {Object.keys(report.total_budget)
              .sort()
              .map((ccy) => {
                const budget = report.total_budget[ccy];
                const actual = report.total_actual[ccy] ?? 0;
                return (
                  <div key={ccy} className="flex justify-between">
                    <span className="font-medium">Total ({ccy})</span>
                    <span className="tabular-nums">
                      {formatCents(actual, ccy)} / {formatCents(budget, ccy)} · Remaining{' '}
                      {formatCents(budget - actual, ccy)}
                    </span>
                  </div>
                );
              })}
          </div>
        </div>
      )}
    </div>
  );
}
