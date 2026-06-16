import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';

export type KpiVariant = 'green' | 'red' | 'neutral';

interface Props {
  label: string;
  amount: number;
  currency: string;
  variant: KpiVariant;
  subLine?: string;
}

const VARIANT_CLASS: Record<KpiVariant, string> = {
  green: 'text-green-700 dark:text-green-400',
  red: 'text-red-700 dark:text-red-400',
  neutral: 'text-foreground',
};

export function KpiCard({ label, amount, currency, variant, subLine }: Props) {
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
    </div>
  );
}
