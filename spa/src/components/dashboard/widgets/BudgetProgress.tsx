import { Link } from '@tanstack/react-router';
import { WARN_PCT, elapsedFraction } from '../../../lib/budgets';
import { useBudgetReport } from '../../../lib/hooks/useBudgets';
import type { BudgetReportRow } from '../../../lib/types';
import { BudgetProgressBar } from '../../budgets/BudgetProgressBar';

export type BudgetProgressConfig = { limit: 5 | 10; onlyWarnings: boolean };
export const BUDGET_PROGRESS_DEFAULT: BudgetProgressConfig = { limit: 5, onlyWarnings: false };

// Sort key: zero budgets with spending sort first (infinitely over).
function ratio(r: BudgetReportRow): number {
  if (r.budget === 0) return r.actual > 0 ? Number.POSITIVE_INFINITY : 0;
  return r.actual / r.budget;
}

function pctLabel(r: BudgetReportRow): string {
  if (r.budget === 0) return r.actual > 0 ? 'over' : '';
  return `${Math.round((r.actual / r.budget) * 100)}%`;
}

export function BudgetProgress({ config }: { config: BudgetProgressConfig }) {
  // No month: the server resolves the current month in its local time.
  const q = useBudgetReport();
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const report = q.data;
  if (!report || report.rows.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        <span>
          No budgets yet.{' '}
          <Link to="/budgets" className="underline">
            Set up budgets
          </Link>
        </span>
      </div>
    );
  }

  const elapsed = elapsedFraction(report.month);
  const items = [...report.rows]
    .filter((r) => !config.onlyWarnings || ratio(r) * 100 >= WARN_PCT)
    .sort((a, b) => ratio(b) - ratio(a))
    .slice(0, config.limit);

  return (
    <Link to="/reports/budget" className="flex h-full flex-col">
      <ul className="flex flex-col gap-2 overflow-auto">
        {items.map((r) => (
          <li
            key={r.account_id}
            data-testid="budget-progress-item"
            data-account={r.account_name}
            className="text-xs"
          >
            <div className="flex justify-between">
              <span className="truncate">{r.account_name}</span>
              <span className="tabular-nums">{pctLabel(r)}</span>
            </div>
            <div className="mt-0.5">
              <BudgetProgressBar budget={r.budget} actual={r.actual} elapsed={elapsed} compact />
            </div>
          </li>
        ))}
        {items.length === 0 && (
          <li className="text-xs text-muted-foreground">All budgets under {WARN_PCT}%</li>
        )}
      </ul>
    </Link>
  );
}

export function BudgetProgressConfigForm({
  config,
  onChange,
}: { config: BudgetProgressConfig; onChange: (c: BudgetProgressConfig) => void }) {
  return (
    <div className="flex flex-col gap-2 text-sm">
      <label className="flex flex-col gap-1">
        <span className="text-xs font-medium">Number of budgets</span>
        <select
          value={config.limit}
          onChange={(e) =>
            onChange({ ...config, limit: Number(e.target.value) as BudgetProgressConfig['limit'] })
          }
          className="rounded border px-2 py-1 text-sm"
        >
          <option value={5}>5</option>
          <option value={10}>10</option>
        </select>
      </label>
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={config.onlyWarnings}
          onChange={(e) => onChange({ ...config, onlyWarnings: e.target.checked })}
        />
        <span className="text-xs">Only show budgets at {WARN_PCT}% or more</span>
      </label>
    </div>
  );
}
