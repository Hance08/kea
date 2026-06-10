import { TransactionRow } from '@/components/transactions/TransactionRow';
import type { TransactionsSearch } from '@/lib/transactions-search-params';
import type { TransactionDetail } from '@/lib/types';

interface Props {
  items: TransactionDetail[];
  search: TransactionsSearch;
}

// Fixed widths for every column except Account and Offset, which each
// use minmax(170px, 1fr) so they share any extra width at wider
// viewports (real account paths are long and benefit from the room)
// and stay at 170px minimum on narrow viewports. Description is
// narrower and ellipsis-truncates when it overflows.
// Min total: 110 + 100 + 200 + 170 + 170 + 120 + 90 = 960px. The
// wrapper scrolls horizontally below that.
export const TRANSACTIONS_GRID_COLS =
  '110px 100px 200px minmax(170px, 1fr) minmax(170px, 1fr) 120px 90px';

export function TransactionsTable({ items, search }: Props) {
  return (
    // w-full bounds the container to the parent's width so overflow-x-auto
    // actually clips and scrolls instead of expanding the page. At wider
    // viewports the Account and Offset columns (each minmax(170px, 1fr))
    // share the extra width so the table fills its parent container.
    <div className="w-full overflow-x-auto rounded-md border bg-card">
      <div
        className="grid gap-3 bg-muted/50 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground"
        style={{ gridTemplateColumns: TRANSACTIONS_GRID_COLS }}
      >
        <span className="text-left">Date</span>
        <span className="text-center">Type</span>
        <span className="text-center">Description</span>
        <span className="text-left">Account</span>
        <span className="text-left">Offset</span>
        <span className="text-right">Amount</span>
        <span className="text-right">Status</span>
      </div>
      {items.map((tx) => (
        <TransactionRow key={tx.id} tx={tx} search={search} />
      ))}
    </div>
  );
}
