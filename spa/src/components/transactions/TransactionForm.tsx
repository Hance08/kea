import { SimpleFields } from '@/components/transactions/SimpleFields';
import { SplitsEditor, newSplitRow } from '@/components/transactions/SplitsEditor';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ApiError } from '@/lib/api';
import { listAccounts } from '@/lib/transactions';
import type {
  CreateTransactionInput,
  TransactionDetail,
  TransactionStatus,
  UpdateTransactionInput,
} from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

interface SplitRow {
  id?: number;
  clientKey: string;
  account_name: string;
  amountStr: string;
  currency: string;
  memo?: string;
}

interface FormState {
  description: string;
  date: string;
  status: TransactionStatus;
  fromAccount: string;
  toAccount: string;
  amount: string;
  splits: SplitRow[];
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

function dateToUnix(d: string): number {
  return Math.floor(new Date(`${d}T12:00:00Z`).getTime() / 1000);
}

function unixToDate(u: number): string {
  return new Date(u * 1000).toISOString().slice(0, 10);
}

function parseCents(s: string): number {
  const n = Number(s);
  if (!Number.isFinite(n)) return Number.NaN;
  return Math.round(n * 100);
}

function initialFromDetail(tx: TransactionDetail): FormState {
  return {
    description: tx.description,
    date: unixToDate(tx.timestamp),
    status: tx.status,
    fromAccount: '',
    toAccount: '',
    amount: '',
    splits: tx.splits.map((s) =>
      newSplitRow({
        id: s.id,
        account_name: s.account_name,
        amountStr: (s.amount / 100).toString(),
        currency: s.currency,
        memo: s.memo,
      }),
    ),
  };
}

function initialEmpty(): FormState {
  return {
    description: '',
    date: todayISO(),
    status: 'Cleared',
    fromAccount: '',
    toAccount: '',
    amount: '',
    splits: [newSplitRow(), newSplitRow()],
  };
}

interface Props {
  mode: 'create' | 'edit';
  initial?: TransactionDetail;
  onSubmit: (
    payload: CreateTransactionInput | UpdateTransactionInput,
  ) => Promise<TransactionDetail>;
  onSuccess: (tx: TransactionDetail) => void;
  onCancel: () => void;
}

export function TransactionForm({ mode, initial, onSubmit, onSuccess, onCancel }: Props) {
  const isEdit = mode === 'edit';
  const [state, setState] = useState<FormState>(
    initial ? initialFromDetail(initial) : initialEmpty(),
  );
  const [advanced, setAdvanced] = useState(isEdit);
  const [submitting, setSubmitting] = useState(false);
  const [topError, setTopError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const accountsQuery = useQuery({
    queryKey: ['accounts', 'list'],
    queryFn: listAccounts,
    staleTime: 60_000,
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setTopError(null);
    setFieldErrors({});

    try {
      const timestamp = dateToUnix(state.date);
      let splits: {
        id?: number;
        account_name: string;
        amount: number;
        currency: string;
        memo?: string;
      }[];

      if (advanced) {
        splits = state.splits.map((s) => ({
          id: s.id,
          account_name: s.account_name.trim(),
          amount: parseCents(s.amountStr),
          currency: s.currency || 'USD',
          memo: s.memo,
        }));
        if (splits.some((s) => !Number.isFinite(s.amount))) {
          setFieldErrors({ splits: 'All split amounts must be valid numbers.' });
          setSubmitting(false);
          return;
        }
      } else {
        const amt = parseCents(state.amount);
        if (!Number.isFinite(amt) || amt === 0) {
          setFieldErrors({ amount: 'Amount must be a non-zero number.' });
          setSubmitting(false);
          return;
        }
        if (!state.fromAccount.trim()) {
          setFieldErrors({ fromAccount: 'From account is required.' });
          setSubmitting(false);
          return;
        }
        if (!state.toAccount.trim()) {
          setFieldErrors({ toAccount: 'To account is required.' });
          setSubmitting(false);
          return;
        }
        const fromAcc = accountsQuery.data?.items.find((a) => a.name === state.fromAccount);
        const currency = fromAcc?.currency ?? 'USD';
        splits = [
          { account_name: state.fromAccount.trim(), amount: -amt, currency },
          { account_name: state.toAccount.trim(), amount: amt, currency },
        ];
      }

      let payload: CreateTransactionInput | UpdateTransactionInput;
      if (isEdit) {
        if (!initial) {
          setTopError('Edit mode requires the initial transaction.');
          setSubmitting(false);
          return;
        }
        payload = {
          id: initial.id,
          description: state.description,
          timestamp,
          status: state.status,
          splits,
        } satisfies UpdateTransactionInput;
      } else {
        payload = {
          description: state.description,
          timestamp,
          status: state.status,
          splits,
        } satisfies CreateTransactionInput;
      }

      const result = await onSubmit(payload);
      onSuccess(result);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.field) {
          const mapped =
            err.field === 'splits'
              ? 'splits'
              : err.field === 'from_account'
                ? 'fromAccount'
                : err.field === 'to_account'
                  ? 'toAccount'
                  : err.field;
          setFieldErrors({ [mapped]: err.message });
        } else {
          setTopError(err.message);
        }
      } else {
        setTopError(err instanceof Error ? err.message : 'Submission failed');
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">{isEdit ? 'Edit transaction' : 'New transaction'}</h1>
        {!isEdit && (
          <label className="flex items-center gap-2 text-sm text-muted-foreground">
            <input
              type="checkbox"
              checked={advanced}
              onChange={(e) => setAdvanced(e.target.checked)}
            />
            Advanced (edit splits)
          </label>
        )}
      </div>

      {topError && (
        <div className="rounded-md border border-destructive bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {topError}
        </div>
      )}

      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label htmlFor="date">Date</Label>
          <Input
            id="date"
            type="date"
            value={state.date}
            onChange={(e) => setState((s) => ({ ...s, date: e.target.value }))}
            aria-invalid={!!fieldErrors.timestamp}
          />
          {fieldErrors.timestamp && (
            <p className="mt-1 text-xs text-destructive">{fieldErrors.timestamp}</p>
          )}
        </div>
        <div>
          <Label htmlFor="status">Status</Label>
          <select
            id="status"
            className="flex h-10 w-full rounded-md border border-input bg-background px-2 text-sm"
            value={state.status}
            onChange={(e) =>
              setState((s) => ({ ...s, status: e.target.value as TransactionStatus }))
            }
          >
            <option value="Pending">Pending</option>
            <option value="Cleared">Cleared</option>
          </select>
        </div>
      </div>

      <div>
        <Label htmlFor="desc">Description</Label>
        <Input
          id="desc"
          type="text"
          value={state.description}
          onChange={(e) => setState((s) => ({ ...s, description: e.target.value }))}
          aria-invalid={!!fieldErrors.description}
        />
        {fieldErrors.description && (
          <p className="mt-1 text-xs text-destructive">{fieldErrors.description}</p>
        )}
      </div>

      {advanced ? (
        <SplitsEditor
          splits={state.splits}
          onChange={(next) => setState((s) => ({ ...s, splits: next }))}
          splitsError={fieldErrors.splits}
        />
      ) : (
        <SimpleFields
          fromAccount={state.fromAccount}
          toAccount={state.toAccount}
          amount={state.amount}
          onFromChange={(name) => setState((s) => ({ ...s, fromAccount: name }))}
          onToChange={(name) => setState((s) => ({ ...s, toAccount: name }))}
          onAmountChange={(value) => setState((s) => ({ ...s, amount: value }))}
          fieldErrors={{
            fromAccount: fieldErrors.fromAccount,
            toAccount: fieldErrors.toAccount,
            amount: fieldErrors.amount,
          }}
        />
      )}

      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" disabled={submitting}>
          {isEdit ? 'Save' : 'Create'}
        </Button>
      </div>
    </form>
  );
}
