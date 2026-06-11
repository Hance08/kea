import { cn } from '@/lib/cn';
import type { AccountType } from '@/lib/types';

const TYPE_CLASSES: Record<AccountType, string> = {
  A: 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200',
  L: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200',
  C: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200',
  R: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200',
  E: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200',
};

const TYPE_LABEL: Record<AccountType, string> = {
  A: 'Asset',
  L: 'Liability',
  C: 'Equity',
  R: 'Revenue',
  E: 'Expense',
};

interface Props {
  type: AccountType;
  className?: string;
}

export function AccountTypeBadge({ type, className }: Props) {
  return (
    <span
      className={cn(
        'inline-flex min-w-[4.5rem] items-center justify-center rounded px-1.5 py-0.5 text-xs font-medium',
        TYPE_CLASSES[type],
        className,
      )}
    >
      {TYPE_LABEL[type]}
    </span>
  );
}
