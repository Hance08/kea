import { MonthPicker } from '@/components/budgets/MonthPicker';
import { SavingsForm } from '@/components/savings/SavingsForm';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { listAccounts } from '@/lib/accounts';
import { deleteSavingsTarget, stopSavingsTarget } from '@/lib/api/savings';
import { activeVersions, currentMonth, depthIn } from '@/lib/budgets';
import { useSavingsTargets } from '@/lib/hooks/useSavings';
import { type MonthSearchParams, parseMonthSearch } from '@/lib/reports-search-params';
import { useAmountFormat, useServerConfig } from '@/lib/server-config';
import type { SavingsTarget } from '@/lib/types';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { toast } from 'sonner';

export const Route = createFileRoute('/savings')({
  validateSearch: (s): MonthSearchParams => parseMonthSearch(s),
  component: SavingsPage,
});

type Panel = { kind: 'add' } | { kind: 'edit'; target: SavingsTarget } | null;

function SavingsPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/savings' });
  const month = search.month ?? currentMonth();
  const query = useSavingsTargets();
  const queryClient = useQueryClient();
  const { formatCents } = useAmountFormat();
  const defaultCurrency = useServerConfig().defaults.currency;
  const accounts = useQuery({
    queryKey: ['accounts', 'list', 'all'],
    queryFn: () => listAccounts({ include_hidden: true }),
    staleTime: 60_000,
  });
  const currencyOf = (t: SavingsTarget) => {
    const acc = accounts.data?.items.find((a) => a.id === t.account_id);
    return acc?.currency || defaultCurrency;
  };
  const [panel, setPanel] = useState<Panel>(null);
  const [historyFor, setHistoryFor] = useState<number | null>(null);
  const [confirm, setConfirm] = useState<
    { kind: 'stop'; target: SavingsTarget } | { kind: 'delete'; target: SavingsTarget } | null
  >(null);

  const onError = (err: unknown) =>
    toast.error(err instanceof Error ? err.message : 'Request failed');
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['savings'] });
  const stopMutation = useMutation({
    mutationFn: (t: SavingsTarget) =>
      stopSavingsTarget({ account_name: t.account_name, effective_month: month }),
    onSuccess: invalidate,
    onError,
  });
  const deleteMutation = useMutation({
    mutationFn: (t: SavingsTarget) => deleteSavingsTarget(t.id),
    onSuccess: invalidate,
    onError,
  });

  const setMonth = (m: string | undefined) => navigate({ search: () => (m ? { month: m } : {}) });

  if (query.isPending || accounts.isPending) return <Skeleton className="h-48" />;
  if (query.isError || accounts.isError) {
    const err = query.error ?? accounts.error;
    return (
      <Alert variant="destructive">
        <AlertTitle>
          {query.isError ? 'Failed to load savings targets' : 'Failed to load accounts'}
        </AlertTitle>
        <AlertDescription>{err instanceof Error ? err.message : 'Unknown error'}</AlertDescription>
      </Alert>
    );
  }

  const all = query.data.items;
  const active = activeVersions(all, month);
  const names = active.map((t) => t.account_name);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Savings targets</h1>
        <Button size="sm" onClick={() => setPanel({ kind: 'add' })}>
          Add target
        </Button>
      </div>
      <p className="text-sm text-muted-foreground">
        Saved is the account's balance change in the month (sub-accounts included, opening balances
        excluded).
      </p>
      <MonthPicker value={search.month} onChange={setMonth} />

      {panel?.kind === 'add' && <SavingsForm defaultMonth={month} onDone={() => setPanel(null)} />}

      {confirm && (
        <div className="flex items-center gap-3 rounded border border-amber-500 p-3 text-sm">
          <span>
            {confirm.kind === 'stop'
              ? `Stop the savings target for ${confirm.target.account_name} from ${month}?`
              : `Delete the ${confirm.target.effective_month} version of ${confirm.target.account_name}?`}
          </span>
          <Button
            size="sm"
            variant="destructive"
            onClick={() => {
              if (confirm.kind === 'stop') stopMutation.mutate(confirm.target);
              else deleteMutation.mutate(confirm.target);
              setConfirm(null);
            }}
          >
            {confirm.kind === 'stop' ? 'Confirm stop' : 'Confirm delete'}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setConfirm(null)}>
            Cancel
          </Button>
        </div>
      )}

      {active.length === 0 ? (
        <p className="text-sm text-muted-foreground">No savings targets in {month}.</p>
      ) : (
        <ul className="divide-y rounded border">
          {active.map((t) => (
            <li key={t.account_id} data-testid="savings-setting-row" className="space-y-2 p-3">
              <div
                className="flex flex-wrap items-center justify-between gap-2"
                style={{ paddingLeft: `${depthIn(t.account_name, names) * 1.25}rem` }}
              >
                <div className="text-sm">
                  <div className="font-medium">{t.account_name}</div>
                  <div className="text-xs text-muted-foreground">
                    <span data-testid="savings-currency">{currencyOf(t)}</span> · from{' '}
                    <span>{t.effective_month}</span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <span className="tabular-nums text-sm">
                    {formatCents(t.amount, currencyOf(t))}
                  </span>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setPanel({ kind: 'edit', target: t })}
                  >
                    Edit
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setConfirm({ kind: 'stop', target: t })}
                  >
                    Stop
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setHistoryFor(historyFor === t.account_id ? null : t.account_id)}
                  >
                    History
                  </Button>
                </div>
              </div>
              {panel?.kind === 'edit' && panel.target.account_id === t.account_id && (
                <SavingsForm
                  initial={t}
                  lockAccount
                  defaultMonth={month}
                  onDone={() => setPanel(null)}
                />
              )}
              {historyFor === t.account_id && (
                <ul className="ml-4 space-y-1 text-xs">
                  {all
                    .filter((v) => v.account_id === t.account_id)
                    .map((v) => (
                      <li
                        key={v.id}
                        data-testid="savings-version"
                        className="flex items-center gap-3"
                      >
                        <span className="tabular-nums">{v.effective_month}</span>
                        <span className="tabular-nums">
                          {v.stopped ? 'stopped' : formatCents(v.amount, currencyOf(v))}
                        </span>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => setConfirm({ kind: 'delete', target: v })}
                        >
                          Delete
                        </Button>
                      </li>
                    ))}
                </ul>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
