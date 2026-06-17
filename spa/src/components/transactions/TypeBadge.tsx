import { cn } from '@/lib/cn';
import type { TransactionType } from '@/lib/types';

const TYPE_CLASSES: Record<TransactionType, string> = {
  Expense: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200',
  Income: 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200',
  Transfer: 'bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-200',
  Opening: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200',
  Deposit: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200',
  Withdrawal: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200',
  Other: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200',
  Investment: 'bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-200',
};

interface Props {
  type: TransactionType;
  className?: string;
}

export function TypeBadge({ type, className }: Props) {
  return (
    <span
      // min-w-[5.5rem] ensures every badge takes the same horizontal
      // space regardless of label length (e.g., Income vs Withdrawal);
      // justify-center centers the label within that fixed width.
      className={cn(
        'inline-flex min-w-[5.5rem] items-center justify-center rounded px-1.5 py-0.5 text-xs font-medium',
        TYPE_CLASSES[type],
        className,
      )}
    >
      {type}
    </span>
  );
}
