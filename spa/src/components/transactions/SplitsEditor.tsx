import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { buildCurrencyFilter, buildLeafFilter, combineFilters } from '@/lib/accountFilters';
import { determineType } from '@/lib/determineType';
import { listAccounts } from '@/lib/transactions';
import type { Account, AccountType, TransactionType } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

interface SplitRow {
  id?: number;
  // Stable per-row identifier for React's reconciliation. Distinct from `id`
  // which is the server-assigned split id (undefined for unsaved rows).
  clientKey: string;
  account_name: string;
  amountStr: string;
  currency: string;
  memo?: string;
}

export function newSplitRow(initial?: Partial<SplitRow>): SplitRow {
  return {
    clientKey: crypto.randomUUID(),
    account_name: '',
    amountStr: '',
    currency: 'USD',
    memo: '',
    ...initial,
  };
}

interface Props {
  splits: SplitRow[];
  onChange: (next: SplitRow[]) => void;
  splitsError?: string;
}

function parseCents(s: string): number {
  const n = Number(s);
  if (!Number.isFinite(n)) return Number.NaN;
  return Math.round(n * 100);
}

function balance(splits: SplitRow[]): number {
  return splits.reduce((acc, s) => {
    const cents = parseCents(s.amountStr);
    return Number.isFinite(cents) ? acc + cents : acc;
  }, 0);
}

export function SplitsEditor({ splits, onChange, splitsError }: Props) {
  const accounts = useQuery({
    queryKey: ['accounts', 'list'],
    queryFn: listAccounts,
    staleTime: 60_000,
  });

  // Only leaf accounts can hold transactions; hide parents from every
  // split's combobox to prevent invalid submissions.
  const leafFilter = useMemo(() => buildLeafFilter(accounts.data?.items), [accounts.data]);

  const accountByName = useMemo(() => {
    const m = new Map<string, Account>();
    for (const a of accounts.data?.items ?? []) m.set(a.name, a);
    return m;
  }, [accounts.data]);

  const accountTypeMap = useMemo(() => {
    const m = new Map<string, AccountType>();
    for (const a of accounts.data?.items ?? []) m.set(a.name, a.type);
    return m;
  }, [accounts.data]);

  // Splits in one transaction must share a currency. Once any split
  // has a picked account, lock every split's combobox to that
  // account's currency.
  const pickedCurrency = useMemo(() => {
    for (const s of splits) {
      const acc = accountByName.get(s.account_name);
      if (acc) return acc.currency;
    }
    return undefined;
  }, [accountByName, splits]);
  const accountFilter = useMemo(
    () => combineFilters(leafFilter, buildCurrencyFilter(pickedCurrency)),
    [leafFilter, pickedCurrency],
  );

  const derivedType: TransactionType | '…' = useMemo(() => {
    if (!accounts.data) return '…';
    const sd = splits.map((s) => {
      const cents = parseCents(s.amountStr);
      const t = accountTypeMap.get(s.account_name);
      if (!t || !Number.isFinite(cents)) return null;
      return {
        id: 0,
        account_id: 0,
        account_name: s.account_name,
        account_type: t,
        amount: cents,
        currency: s.currency || 'USD',
        memo: s.memo ?? '',
      };
    });
    if (sd.some((x) => x === null)) return '…';
    return determineType(sd as Parameters<typeof determineType>[0]);
  }, [accounts.data, accountTypeMap, splits]);

  const bal = balance(splits);
  const balanced = bal === 0 && splits.length >= 2;

  const updateRow = (i: number, partial: Partial<SplitRow>) => {
    const next = splits.map((s, idx) => (idx === i ? { ...s, ...partial } : s));
    onChange(next);
  };

  const addRow = () => {
    onChange([...splits, newSplitRow()]);
  };

  const removeRow = (i: number) => {
    onChange(splits.filter((_, idx) => idx !== i));
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>Splits</Label>
        {derivedType !== '…' && (
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span>Type:</span>
            <TypeBadge type={derivedType} />
          </div>
        )}
      </div>

      {splitsError && (
        <div className="rounded-md border border-destructive bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {splitsError}
        </div>
      )}

      <div className="rounded-md border bg-card">
        <div className="grid grid-cols-[2fr_1fr_1fr_120px_30px] gap-2 bg-muted/50 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          <span>Account</span>
          <span className="text-right">Amount</span>
          <span>Memo</span>
          <span>Currency</span>
          <span />
        </div>
        {splits.map((s, i) => (
          <div
            key={s.clientKey}
            className="grid grid-cols-[2fr_1fr_1fr_120px_30px] items-center gap-2 border-t px-3 py-2 text-sm"
          >
            <AccountCombobox
              value={s.account_name}
              // When the user picks an account from the dropdown, sync
              // the row's currency to that account so it doesn't get
              // out of step with the picked currency lock.
              onChange={(name, account) =>
                updateRow(i, {
                  account_name: name,
                  ...(account ? { currency: account.currency } : {}),
                })
              }
              placeholder="Account…"
              filter={accountFilter}
            />
            <Input
              type="text"
              inputMode="decimal"
              className="text-right"
              value={s.amountStr}
              onChange={(e) => updateRow(i, { amountStr: e.target.value })}
              placeholder="0.00"
            />
            <Input
              type="text"
              value={s.memo ?? ''}
              onChange={(e) => updateRow(i, { memo: e.target.value })}
              placeholder="(optional)"
            />
            <Input
              type="text"
              value={s.currency}
              onChange={(e) => updateRow(i, { currency: e.target.value.toUpperCase() })}
            />
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => removeRow(i)}
              aria-label={`Remove split ${i + 1}`}
            >
              ×
            </Button>
          </div>
        ))}
      </div>

      <div className="flex items-center justify-between text-sm">
        <Button type="button" variant="outline" size="sm" onClick={addRow}>
          + Add split
        </Button>
        <span className={balanced ? 'text-green-600' : 'text-red-600'}>
          Balance: {(bal / 100).toFixed(2)} {balanced ? '✓' : '✗'}
        </span>
      </div>
    </div>
  );
}
