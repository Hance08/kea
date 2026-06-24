import { cn } from '@/lib/cn';
import type { DailyBalancePoint } from '@/lib/types';

export type ChartRange = '1M' | '3M' | 'YTD' | '1Y' | 'ALL';

const OPTIONS: { value: ChartRange; label: string }[] = [
  { value: '1M', label: '1M' },
  { value: '3M', label: '3M' },
  { value: 'YTD', label: 'YTD' },
  { value: '1Y', label: '1Y' },
  { value: 'ALL', label: 'All' },
];

interface Props {
  value: ChartRange;
  onChange: (range: ChartRange) => void;
  className?: string;
}

export function ChartRangeSelector({ value, onChange, className }: Props) {
  return (
    <div role="radiogroup" aria-label="Chart range" className={cn('flex gap-0.5', className)}>
      {OPTIONS.map((opt) => {
        const isActive = value === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            role="radio"
            aria-checked={isActive}
            onClick={() => onChange(opt.value)}
            className={cn(
              'rounded-md px-2 py-1 text-xs font-medium transition-colors',
              isActive
                ? 'bg-primary text-primary-foreground'
                : 'text-muted-foreground hover:bg-accent hover:text-foreground',
            )}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}

export function filterPointsByRange(
  points: DailyBalancePoint[],
  range: ChartRange,
): DailyBalancePoint[] {
  if (range === 'ALL' || points.length === 0) return points;
  const last = points[points.length - 1].date;
  const anchor = new Date(`${last}T00:00:00Z`);
  let cutoff: Date;
  if (range === 'YTD') {
    cutoff = new Date(Date.UTC(anchor.getUTCFullYear(), 0, 1));
  } else {
    const months = range === '1M' ? 1 : range === '3M' ? 3 : 12;
    cutoff = new Date(anchor);
    cutoff.setUTCMonth(cutoff.getUTCMonth() - months);
  }
  const cutoffStr = cutoff.toISOString().slice(0, 10);
  return points.filter((p) => p.date >= cutoffStr);
}
