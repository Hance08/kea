import { AccountTypeBadge } from '@/components/accounts/AccountTypeBadge';
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
        'grid grid-cols-[1fr_5rem_5rem_7rem] items-center gap-3 border-b border-border/60 px-3 py-1.5 text-sm hover:bg-muted/40',
        account.is_hidden && 'text-muted-foreground',
      )}
    >
      {/* Name column with vertical guide lines + chevron + indent */}
      <div className="flex items-center">
        {Array.from({ length: depth }, (_, i) => `guide-${i}`).map((key) => (
          <span
            key={key}
            aria-hidden="true"
            className="inline-block h-6 w-5 border-l border-border/60"
          />
        ))}
        <div className="flex items-center gap-1 pl-1">
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
            className={cn('truncate hover:underline', hasChildren && 'font-medium')}
          >
            {leafName}
          </Link>
          {account.is_hidden && <span className="ml-2 text-xs uppercase">hidden</span>}
        </div>
      </div>

      {/* Type */}
      <div>
        <AccountTypeBadge type={account.type} />
      </div>

      {/* Currency */}
      <div className="text-xs text-muted-foreground">{account.currency}</div>

      {/* Balance */}
      <div className="text-right tabular-nums">
        {balance ? (
          <span className={cn(balance.amount < 0 && 'text-destructive')}>
            {formatCents(balance.amount, balance.currency)}
          </span>
        ) : (
          <Skeleton className="ml-auto h-4 w-16" />
        )}
      </div>
    </div>
  );
}
