import { cn } from '@/lib/cn';
import { useAmountFormat } from '@/lib/server-config';

export type KpiVariant = 'green' | 'red' | 'neutral';

export interface KpiDiff {
  delta: number;
  prevAmount: number;
  goodWhen: 'up' | 'down';
}

interface Props {
  label: string;
  amount: number;
  currency: string;
  variant: KpiVariant;
  subLine?: string;
  diff?: KpiDiff;
}

const VARIANT_CLASS: Record<KpiVariant, string> = {
  green: 'text-green-700 dark:text-green-400',
  red: 'text-red-700 dark:text-red-400',
  neutral: 'text-foreground',
};

const GOOD_CLASS = 'text-green-700 dark:text-green-400';
const BAD_CLASS = 'text-red-700 dark:text-red-400';
const NEUTRAL_DIFF_CLASS = 'text-muted-foreground';

export function KpiCard({ label, amount, currency, variant, subLine, diff }: Props) {
  const { formatCents } = useAmountFormat();

  const formatDiff = (d: KpiDiff): { text: string; className: string } => {
    if (d.delta === 0) {
      return { text: '— no change vs last period', className: NEUTRAL_DIFF_CLASS };
    }
    const arrow = d.delta > 0 ? '▲' : '▼';
    const sign = d.delta > 0 ? '+' : '-';
    const absAmount = formatCents(Math.abs(d.delta), currency);
    let pctPart = '';
    if (d.prevAmount !== 0) {
      const pct = (d.delta / Math.abs(d.prevAmount)) * 100;
      const pctSign = pct > 0 ? '+' : pct < 0 ? '-' : '';
      pctPart = ` (${pctSign}${Math.abs(pct).toFixed(1)}%)`;
    }
    const isGood = (d.delta > 0 && d.goodWhen === 'up') || (d.delta < 0 && d.goodWhen === 'down');
    return {
      text: `${arrow} ${sign}${absAmount}${pctPart} vs last period`,
      className: isGood ? GOOD_CLASS : BAD_CLASS,
    };
  };

  const diffRendered = diff ? formatDiff(diff) : null;
  return (
    <div className="rounded-md border bg-card p-4">
      <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
      <div
        data-testid="kpi-amount"
        className={cn('mt-1 text-2xl font-semibold', VARIANT_CLASS[variant])}
      >
        {formatCents(amount, currency)}
      </div>
      {subLine && (
        <div data-testid="kpi-subline" className="mt-1 text-xs text-muted-foreground">
          {subLine}
        </div>
      )}
      {diffRendered && (
        <div data-testid="kpi-diff" className={cn('mt-1 text-xs', diffRendered.className)}>
          {diffRendered.text}
        </div>
      )}
    </div>
  );
}
