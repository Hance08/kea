import { stripAccountTypePrefix } from '@/lib/accounts';
import { cn } from '@/lib/cn';
import { useAmountFormat } from '@/lib/server-config';
import { DEFAULT_TRANSACTIONS_LIMIT } from '@/lib/transactions-search-params';
import type { ReportRow } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  rows: ReportRow[];
  currency: string;
  nameToId: Map<string, number>;
  // null → link to /accounts/{id} (Balance Sheet); otherwise drill into Transactions
  period: { startUnix: number; endUnix: number } | null;
  // parallel to rows; if provided, renders a colored swatch before each account name
  swatchColors?: string[];
  // when set, the table scrolls vertically and the header sticks to the top
  // after this many rows fit. Unset → render unchanged (no wrapper, no sticky).
  maxVisibleRows?: number;
}

export function ReportRowTable({
  rows,
  currency,
  nameToId,
  period,
  swatchColors,
  maxVisibleRows,
}: Props) {
  const { formatCents } = useAmountFormat();
  if (rows.length === 0) {
    return <p className="text-sm text-muted-foreground">No rows.</p>;
  }
  const table = (
    <table className="w-full text-sm">
      <thead
        className={cn(
          'text-xs uppercase text-muted-foreground',
          maxVisibleRows !== undefined && 'sticky top-0 z-10 bg-background',
        )}
      >
        <tr>
          <th className="py-1 text-left font-medium">Account</th>
          <th className="py-1 text-left font-medium">Offset</th>
          <th className="py-1 text-right font-medium">Amount</th>
          <th className="py-1 text-right font-medium">Tx</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row, rowIndex) => {
          const id = nameToId.get(row.account_name);
          const labelSpan = (
            <span className="truncate" title={row.account_name}>
              {stripAccountTypePrefix(row.account_name)}
            </span>
          );
          const swatch = swatchColors?.[rowIndex] ? (
            <span
              data-testid="row-swatch"
              className={cn(
                'mr-2 inline-block h-2 w-2 rounded-[2px] align-middle',
                swatchColors[rowIndex],
              )}
              aria-hidden="true"
            />
          ) : null;
          const linkContent = swatch ? (
            <span className="inline-flex min-w-0 items-center">
              {swatch}
              {labelSpan}
            </span>
          ) : (
            labelSpan
          );
          return (
            <tr key={row.account_name} className="border-t hover:bg-muted/40">
              <td className="py-1.5 max-w-[260px]">
                {period === null ? (
                  id !== undefined ? (
                    <Link
                      to="/accounts/$id"
                      params={{ id: String(id) }}
                      search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
                      className="hover:underline"
                    >
                      {linkContent}
                    </Link>
                  ) : (
                    linkContent
                  )
                ) : (
                  <Link
                    to="/transactions"
                    search={{
                      ...(id !== undefined ? { account_id: id } : {}),
                      start_time: period.startUnix,
                      end_time: period.endUnix,
                      limit: DEFAULT_TRANSACTIONS_LIMIT,
                      offset: 0,
                    }}
                    className="hover:underline"
                  >
                    {linkContent}
                  </Link>
                )}
              </td>
              <td className="py-1.5 text-muted-foreground" title={row.offset_account}>
                {stripAccountTypePrefix(row.offset_account)}
              </td>
              <td className="py-1.5 text-right font-mono tabular-nums">
                {formatCents(row.amount, currency)}
              </td>
              <td className="py-1.5 text-right text-muted-foreground">{row.tx_count}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );

  if (maxVisibleRows === undefined) return table;

  return (
    <div
      data-testid="report-row-scroll"
      className="overflow-y-auto rounded-md border px-3"
      style={{ maxHeight: `${maxVisibleRows * 2 + 2}rem` }}
    >
      {table}
    </div>
  );
}
