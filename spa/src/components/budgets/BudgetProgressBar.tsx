import { budgetStatus } from '@/lib/budgets';
import { cn } from '@/lib/cn';

interface Props {
  budget: number;
  actual: number;
  elapsed?: number | null; // 0..1, shown as a marker for the current month
  compact?: boolean;
}

const FILL = { ok: 'bg-primary', warning: 'bg-amber-500', over: 'bg-red-600' } as const;

export function BudgetProgressBar({ budget, actual, elapsed, compact }: Props) {
  const status = budgetStatus(budget, actual);
  const ratio = budget === 0 ? (actual > 0 ? 1 : 0) : Math.max(0, actual / budget);
  return (
    <div
      data-testid="budget-bar"
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
