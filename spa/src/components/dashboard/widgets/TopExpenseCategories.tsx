import { useExpenseBreakdown } from '../../../lib/hooks/useReport';
import { useAmountFormat, useServerConfig } from '../../../lib/server-config';
import type { ReportResult } from '../../../lib/types';

export type TopExpenseConfig = { limit: 5 | 10 };
export const TOP_EXPENSE_DEFAULT: TopExpenseConfig = { limit: 5 };

function thisMonthStr(): string {
  const d = new Date();
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`;
}

function topCategories(report: ReportResult | undefined, currency: string, limit: number) {
  return (report?.expense_rows ?? [])
    .filter((r) => r.currency === currency && r.amount > 0)
    .map((r) => ({ name: r.account_name, amount: r.amount }))
    .sort((a, b) => b.amount - a.amount)
    .slice(0, limit);
}

export function TopExpenseCategories({ config }: { config: TopExpenseConfig }) {
  const currency = useServerConfig().defaults.currency;
  const { formatAmount } = useAmountFormat();
  const q = useExpenseBreakdown({ month: thisMonthStr() });
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;
  const items = topCategories(q.data, currency, config.limit);
  if (items.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        No expenses this month
      </div>
    );
  }
  const max = items[0].amount;
  return (
    <div className="flex h-full flex-col">
      <ul className="flex flex-col gap-1.5 overflow-auto">
        {items.map((it) => (
          <li key={it.name} className="text-xs">
            <div className="flex justify-between">
              <span className="truncate">{it.name}</span>
              <span className="tabular-nums">{formatAmount(it.amount)}</span>
            </div>
            <div className="mt-0.5 h-1.5 rounded bg-muted">
              <div
                className="h-full rounded bg-primary"
                style={{ width: `${(it.amount / max) * 100}%` }}
              />
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function TopExpenseConfigForm({
  config,
  onChange,
}: { config: TopExpenseConfig; onChange: (c: TopExpenseConfig) => void }) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-xs font-medium">Number of categories</span>
      <select
        value={config.limit}
        onChange={(e) => onChange({ limit: Number(e.target.value) as TopExpenseConfig['limit'] })}
        className="rounded border px-2 py-1 text-sm"
      >
        <option value={5}>5</option>
        <option value={10}>10</option>
      </select>
    </label>
  );
}
