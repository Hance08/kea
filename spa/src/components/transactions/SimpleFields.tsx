import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { buildCurrencyFilter, buildLeafFilter, combineFilters } from '@/lib/accountFilters';
import { determineType } from '@/lib/determineType';
import { listAccounts } from '@/lib/transactions';
import type { Account, TransactionType } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

interface Props {
  fromAccount: string;
  toAccount: string;
  amount: string;
  onFromChange: (name: string) => void;
  onToChange: (name: string) => void;
  onAmountChange: (value: string) => void;
  fieldErrors?: { fromAccount?: string; toAccount?: string; amount?: string };
}

export function SimpleFields(props: Props) {
  const accounts = useQuery({
    queryKey: ['accounts', 'list'],
    queryFn: listAccounts,
    staleTime: 60_000,
  });

  // Only leaf accounts can hold transactions; hide parents from the
  // From/To combobox suggestions to prevent invalid submissions.
  const leafFilter = useMemo(() => buildLeafFilter(accounts.data?.items), [accounts.data]);

  // Splits in one transaction must share a currency. Once From OR To
  // is picked, lock the other combobox to that account's currency.
  const accountByName = useMemo(() => {
    const m = new Map<string, Account>();
    for (const a of accounts.data?.items ?? []) m.set(a.name, a);
    return m;
  }, [accounts.data]);
  const pickedCurrency =
    accountByName.get(props.fromAccount)?.currency ?? accountByName.get(props.toAccount)?.currency;
  const accountFilter = useMemo(
    () => combineFilters(leafFilter, buildCurrencyFilter(pickedCurrency)),
    [leafFilter, pickedCurrency],
  );

  const derivedType: TransactionType | '…' = useMemo(() => {
    if (!accounts.data) return '…';
    const map = new Map(accounts.data.items.map((a) => [a.name, a.type]));
    const fromType = map.get(props.fromAccount);
    const toType = map.get(props.toAccount);
    if (!fromType || !toType) return '…';
    const amountNum = Math.round(Number(props.amount) * 100);
    if (!Number.isFinite(amountNum) || amountNum === 0) return '…';
    return determineType([
      {
        id: 0,
        account_id: 0,
        account_name: props.fromAccount,
        account_type: fromType,
        amount: -amountNum,
        currency: 'USD',
        memo: '',
      },
      {
        id: 0,
        account_id: 0,
        account_name: props.toAccount,
        account_type: toType,
        amount: amountNum,
        currency: 'USD',
        memo: '',
      },
    ]);
  }, [accounts.data, props.fromAccount, props.toAccount, props.amount]);

  return (
    <>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label htmlFor="from">From account</Label>
          <AccountCombobox
            id="from"
            value={props.fromAccount}
            onChange={(name) => props.onFromChange(name)}
            placeholder="Money comes from…"
            filter={accountFilter}
            aria-invalid={!!props.fieldErrors?.fromAccount}
          />
          {props.fieldErrors?.fromAccount && (
            <p className="mt-1 text-xs text-destructive">{props.fieldErrors.fromAccount}</p>
          )}
        </div>
        <div>
          <Label htmlFor="to">To account</Label>
          <AccountCombobox
            id="to"
            value={props.toAccount}
            onChange={(name) => props.onToChange(name)}
            placeholder="Money goes to…"
            filter={accountFilter}
            aria-invalid={!!props.fieldErrors?.toAccount}
          />
          {props.fieldErrors?.toAccount && (
            <p className="mt-1 text-xs text-destructive">{props.fieldErrors.toAccount}</p>
          )}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label htmlFor="amount">Amount</Label>
          <Input
            id="amount"
            type="text"
            inputMode="decimal"
            value={props.amount}
            onChange={(e) => props.onAmountChange(e.target.value)}
            placeholder="0.00"
            aria-invalid={!!props.fieldErrors?.amount}
          />
          {props.fieldErrors?.amount && (
            <p className="mt-1 text-xs text-destructive">{props.fieldErrors.amount}</p>
          )}
        </div>
        <div>
          <Label>Type (auto)</Label>
          <div className="flex h-10 items-center">
            {derivedType === '…' ? (
              <span className="text-sm text-muted-foreground">…</span>
            ) : (
              <TypeBadge type={derivedType} />
            )}
          </div>
        </div>
      </div>
    </>
  );
}
