import { TransactionRow } from '@/components/transactions/TransactionRow';
import type { TransactionsSearch } from '@/lib/transactions-search-params';
import type { TransactionDetail } from '@/lib/types';

interface Props {
  items: TransactionDetail[];
  search: TransactionsSearch;
}

// Fixed column widths (in px). Account → Offset is the widest because real
// account paths are long ("Assets:Bank:Checking → Expenses:Coffee"); the
// description column is narrower and ellipsis-truncates when it overflows.
// Status is sized to fit "Reconciled" (the longest status) without leaving
// dead space on the right. Total: 110 + 100 + 200 + 340 + 120 + 90 = 960px.
// The wrapper scrolls horizontally on narrower viewports so the layout
// stays predictable.
export const TRANSACTIONS_GRID_COLS = '110px 100px 200px 340px 120px 90px';

export function TransactionsTable({ items, search }: Props) {
  return (
    <div className="overflow-x-auto rounded-md border bg-card">
      <div
        className="grid gap-3 bg-muted/50 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground"
        style={{ gridTemplateColumns: TRANSACTIONS_GRID_COLS }}
      >
        <span className="text-left">Date</span>
        <span className="text-center">Type</span>
        <span className="text-center">Description</span>
        <span className="text-center">Account → Offset</span>
        <span className="text-right">Amount</span>
        <span className="text-right">Status</span>
      </div>
      {items.map((tx) => (
        <TransactionRow key={tx.id} tx={tx} search={search} />
      ))}
    </div>
  );
}
