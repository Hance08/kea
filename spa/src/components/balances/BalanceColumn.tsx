import { BalanceColumnRow } from '@/components/balances/BalanceColumnRow';
import { Pagination } from '@/components/transactions/Pagination';
import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';

export const BALANCE_COLUMN_PAGE_SIZE = 8;

interface Props {
  label: 'Assets' | 'Liabilities';
  total: number;
  currency: string;
  rows: AccountBalance[]; // already sorted and sliced by the parent
  totalRowCount: number; // pre-slice length, used by pagination
  sortDir: 'asc' | 'desc';
  onToggleSort: () => void;
  offset: number;
  onOffsetChange: (offset: number) => void;
  emptyText: string;
}

export function BalanceColumn({
  label,
  total,
  currency,
  rows,
  totalRowCount,
  sortDir,
  onToggleSort,
  offset,
  onOffsetChange,
  emptyText,
}: Props) {
  const isEmpty = rows.length === 0;
  const negativeTotal = label === 'Liabilities';

  return (
    <Card className="overflow-hidden">
      <div className="flex items-center justify-between border-b bg-muted/40 px-3 py-2">
        <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {label}
        </div>
        <div
          className={cn(
            'text-sm font-semibold tabular-nums',
            negativeTotal ? 'text-destructive' : 'text-foreground',
          )}
        >
          {formatCents(total, currency)}
        </div>
      </div>

      {isEmpty ? (
        <CardContent className="p-6 text-center text-sm text-muted-foreground">
          {emptyText}
        </CardContent>
      ) : (
        <>
          <div className="flex items-center justify-between border-b px-3 py-2 text-xs uppercase text-muted-foreground">
            <span>Account</span>
            <button
              type="button"
              onClick={onToggleSort}
              className="inline-flex items-center gap-1 uppercase hover:text-foreground"
            >
              Balance
              <span aria-hidden="true">{sortDir === 'asc' ? '▲' : '▼'}</span>
            </button>
          </div>
          <div>
            {rows.map((row) => (
              <BalanceColumnRow key={row.account_id} row={row} />
            ))}
          </div>
          <div className="px-3 pb-3">
            <Pagination
              total={totalRowCount}
              limit={BALANCE_COLUMN_PAGE_SIZE}
              offset={offset}
              onChange={onOffsetChange}
            />
          </div>
        </>
      )}
    </Card>
  );
}
