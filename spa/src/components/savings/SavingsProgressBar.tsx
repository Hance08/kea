import { cn } from '@/lib/cn';
import { savingsStatus } from '@/lib/savings';

interface Props {
  target: number;
  saved: number;
  elapsed?: number | null; // 0..1 for the current month; null/undefined = month complete
  compact?: boolean;
}

const FILL = {
  achieved: 'bg-emerald-600',
  'on-track': 'bg-primary',
  behind: 'bg-amber-500',
  negative: 'bg-red-600',
} as const;

export function SavingsProgressBar({ target, saved, elapsed, compact }: Props) {
  const status = savingsStatus(target, saved, elapsed ?? null);
  const ratio = target === 0 ? (saved >= 0 ? 1 : 0) : Math.max(0, saved / target);
  return (
    <div
      data-testid="savings-bar"
      data-status={status}
      className={cn('relative w-full rounded bg-muted', compact ? 'h-1.5' : 'h-2.5')}
    >
      <div
        className={cn('h-full rounded', FILL[status])}
        style={{ width: `${Math.min(ratio, 1) * 100}%` }}
      />
      {elapsed != null && (
        <div
          data-testid="time-marker"
          title="Time elapsed this month"
          className="absolute -top-0.5 h-[calc(100%+4px)] w-px bg-foreground/70"
          style={{ left: `${elapsed * 100}%` }}
        />
      )}
    </div>
  );
}
