import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { AccountBalance, AccountNode } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  accounts: AccountNode[];
  balances?: AccountBalance[];
}

export function ChildAccountsCard({ accounts, balances }: Props) {
  const balanceById = new Map<number, { amount: number; currency: string }>();
  if (balances) {
    for (const b of balances) {
      balanceById.set(b.account_id, { amount: b.amount, currency: b.currency });
    }
  }
  return (
    <div className="rounded-md border bg-card">
      <div className="border-b bg-muted/40 px-3 py-2 text-xs font-semibold uppercase text-muted-foreground">
        Child accounts
      </div>
      {accounts.length === 0 && (
        <div className="px-3 py-4 text-sm text-muted-foreground">No child accounts.</div>
      )}
      {accounts.map((node) => {
        const acc = node.account;
        const bal = balanceById.get(acc.id);
        const leafName = acc.name.split(':').pop() ?? acc.name;
        return (
          <div
            key={acc.id}
            className="flex items-center justify-between border-b border-border/60 px-3 py-2 text-sm last:border-b-0 hover:bg-muted/40"
          >
            <Link
              to="/accounts/$id"
              params={{ id: String(acc.id) }}
              search={{ include_hidden: false }}
              className="hover:underline"
            >
              {leafName}
            </Link>
            {bal && (
              <span className={cn('tabular-nums', bal.amount < 0 && 'text-destructive')}>
                {formatCents(bal.amount, bal.currency)}
              </span>
            )}
          </div>
        );
      })}
    </div>
  );
}
