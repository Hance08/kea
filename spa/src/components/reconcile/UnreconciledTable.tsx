import { cn } from '@/lib/cn';
import { useAmountFormat } from '@/lib/server-config';
import type { ReconcileEntry } from '@/lib/types';

interface Props {
  entries: ReconcileEntry[];
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
}

function fmtDate(unix: number): string {
  const d = new Date(unix * 1000);
  const year = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  if (year === new Date().getFullYear()) return `${mm}-${dd}`;
  return `${String(year).slice(2)}-${mm}-${dd}`;
}

export function UnreconciledTable({ entries, selectedIds, onToggle }: Props) {
  const { formatBalanceAbs } = useAmountFormat();

  if (entries.length === 0) {
    return (
      <div className="rounded-md border p-6 text-center text-sm text-muted-foreground">
        No unreconciled transactions for this account.
      </div>
    );
  }

  return (
    <div className="max-h-[60vh] overflow-y-auto rounded-md border">
      <table className="w-full text-sm">
        <thead className="sticky top-0 z-10 bg-muted/50 backdrop-blur">
          <tr className="text-left text-xs uppercase text-muted-foreground">
            <th className="w-10 px-3 py-2"> </th>
            <th className="w-20 px-2 py-2">Date</th>
            <th className="px-2 py-2">Offset account</th>
            <th className="px-2 py-2">Description</th>
            <th className="w-32 px-3 py-2 text-right">Amount</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry) => {
            const checked = selectedIds.has(entry.id);
            const cls =
              entry.amount < 0 ? 'text-red-600' : entry.amount > 0 ? 'text-green-600' : '';
            return (
              <tr key={entry.id} className="border-t hover:bg-muted/30">
                <td className="px-3 py-2">
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => onToggle(entry.id)}
                    aria-label={`Toggle ${fmtDate(entry.timestamp)} ${entry.description} ${formatBalanceAbs(entry.amount)}`}
                  />
                </td>
                <td className="px-2 py-2 tabular-nums text-muted-foreground">
                  {fmtDate(entry.timestamp)}
                </td>
                <td className="px-2 py-2 text-muted-foreground" title={entry.offset_account}>
                  {entry.offset_account}
                </td>
                <td className="px-2 py-2" title={entry.description}>
                  {entry.description}
                </td>
                <td className={cn('px-3 py-2 text-right tabular-nums', cls)}>
                  {formatBalanceAbs(entry.amount)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
