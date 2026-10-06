import { MonthPicker } from '@/components/budgets/MonthPicker';
import { SavingsProgressBar } from '@/components/savings/SavingsProgressBar';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { depthIn, elapsedFraction } from '@/lib/budgets';
import { cn } from '@/lib/cn';
import { makeFilterMemoryLoader } from '@/lib/filter-memory';
import { useSavingsReport } from '@/lib/hooks/useSavings';
import { type MonthSearchParams, parseMonthSearch } from '@/lib/reports-search-params';
import { remainingLabel } from '@/lib/savings';
import { useAmountFormat } from '@/lib/server-config';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/reports/savings')({
  validateSearch: (s): MonthSearchParams => parseMonthSearch(s),
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<MonthSearchParams>({
    pageId: 'reports/savings',
    defaults: {},
    redirectTo: '/reports/savings',
  }),
  component: SavingsReportPage,
});

const shortName = (name: string) => name.split(':').slice(1).join(':') || name;

function SavingsReportPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/reports/savings' });
  const query = useSavingsReport(search.month);
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
          <AlertTitle>Failed to load savings report</AlertTitle>
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
    <div className="space-y-4" data-testid="savings-report">
      {picker}
      {report.rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No savings targets in {report.month}.{' '}
          <Link to="/savings" className="underline">
            Set up savings targets
          </Link>
        </p>
      ) : (
        <div className="space-y-4">
          {report.rows.map((r) => {
            const depth = depthIn(r.account_name, names);
            const fmt = (c: number) => formatCents(c, r.currency);
            return (
              <div
                key={r.account_id}
                data-testid="savings-row"
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
                    {fmt(r.saved)} / {fmt(r.target)}
                  </span>
                </div>
                <SavingsProgressBar target={r.target} saved={r.saved} elapsed={elapsed} />
                <div className="flex flex-wrap justify-between gap-2 text-xs text-muted-foreground">
                  <span data-testid="savings-remaining">{remainingLabel(r.remaining, fmt)}</span>
                  <span data-testid="savings-ytd">
                    YTD {fmt(r.ytd_saved)} / {fmt(r.ytd_target)} ·{' '}
                    {remainingLabel(r.ytd_remaining, fmt)}
                  </span>
                </div>
                {r.months.length > 0 && (
                  <ul className="grid grid-cols-3 gap-x-4 gap-y-0.5 text-xs sm:grid-cols-6">
                    {r.months.map((m) => {
                      const met = m.saved >= m.target;
                      return (
                        <li
                          key={m.month}
                          data-testid="savings-month"
                          data-met={String(met)}
                          className="flex justify-between gap-2 tabular-nums"
                        >
                          <span className="text-muted-foreground">{m.month.slice(5)}</span>
                          <span className={cn(met ? 'text-emerald-600' : 'text-amber-600')}>
                            {fmt(m.saved)}
                          </span>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </div>
            );
          })}
          <div className="border-t pt-3 text-sm">
            {Object.keys(report.total_target)
              .sort()
              .map((ccy) => {
                const fmt = (c: number) => formatCents(c, ccy);
                const target = report.total_target[ccy];
                const saved = report.total_saved[ccy] ?? 0;
                const ytdTarget = report.total_ytd_target[ccy] ?? 0;
                const ytdSaved = report.total_ytd_saved[ccy] ?? 0;
                return (
                  <div key={ccy} className="flex flex-wrap justify-between gap-2">
                    <span className="font-medium">Total ({ccy})</span>
                    <span className="tabular-nums">
                      {fmt(saved)} / {fmt(target)} · {remainingLabel(target - saved, fmt)} · YTD{' '}
                      {fmt(ytdSaved)} / {fmt(ytdTarget)}
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
