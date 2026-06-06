import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';

interface Props {
  row: AccountBalance;
}

export function AccountListRow({ row }: Props) {
  const negative = row.amount < 0;
  return (
    <div className="flex items-center justify-between border-b border-border/60 px-2 py-2 text-sm">
      <span>{row.name}</span>
      <span className={cn('tabular-nums', negative && 'text-destructive')}>
        {formatCents(row.amount, row.currency)}
      </span>
    </div>
  );
}
