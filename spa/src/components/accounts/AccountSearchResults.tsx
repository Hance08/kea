import { AccountTypeBadge } from '@/components/accounts/AccountTypeBadge';
import { cn } from '@/lib/cn';
import type { Account, AccountBalance } from '@/lib/types';
import { Link } from '@tanstack/react-router';

// Balance column shows just `$X,XXX.XX` (no currency code — that's in
// its own column) and conveys sign via color: negative red, positive
// green, zero unstyled. The `-` prefix is dropped from negatives.
function formatBalance(cents: number): string {
  const abs = Math.abs(cents / 100);
  return `$${abs.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

interface Props {
  accounts: Account[];
  balances?: AccountBalance[];
  totalCount: number;
}

export function AccountSearchResults({ accounts, balances, totalCount }: Props) {
  const balanceById = new Map<number, { amount: number; currency: string }>();
  if (balances) {
    for (const b of balances) {
      balanceById.set(b.account_id, { amount: b.amount, currency: b.currency });
    }
  }
  return (
    <div className="space-y-2">
      {totalCount > accounts.length && (
        <div className="rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
          Refine search — showing first {accounts.length} of {totalCount} matches.
        </div>
      )}
      <div className="overflow-hidden rounded-md border bg-card">
        <table className="w-full table-fixed text-sm">
          <colgroup>
            <col />
            <col className="w-28" />
            <col className="w-24" />
            <col className="w-32" />
          </colgroup>
          <thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
            <tr>
              <th className="px-3 py-2 text-left">Name</th>
              <th className="px-3 py-2 text-center">Type</th>
              <th className="px-3 py-2 text-center">Currency</th>
              <th className="px-3 py-2 text-right">Balance</th>
            </tr>
          </thead>
          <tbody>
            {accounts.map((acc) => {
              const bal = balanceById.get(acc.id);
              return (
                <tr key={acc.id} className="border-t hover:bg-muted/40">
                  <td className="px-3 py-2">
                    <Link
                      to="/accounts/$id"
                      params={{ id: String(acc.id) }}
                      search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
                      className="block truncate hover:underline"
                    >
                      {acc.name}
                    </Link>
                  </td>
                  <td className="px-3 py-2 text-center">
                    <AccountTypeBadge type={acc.type} />
                  </td>
                  <td className="px-3 py-2 text-center">{acc.currency}</td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {bal ? (
                      <span
                        className={cn(
                          bal.amount < 0 ? 'text-red-600' : bal.amount > 0 ? 'text-green-600' : '',
                        )}
                      >
                        {formatBalance(bal.amount)}
                      </span>
                    ) : (
                      '—'
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
