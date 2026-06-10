import { TransactionRow } from '@/components/transactions/TransactionRow';
import type { TransactionDetail } from '@/lib/types';

interface Props {
  items: TransactionDetail[];
}

export function TransactionsTable({ items }: Props) {
  return (
    <div className="rounded-md border bg-card">
      <div className="grid grid-cols-[80px_80px_1fr_1fr_120px_90px] gap-3 bg-muted/50 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        <span>Date</span>
        <span>Type</span>
        <span>Description</span>
        <span>Account → Offset</span>
        <span className="text-right">Amount</span>
        <span>Status</span>
      </div>
      {items.map((tx) => (
        <TransactionRow key={tx.id} tx={tx} />
      ))}
    </div>
  );
}
