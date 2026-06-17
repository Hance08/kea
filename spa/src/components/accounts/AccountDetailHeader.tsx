import { Button } from '@/components/ui/button';
import { cn } from '@/lib/cn';
import { useAmountFormat } from '@/lib/server-config';
import type { Account, BalanceResponse } from '@/lib/types';
import { Link } from '@tanstack/react-router';
import type { ReactNode } from 'react';

interface Props {
  account: Account;
  balance?: BalanceResponse;
  deleteSlot: ReactNode;
}

const TYPE_LABEL: Record<Account['type'], string> = {
  A: 'Asset',
  L: 'Liability',
  C: 'Equity',
  R: 'Revenue',
  E: 'Expense',
};

export function AccountDetailHeader({ account, balance, deleteSlot }: Props) {
  const { formatCents } = useAmountFormat();
  return (
    <div className="mb-4 rounded-md border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">{account.name}</h1>
          <div className="mt-1 text-sm text-muted-foreground">
            {TYPE_LABEL[account.type]} · {account.currency}
            {balance && (
              <>
                {' · '}
                <span className={cn(balance.amount < 0 && 'text-destructive', 'tabular-nums')}>
                  {formatCents(balance.amount, balance.currency)}
                </span>
              </>
            )}
            {account.is_hidden && (
              <span className="ml-2 rounded bg-muted px-1.5 py-0.5 text-xs uppercase">hidden</span>
            )}
          </div>
          {account.description && (
            <p className="mt-2 text-sm text-muted-foreground">{account.description}</p>
          )}
        </div>
        <div className="flex gap-2">
          <Button asChild size="sm" variant="outline">
            <Link
              to="/accounts/$id/edit"
              params={{ id: String(account.id) }}
              search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
            >
              Edit
            </Link>
          </Button>
          {deleteSlot}
        </div>
      </div>
    </div>
  );
}
