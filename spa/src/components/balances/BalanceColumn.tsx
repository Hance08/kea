import { BalanceCardGrid } from '@/components/balances/BalanceCardGrid';
import { BalanceColumnRow } from '@/components/balances/BalanceColumnRow';
import { Pagination } from '@/components/transactions/Pagination';
import { Card, CardContent } from '@/components/ui/card';
import { balanceColor } from '@/lib/accounts';
import { cn } from '@/lib/cn';
import { useAmountFormat } from '@/lib/server-config';
import type { AccountBalance, BalanceHistoryPoint } from '@/lib/types';

export const LIST_PAGE_SIZE = 8;
export const CARDS_PAGE_SIZE = 6;

// Per-view page size for slicing/pagination/placeholder count.
export function balanceColumnPageSize(view: 'list' | 'cards'): number {
  return view === 'cards' ? CARDS_PAGE_SIZE : LIST_PAGE_SIZE;
}

// Retained so existing list-mode tests/imports keep working.
export const BALANCE_COLUMN_PAGE_SIZE = LIST_PAGE_SIZE;

interface Props {
  label: 'Assets' | 'Liabilities';
  total: number;
  rows: AccountBalance[]; // already sorted and sliced by the parent
  shares: (number | null)[]; // aligned with rows; only consumed in cards mode
  totalRowCount: number; // pre-slice length, used by pagination
  sortDir: 'asc' | 'desc';
  onToggleSort: () => void;
  offset: number;
  onOffsetChange: (offset: number) => void;
  emptyText: string;
  view: 'list' | 'cards';
  historyByAccount?: Map<number, BalanceHistoryPoint[]>;
}

export function BalanceColumn({
  label,
  total,
  rows,
  shares,
  totalRowCount,
  sortDir,
  onToggleSort,
  offset,
  onOffsetChange,
  emptyText,
  view,
  historyByAccount,
}: Props) {
  const { formatBalanceAbs } = useAmountFormat();
  const isEmpty = rows.length === 0;
  const totalType = label === 'Assets' ? 'A' : 'L';
  const totalText = formatBalanceAbs(total);
  const totalColorClass = balanceColor(totalType, total);
  const arrow = sortDir === 'asc' ? '▲' : '▼';

  return (
    <Card className="overflow-hidden">
      <div className="flex items-center justify-between border-b bg-muted/40 px-3 py-2">
        <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {label}
        </div>
        {view === 'cards' && !isEmpty ? (
          <button
            type="button"
            onClick={onToggleSort}
            aria-label="Sort by balance"
            className={cn(
              'inline-flex items-center gap-1 text-sm font-semibold tabular-nums',
              'text-muted-foreground hover:text-foreground',
            )}
          >
            <span aria-hidden="true">{arrow}</span>
            <span className={totalColorClass}>{totalText}</span>
          </button>
        ) : (
          <div className={cn('text-sm font-semibold tabular-nums', totalColorClass)}>
            {totalText}
          </div>
        )}
      </div>

      {isEmpty ? (
        <CardContent className="p-6 text-center text-sm text-muted-foreground">
          {emptyText}
        </CardContent>
      ) : view === 'list' ? (
        <>
          <div className="flex items-center justify-between border-b px-3 py-2 text-xs uppercase text-muted-foreground">
            <span>Account</span>
            <button
              type="button"
              onClick={onToggleSort}
              className="inline-flex items-center gap-1 uppercase hover:text-foreground"
            >
              Balance
              <span aria-hidden="true">{arrow}</span>
            </button>
          </div>
          <div>
            {rows.map((row) => (
              <BalanceColumnRow key={row.account_id} row={row} />
            ))}
            {Array.from({ length: LIST_PAGE_SIZE - rows.length }).map((_, i) => (
              <div
                // biome-ignore lint/suspicious/noArrayIndexKey: placeholder rows have no identity
                key={`placeholder-${i}`}
                aria-hidden="true"
                className="px-3 py-2 text-sm"
              >
                &nbsp;
              </div>
            ))}
          </div>
          <div className="px-3 pb-3">
            <Pagination
              total={totalRowCount}
              limit={LIST_PAGE_SIZE}
              offset={offset}
              onChange={onOffsetChange}
            />
          </div>
        </>
      ) : (
        <>
          <BalanceCardGrid
            rows={rows}
            shares={shares}
            columnLabel={label}
            historyByAccount={historyByAccount}
          />
          <div className="px-3 pb-3">
            <Pagination
              total={totalRowCount}
              limit={CARDS_PAGE_SIZE}
              offset={offset}
              onChange={onOffsetChange}
            />
          </div>
        </>
      )}
    </Card>
  );
}
