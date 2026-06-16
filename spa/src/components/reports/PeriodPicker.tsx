import { Button } from '@/components/ui/button';
import { cn } from '@/lib/cn';
import type { PeriodRange } from '@/lib/period';
import type { PeriodSearchParams } from '@/lib/reports-search-params';

const CHIPS: { range: PeriodRange; label: string }[] = [
  { range: 'this-month', label: 'This month' },
  { range: 'last-month', label: 'Last month' },
  { range: 'ytd', label: 'YTD' },
  { range: 'last-12mo', label: 'Last 12 months' },
  { range: 'custom', label: 'Custom…' },
];

interface Props {
  value: PeriodSearchParams;
  onChange: (next: Partial<PeriodSearchParams>) => void;
  label: string;
}

export function PeriodPicker({ value, onChange, label }: Props) {
  return (
    <div className="mb-4 space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="mr-2 text-sm font-medium">{label}</span>
        {CHIPS.map((chip) => {
          const active = value.range === chip.range;
          return (
            <Button
              key={chip.range}
              size="sm"
              variant={active ? 'default' : 'outline'}
              aria-pressed={active}
              onClick={() => onChange({ range: chip.range })}
              className={cn(!active && 'text-muted-foreground')}
            >
              {chip.label}
            </Button>
          );
        })}
      </div>
      {value.range === 'custom' && (
        <div className="flex items-center gap-2 text-sm">
          <label className="flex items-center gap-1">
            <span>From</span>
            <input
              type="date"
              className="rounded border bg-background px-2 py-1"
              value={value.from ?? ''}
              onChange={(e) => onChange({ range: 'custom', from: e.target.value, to: value.to })}
            />
          </label>
          <label className="flex items-center gap-1">
            <span>To</span>
            <input
              type="date"
              className="rounded border bg-background px-2 py-1"
              value={value.to ?? ''}
              onChange={(e) =>
                onChange({ range: 'custom', from: value.from, to: e.target.value })
              }
            />
          </label>
        </div>
      )}
    </div>
  );
}
