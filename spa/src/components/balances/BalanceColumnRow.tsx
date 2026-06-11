import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  row: AccountBalance;
}

export function BalanceColumnRow({ row }: Props) {
  const negative = row.amount < 0;
  return (
    <Link
      to="/accounts/$id"
      params={{ id: String(row.account_id) }}
      search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
      className="flex items-center justify-between border-t border-border/60 px-3 py-2 text-sm hover:bg-muted/40"
    >
      <span className="truncate">{row.name}</span>
      <span className={cn('tabular-nums', negative && 'text-destructive')}>
        {formatCents(row.amount, row.currency)}
      </span>
    </Link>
  );
}
