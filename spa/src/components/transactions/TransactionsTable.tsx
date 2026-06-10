import { TransactionRow } from '@/components/transactions/TransactionRow';
import type { TransactionsSearch } from '@/lib/transactions-search-params';
import type { TransactionDetail } from '@/lib/types';

interface Props {
  items: TransactionDetail[];
  search: TransactionsSearch;
}

// Fixed widths for every column except Account → Offset, which uses
// minmax(340px, 1fr) so it absorbs any extra width at wider viewports
// (real account paths are long and benefit from the room) and stays at
// 340px minimum on narrow viewports. Description is narrower and
// ellipsis-truncates when it overflows.
// Min total: 110 + 100 + 200 + 340 + 120 + 90 = 960px. The wrapper
// scrolls horizontally below that.
export const TRANSACTIONS_GRID_COLS = '110px 100px 200px minmax(340px, 1fr) 120px 90px';

export function TransactionsTable({ items, search }: Props) {
  return (
    // overflow-x-auto preserves horizontal scrolling when the viewport
    // is narrower than the grid's minimum (~1020px). At wider viewports
    // the Account → Offset column (minmax(340px, 1fr)) absorbs the
    // extra width so the table fills its parent container.
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
