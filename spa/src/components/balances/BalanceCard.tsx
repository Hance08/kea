import { balanceColor } from '@/lib/accounts';
import { cn } from '@/lib/cn';
import { formatBalanceAbs } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  row: AccountBalance;
  columnLabel: 'Assets' | 'Liabilities';
  share: number | null;
}

// Strip the canonical column-type prefix when present; leave non-canonical
// names unchanged so users with quirky ledger naming see what they typed.
function stripColumnPrefix(name: string, columnLabel: 'Assets' | 'Liabilities'): string {
  const prefix = `${columnLabel}:`;
  return name.startsWith(prefix) ? name.slice(prefix.length) : name;
}

export function BalanceCard({ row, columnLabel, share }: Props) {
  const displayName = stripColumnPrefix(row.name, columnLabel);
  const shareWording = columnLabel.toLowerCase(); // 'assets' | 'liabilities'

  return (
    <Link
      to="/accounts/$id"
      params={{ id: String(row.account_id) }}
      search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
      className="flex h-full flex-col justify-between rounded-md border bg-card p-3 text-sm hover:bg-muted/40"
    >
      <div className="flex items-start justify-between gap-2">
        <span className="truncate text-xs text-muted-foreground" title={row.name}>
          {displayName}
        </span>
        <span className="shrink-0 rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700">
          {row.currency}
        </span>
      </div>
      <div className="mt-2">
        <div className={cn('text-lg font-bold tabular-nums', balanceColor(row.type, row.amount))}>
          {formatBalanceAbs(row.amount)}
        </div>
        {share !== null && (
          <div className="mt-0.5 text-[11px] text-muted-foreground">
            {share}% of {shareWording}
          </div>
        )}
      </div>
    </Link>
  );
}
