import { formatCents } from '@/lib/format';
import type { ReportRow } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  rows: ReportRow[];
  currency: string;
  nameToId: Map<string, number>;
  // null → link to /accounts/{id} (Balance Sheet); otherwise drill into Transactions
  period: { startUnix: number; endUnix: number } | null;
}

export function ReportRowTable({ rows, currency, nameToId, period }: Props) {
  if (rows.length === 0) {
    return <p className="text-sm text-muted-foreground">No rows.</p>;
  }
  return (
    <table className="w-full text-sm">
      <thead className="text-xs uppercase text-muted-foreground">
        <tr>
          <th className="py-1 text-left font-medium">Account</th>
          <th className="py-1 text-left font-medium">Offset</th>
          <th className="py-1 text-right font-medium">Amount</th>
          <th className="py-1 text-right font-medium">Tx</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => {
          const id = nameToId.get(row.account_name);
          const linkContent = (
            <span className="truncate" title={row.account_name}>
              {row.account_name}
            </span>
          );
          return (
            <tr key={row.account_name} className="border-t hover:bg-muted/40">
              <td className="py-1.5 max-w-[260px]">
                {period === null ? (
                  id !== undefined ? (
                    <Link
                      to="/accounts/$id"
                      params={{ id: String(id) }}
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
                    }}
                    className="hover:underline"
                  >
                    {linkContent}
                  </Link>
                )}
              </td>
              <td className="py-1.5 text-muted-foreground">{row.offset_account}</td>
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
}
