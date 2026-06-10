import { cn } from '@/lib/cn';
import type { AccountType } from '@/lib/types';

interface Props {
  value: AccountType | '';
  onChange: (value: AccountType) => void;
  disabled?: boolean;
}

const OPTIONS: { value: AccountType; label: string }[] = [
  { value: 'A', label: 'Asset' },
  { value: 'L', label: 'Liability' },
  { value: 'C', label: 'Equity' },
  { value: 'R', label: 'Revenue' },
  { value: 'E', label: 'Expense' },
];

export function TypeSelect({ value, onChange, disabled }: Props) {
  return (
    <div className="flex flex-wrap gap-2">
      {OPTIONS.map((opt) => (
        <button
          key={opt.value}
          type="button"
          disabled={disabled}
          onClick={() => onChange(opt.value)}
          className={cn(
            'rounded-md border px-3 py-1.5 text-sm transition-colors',
            value === opt.value
              ? 'border-primary bg-primary text-primary-foreground'
              : 'hover:bg-muted',
            disabled && 'cursor-not-allowed opacity-50',
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
