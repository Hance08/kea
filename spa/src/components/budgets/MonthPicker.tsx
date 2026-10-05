import { Button } from '@/components/ui/button';
import { currentMonth, shiftMonth } from '@/lib/budgets';

interface Props {
  value?: string; // undefined = current month
  onChange: (month: string | undefined) => void;
}

export function MonthPicker({ value, onChange }: Props) {
  const month = value ?? currentMonth();
  const isCurrent = month === currentMonth();
  const go = (m: string) => onChange(m === currentMonth() ? undefined : m);
  return (
    <div className="flex items-center gap-2">
      <Button
        variant="outline"
        size="sm"
        aria-label="Previous month"
        onClick={() => go(shiftMonth(month, -1))}
      >
        ‹
      </Button>
      <span className="min-w-[5.5rem] text-center text-sm font-medium tabular-nums">{month}</span>
      <Button
        variant="outline"
        size="sm"
        aria-label="Next month"
        onClick={() => go(shiftMonth(month, 1))}
      >
        ›
      </Button>
      {!isCurrent && (
        <Button variant="ghost" size="sm" onClick={() => onChange(undefined)}>
          This month
        </Button>
      )}
    </div>
  );
}
