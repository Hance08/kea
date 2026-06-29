import { useIncomeStatement } from '../../../lib/hooks/useReport';
import { useAmountFormat, useServerConfig } from '../../../lib/server-config';
import type { ReportResult } from '../../../lib/types';

function thisMonthStr(): string {
  const d = new Date();
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`;
}
function lastMonthStr(): string {
  const d = new Date();
  d.setUTCDate(1);
  d.setUTCMonth(d.getUTCMonth() - 1);
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`;
}

function totals(report: ReportResult | undefined, currency: string) {
  return {
    income: report?.total_income[currency] ?? 0,
    expense: report?.total_expense[currency] ?? 0,
  };
}

export function CashFlowKpi() {
  const currency = useServerConfig().defaults.currency;
  const { formatCents } = useAmountFormat();
  const cur = useIncomeStatement({ month: thisMonthStr() });
  const prev = useIncomeStatement({ month: lastMonthStr() });

  if (cur.isLoading || prev.isLoading)
    return <div className="h-full animate-pulse rounded bg-muted" />;
  if (cur.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const a = totals(cur.data, currency);
  const b = totals(prev.data, currency);
  const net = a.income - a.expense;
  const prevNet = b.income - b.expense;
  const netDelta = net - prevNet;

  return (
    <div className="flex h-full flex-col justify-center">
      <div className="grid grid-cols-3 gap-2 text-xs">
        <Cell label="Income" value={formatCents(a.income, currency)} tone="pos" />
        <Cell label="Expense" value={formatCents(a.expense, currency)} tone="neg" />
        <Cell label="Net" value={formatCents(net, currency)} tone={net >= 0 ? 'pos' : 'neg'} />
      </div>
      <div className={`mt-1 text-xs ${netDelta >= 0 ? 'text-emerald-600' : 'text-rose-600'}`}>
        {netDelta >= 0 ? '▲' : '▼'} {formatCents(Math.abs(netDelta), currency)} vs last month
      </div>
    </div>
  );
}

function Cell({ label, value, tone }: { label: string; value: string; tone: 'pos' | 'neg' }) {
  return (
    <div>
      <div className="text-muted-foreground">{label}</div>
      <div className={`tabular-nums ${tone === 'pos' ? 'text-emerald-600' : 'text-rose-600'}`}>
        {value}
      </div>
    </div>
  );
}
