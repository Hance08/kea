import { BalanceCard } from '@/components/balances/BalanceCard';
import { BALANCE_COLUMN_PAGE_SIZE } from '@/components/balances/BalanceColumn';
import type { AccountBalance } from '@/lib/types';

interface Props {
  rows: AccountBalance[]; // already sorted and sliced by the parent
  shares: (number | null)[]; // aligned with rows, same length
  columnLabel: 'Assets' | 'Liabilities';
}

export function BalanceCardGrid({ rows, shares, columnLabel }: Props) {
  const placeholderCount = BALANCE_COLUMN_PAGE_SIZE - rows.length;
  return (
    <div className="grid grid-cols-2 gap-3 p-3">
      {rows.map((row, i) => (
        <BalanceCard
          key={row.account_id}
          row={row}
          columnLabel={columnLabel}
          share={shares[i] ?? null}
        />
      ))}
      {Array.from({ length: placeholderCount }).map((_, i) => (
        <div
          // biome-ignore lint/suspicious/noArrayIndexKey: placeholder cards have no identity
          key={`placeholder-${i}`}
          aria-hidden="true"
          className="h-[88px] rounded-md"
        />
      ))}
    </div>
  );
}
