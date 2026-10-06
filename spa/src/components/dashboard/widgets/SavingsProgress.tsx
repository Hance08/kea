import { Link } from '@tanstack/react-router';
import { elapsedFraction } from '../../../lib/budgets';
import { useSavingsReport } from '../../../lib/hooks/useSavings';
import { remainingLabel } from '../../../lib/savings';
import { useAmountFormat } from '../../../lib/server-config';
import { SavingsProgressBar } from '../../savings/SavingsProgressBar';

export function SavingsProgress() {
  // No month: the server resolves the current month in its local time.
  const q = useSavingsReport();
  const { formatCents } = useAmountFormat();
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const report = q.data;
  if (!report || report.rows.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        <span>
          No savings targets yet.{' '}
          <Link to="/savings" className="underline">
            Set up savings targets
          </Link>
        </span>
      </div>
    );
  }

  const elapsed = elapsedFraction(report.month);
  return (
    <Link to="/reports/savings" className="flex h-full flex-col">
      <ul className="flex flex-col gap-2 overflow-auto">
        {report.rows.map((r) => {
          const fmt = (c: number) => formatCents(c, r.currency);
          return (
            <li
              key={r.account_id}
              data-testid="savings-progress-item"
              data-account={r.account_name}
              className="text-xs"
            >
              <div className="flex justify-between gap-2">
                <span className="truncate">{r.account_name}</span>
                <span className="tabular-nums">
                  {fmt(r.saved)} / {fmt(r.target)}
                </span>
              </div>
              <div className="mt-0.5">
                <SavingsProgressBar target={r.target} saved={r.saved} elapsed={elapsed} compact />
              </div>
              <div className="mt-0.5 text-muted-foreground">
                YTD {remainingLabel(r.ytd_remaining, fmt)}
              </div>
            </li>
          );
        })}
      </ul>
    </Link>
  );
}
