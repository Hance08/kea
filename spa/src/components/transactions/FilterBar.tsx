import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import type { TransactionsSearch } from '@/lib/transactions-search-params';
import type { TransactionStatus, TransactionType } from '@/lib/types';

const TYPES: TransactionType[] = [
  'Expense',
  'Income',
  'Transfer',
  'Opening',
  'Deposit',
  'Withdrawal',
  'Other',
  'Investment',
];
const STATUSES: TransactionStatus[] = ['Pending', 'Cleared', 'Reconciled'];

interface Props {
  search: TransactionsSearch;
  onChange: (partial: Partial<TransactionsSearch>) => void;
  onClear: () => void;
}

function unixToDate(u?: number): string {
  if (!u) return '';
  return new Date(u * 1000).toISOString().slice(0, 10);
}

function dateToUnix(d: string, endOfDay: boolean): number | undefined {
  if (!d) return undefined;
  const ms = new Date(`${d}T${endOfDay ? '23:59:59' : '00:00:00'}Z`).getTime();
  return Number.isNaN(ms) ? undefined : Math.floor(ms / 1000);
}

export function FilterBar({ search, onChange, onClear }: Props) {
  const hasAny =
    search.account_id !== undefined ||
    search.type !== undefined ||
    search.status !== undefined ||
    search.start_time !== undefined ||
    search.end_time !== undefined ||
    (search.description !== undefined && search.description !== '');

  return (
    // Use 6 columns only at xl (>=1280px) where the inputs comfortably
    // fit on one row; stack below that to avoid pushing the page wider
    // than the viewport.
    <div className="mb-4 grid grid-cols-1 gap-3 rounded-md border bg-card p-3 xl:grid-cols-6">
      <div className="xl:col-span-2">
        <Label htmlFor="f-account">Account</Label>
        <AccountCombobox
          id="f-account"
          value=""
          onChange={(_name, account) => onChange({ account_id: account?.id })}
          placeholder="Any account"
        />
      </div>

      <div>
        <Label htmlFor="f-type">Type</Label>
        <select
          id="f-type"
          className="flex h-10 w-full rounded-md border border-input bg-background px-2 text-sm"
          value={search.type ?? ''}
          onChange={(e) =>
            onChange({
              type: (e.target.value || undefined) as TransactionType | undefined,
            })
          }
        >
          <option value="">Any</option>
          {TYPES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </div>

      <div>
        <Label htmlFor="f-status">Status</Label>
        <select
          id="f-status"
          className="flex h-10 w-full rounded-md border border-input bg-background px-2 text-sm"
          value={search.status ?? ''}
          onChange={(e) =>
            onChange({
              status: (e.target.value || undefined) as TransactionStatus | undefined,
            })
          }
        >
          <option value="">Any</option>
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </div>

      <div>
        <Label htmlFor="f-from">From date</Label>
        <Input
          id="f-from"
          type="date"
          value={unixToDate(search.start_time)}
          onChange={(e) => onChange({ start_time: dateToUnix(e.target.value, false) })}
        />
      </div>

      <div>
        <Label htmlFor="f-to">To date</Label>
        <Input
          id="f-to"
          type="date"
          value={unixToDate(search.end_time)}
          onChange={(e) => onChange({ end_time: dateToUnix(e.target.value, true) })}
        />
      </div>

      <div className="xl:col-span-4">
        <Input
          id="f-desc"
          type="text"
          value={search.description ?? ''}
          onChange={(e) =>
            onChange({
              description: e.target.value === '' ? undefined : e.target.value,
            })
          }
          placeholder="Search description…"
        />
      </div>

      <div className="flex items-end gap-2 xl:col-span-2">
        {hasAny && (
          <Button variant="outline" size="sm" onClick={onClear}>
            Clear filters
          </Button>
        )}
      </div>
    </div>
  );
}
