import { StatusText } from '@/components/transactions/StatusText';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import { displayAccount, displayAmount, displayOffsetAccount } from '@/lib/transactionDisplay';
import type { TransactionsSearch } from '@/lib/transactions-search-params';
import type { TransactionDetail } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  tx: TransactionDetail;
  search: TransactionsSearch;
}

function fmtDate(unix: number): string {
  const d = new Date(unix * 1000);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

export function TransactionRow({ tx, search }: Props) {
  const acc = displayAccount(tx.splits, tx.type);
  const offset = displayOffsetAccount(tx.splits, tx.type, acc);
  const { amount, currency } = displayAmount(tx.splits, tx.type);
  const signClass = amount < 0 ? 'text-red-600' : amount > 0 ? 'text-green-600' : '';

  return (
    <Link
      to="/transactions/$id"
      params={{ id: String(tx.id) }}
      search={search}
      className="grid grid-cols-[80px_80px_1fr_1fr_120px_90px] items-center gap-3 border-t px-3 py-2 text-sm hover:bg-muted/50"
    >
      <span className="text-muted-foreground">{fmtDate(tx.timestamp)}</span>
      <TypeBadge type={tx.type} />
      <span className="truncate" title={tx.description}>
        {tx.description}
      </span>
      <span className="truncate text-muted-foreground">
        {acc} → {offset}
      </span>
      <span className={cn('text-right tabular-nums', signClass)}>
        {formatCents(amount, currency)}
      </span>
      <StatusText status={tx.status} />
    </Link>
  );
}
