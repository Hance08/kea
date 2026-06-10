import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { Account } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  account: Account;
  depth: number;
  hasChildren: boolean;
  expanded: boolean;
  onToggle: () => void;
  balance?: { amount: number; currency: string };
}

export function AccountTreeNode({
  account,
  depth,
  hasChildren,
  expanded,
  onToggle,
  balance,
}: Props) {
  const leafName = account.name.split(':').pop() ?? account.name;
  return (
    <div
      className={cn(
        'flex items-center justify-between border-b border-border/60 px-2 py-1.5 text-sm hover:bg-muted/40',
        account.is_hidden && 'text-muted-foreground',
      )}
      style={{ paddingLeft: `${0.5 + depth * 1.25}rem` }}
    >
      <div className="flex items-center gap-1">
        {hasChildren ? (
          <button
            type="button"
            aria-label={expanded ? 'Collapse' : 'Expand'}
            aria-expanded={expanded}
            onClick={onToggle}
            className="inline-flex h-4 w-4 items-center justify-center text-xs text-muted-foreground"
          >
            {expanded ? '▾' : '▸'}
          </button>
        ) : (
          <span className="inline-block h-4 w-4" aria-hidden="true" />
        )}
        <Link
          to="/accounts/$id"
          params={{ id: String(account.id) }}
          search={{ include_hidden: false }}
          className="hover:underline"
        >
          {leafName}
        </Link>
        {account.is_hidden && <span className="ml-2 text-xs uppercase">hidden</span>}
      </div>
      <div className="tabular-nums">
        {balance ? (
          <span className={cn(balance.amount < 0 && 'text-destructive')}>
            {formatCents(balance.amount, balance.currency)}
          </span>
        ) : (
          <Skeleton className="h-4 w-16" />
        )}
      </div>
    </div>
  );
}
