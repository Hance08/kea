import { StatusText } from '@/components/transactions/StatusText';
import { TRANSACTIONS_GRID_COLS } from '@/components/transactions/TransactionsTable';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { cn } from '@/lib/cn';
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

function formatDollars(cents: number): string {
  return `$${(cents / 100).toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

export function TransactionRow({ tx, search }: Props) {
  const acc = displayAccount(tx.splits, tx.type);
  const offset = displayOffsetAccount(tx.splits, tx.type, acc);
  const { amount } = displayAmount(tx.splits, tx.type);
  // Color reflects sign, but the rendered amount is always positive (the
  // negative sign is implied by the red color, per the design call).
  const signClass = amount < 0 ? 'text-red-600' : amount > 0 ? 'text-green-600' : '';
  const absAmount = Math.abs(amount);

  return (
    <Link
      to="/transactions/$id"
      params={{ id: String(tx.id) }}
      search={search}
      className="grid items-center gap-3 border-t px-3 py-2 text-sm hover:bg-muted/50"
      style={{ gridTemplateColumns: TRANSACTIONS_GRID_COLS }}
    >
      <span className="text-left text-muted-foreground">{fmtDate(tx.timestamp)}</span>
      <span className="flex justify-center">
        <TypeBadge type={tx.type} />
      </span>
      <span className="truncate text-center" title={tx.description}>
        {tx.description}
      </span>
      <span className="truncate text-center text-muted-foreground" title={`${acc} → ${offset}`}>
        {acc} → {offset}
      </span>
      <span className={cn('text-right tabular-nums', signClass)}>{formatDollars(absAmount)}</span>
      <span className="text-right">
        <StatusText status={tx.status} />
      </span>
    </Link>
  );
}
