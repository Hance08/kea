import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  row: AccountBalance;
}

export function AccountListRow({ row }: Props) {
  const negative = row.amount < 0;
  return (
    <Link
      to="/accounts/$id"
      params={{ id: String(row.account_id) }}
      search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
      className="flex items-center justify-between border-b border-border/60 px-2 py-2 text-sm hover:bg-muted/40"
    >
      <span>{row.name}</span>
      <span className={cn('tabular-nums', negative && 'text-destructive')}>
        {formatCents(row.amount, row.currency)}
      </span>
    </Link>
  );
}
