import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ApiError } from '@/lib/api';
import { setBudget } from '@/lib/api/budgets';
import { parseCents } from '@/lib/budgets';
import type { SetBudgetInput } from '@/lib/types';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { FormEvent } from 'react';
import { useId, useState } from 'react';
import { toast } from 'sonner';

interface Props {
  initial?: { account_name: string; amount: number; effective_month: string };
  lockAccount?: boolean;
  defaultMonth: string;
  onDone: () => void;
}

type FieldErrors = Partial<Record<'account_name' | 'effective_month' | 'amount', string>>;

export function BudgetForm({ initial, lockAccount, defaultMonth, onDone }: Props) {
  const id = useId();
  const queryClient = useQueryClient();
  const [account, setAccount] = useState(initial?.account_name ?? '');
  const [amount, setAmount] = useState(initial ? String(initial.amount / 100) : '');
  const [month, setMonth] = useState(defaultMonth);
  const [errors, setErrors] = useState<FieldErrors>({});

  const mutation = useMutation({
    mutationFn: (input: SetBudgetInput) => setBudget(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['budgets'] });
      toast.success('Budget saved');
      onDone();
    },
    onError: (err) => {
      if (
        err instanceof ApiError &&
        err.field &&
        err.field in { account_name: 1, effective_month: 1, amount: 1 }
      ) {
        setErrors({ [err.field]: err.message });
      } else {
        toast.error(err instanceof Error ? err.message : 'Failed to save budget');
      }
    },
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    const cents = parseCents(amount);
    const next: FieldErrors = {};
    if (!account) next.account_name = 'Choose an Expense account';
    if (Number.isNaN(cents)) next.amount = 'Enter a number';
    setErrors(next);
    if (Object.keys(next).length > 0) return;
    mutation.mutate({ account_name: account, effective_month: month, amount: cents });
  }

  return (
    <form onSubmit={submit} className="grid gap-3 rounded border p-3 sm:grid-cols-4 sm:items-end">
      <div className="sm:col-span-2">
        <Label htmlFor={`${id}-account`}>Account</Label>
        {lockAccount ? (
          <p id={`${id}-account`} className="py-2 text-sm font-medium">
            {account}
          </p>
        ) : (
          <AccountCombobox
            id={`${id}-account`}
            value={account}
            onChange={(name) => setAccount(name)}
            allowedTypes={['E']}
            placeholder="Expense account…"
            aria-invalid={!!errors.account_name}
          />
        )}
        {errors.account_name && (
          <p className="mt-1 text-xs text-destructive">{errors.account_name}</p>
        )}
      </div>
      <div>
        <Label htmlFor={`${id}-amount`}>Amount</Label>
        <Input
          id={`${id}-amount`}
          inputMode="decimal"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          aria-invalid={!!errors.amount}
        />
        {errors.amount && <p className="mt-1 text-xs text-destructive">{errors.amount}</p>}
      </div>
      <div>
        <Label htmlFor={`${id}-month`}>Effective from</Label>
        <Input
          id={`${id}-month`}
          type="month"
          value={month}
          onChange={(e) => setMonth(e.target.value)}
          aria-invalid={!!errors.effective_month}
        />
        {errors.effective_month && (
          <p className="mt-1 text-xs text-destructive">{errors.effective_month}</p>
        )}
      </div>
      <div className="flex gap-2 sm:col-span-4">
        <Button type="submit" size="sm" disabled={mutation.isPending}>
          Save
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={onDone}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
