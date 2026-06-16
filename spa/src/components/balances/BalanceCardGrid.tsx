import { BalanceCard } from '@/components/balances/BalanceCard';
import { CARDS_PAGE_SIZE } from '@/components/balances/BalanceColumn';
import type { AccountBalance, BalanceHistoryPoint } from '@/lib/types';

interface Props {
  rows: AccountBalance[]; // already sorted and sliced by the parent
  shares: (number | null)[]; // aligned with rows, same length
  columnLabel: 'Assets' | 'Liabilities';
  historyByAccount?: Map<number, BalanceHistoryPoint[]>;
}

export function BalanceCardGrid({ rows, shares, columnLabel, historyByAccount }: Props) {
  const placeholderCount = CARDS_PAGE_SIZE - rows.length;
  return (
    <div className="grid grid-cols-2 gap-3 p-3">
      {rows.map((row, i) => (
        <BalanceCard
          key={row.account_id}
          row={row}
          columnLabel={columnLabel}
          share={shares[i] ?? null}
          points={historyByAccount?.get(row.account_id)}
        />
      ))}
      {Array.from({ length: placeholderCount }).map((_, i) => (
        <div
          // biome-ignore lint/suspicious/noArrayIndexKey: placeholder cards have no identity
          key={`placeholder-${i}`}
          aria-hidden="true"
          className="h-[112px] rounded-md"
        />
      ))}
    </div>
  );
}
