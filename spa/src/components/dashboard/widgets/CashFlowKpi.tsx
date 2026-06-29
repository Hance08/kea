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
  const { formatAmount } = useAmountFormat();
  const cur = useIncomeStatement({ month: thisMonthStr() });
  const prev = useIncomeStatement({ month: lastMonthStr() });

  if (cur.isLoading || prev.isLoading)
    return <div className="h-full animate-pulse rounded bg-muted" />;
  if (cur.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const a = totals(cur.data, currency);
  const net = a.income - a.expense;

  return (
    <div className="flex h-full flex-col justify-center">
      <div className="grid grid-cols-3 gap-2 text-xs">
        <Cell label="Income" value={formatAmount(a.income)} tone="pos" />
        <Cell label="Expense" value={formatAmount(a.expense)} tone="neg" />
        <Cell label="Net" value={formatAmount(net)} tone={net >= 0 ? 'pos' : 'neg'} />
      </div>
    </div>
  );
}

export function CashFlowKpiHeader() {
  const currency = useServerConfig().defaults.currency;
  const { formatAmount } = useAmountFormat();
  const cur = useIncomeStatement({ month: thisMonthStr() });
  const prev = useIncomeStatement({ month: lastMonthStr() });
  if (cur.isLoading || prev.isLoading || cur.isError || prev.isError) return null;
  const a = totals(cur.data, currency);
  const b = totals(prev.data, currency);
  const netDelta = a.income - a.expense - (b.income - b.expense);
  return (
    <span className={netDelta >= 0 ? 'text-emerald-600' : 'text-rose-600'}>
      {netDelta >= 0 ? '▲' : '▼'} {formatAmount(Math.abs(netDelta))}
    </span>
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
