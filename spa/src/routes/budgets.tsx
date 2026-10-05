import { BudgetForm } from '@/components/budgets/BudgetForm';
import { MonthPicker } from '@/components/budgets/MonthPicker';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { listAccounts } from '@/lib/accounts';
import { deleteBudget, stopBudget } from '@/lib/api/budgets';
import { activeBudgets, currentMonth, depthIn } from '@/lib/budgets';
import { useBudgets } from '@/lib/hooks/useBudgets';
import { type MonthSearchParams, parseMonthSearch } from '@/lib/reports-search-params';
import { useAmountFormat, useServerConfig } from '@/lib/server-config';
import type { Budget } from '@/lib/types';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { toast } from 'sonner';

export const Route = createFileRoute('/budgets')({
  validateSearch: (s): MonthSearchParams => parseMonthSearch(s),
  component: BudgetsPage,
});

type Panel = { kind: 'add' } | { kind: 'edit'; budget: Budget } | null;

function BudgetsPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/budgets' });
  const month = search.month ?? currentMonth();
  const query = useBudgets();
  const queryClient = useQueryClient();
  const { formatCents } = useAmountFormat();
  const defaultCurrency = useServerConfig().defaults.currency;
  const accounts = useQuery({
    queryKey: ['accounts', 'list'],
    queryFn: () => listAccounts(),
    staleTime: 60_000,
  });
  const currencyOf = (b: Budget) => {
    const acc = accounts.data?.items.find((a) => a.id === b.account_id);
    return acc?.currency || defaultCurrency;
  };
  const [panel, setPanel] = useState<Panel>(null);
  const [historyFor, setHistoryFor] = useState<number | null>(null);
  const [confirm, setConfirm] = useState<
    { kind: 'stop'; budget: Budget } | { kind: 'delete'; budget: Budget } | null
  >(null);

  const onError = (err: unknown) =>
    toast.error(err instanceof Error ? err.message : 'Request failed');
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['budgets'] });
  const stopMutation = useMutation({
    mutationFn: (b: Budget) => stopBudget({ account_name: b.account_name, effective_month: month }),
    onSuccess: invalidate,
    onError,
  });
  const deleteMutation = useMutation({
    mutationFn: (b: Budget) => deleteBudget(b.id),
    onSuccess: invalidate,
    onError,
  });

  const setMonth = (m: string | undefined) => navigate({ search: () => (m ? { month: m } : {}) });

  if (query.isPending) return <Skeleton className="h-48" />;
  if (query.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load budgets</AlertTitle>
        <AlertDescription>
          {query.error instanceof Error ? query.error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    );
  }

  const all = query.data.items;
  const active = activeBudgets(all, month);
  const names = active.map((b) => b.account_name);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Budgets</h1>
        <Button size="sm" onClick={() => setPanel({ kind: 'add' })}>
          Add budget
        </Button>
      </div>
      <MonthPicker value={search.month} onChange={setMonth} />

      {panel?.kind === 'add' && <BudgetForm defaultMonth={month} onDone={() => setPanel(null)} />}

      {confirm && (
        <div className="flex items-center gap-3 rounded border border-amber-500 p-3 text-sm">
          <span>
            {confirm.kind === 'stop'
              ? `Stop the budget for ${confirm.budget.account_name} from ${month}?`
              : `Delete the ${confirm.budget.effective_month} version of ${confirm.budget.account_name}?`}
          </span>
          <Button
            size="sm"
            variant="destructive"
            onClick={() => {
              if (confirm.kind === 'stop') stopMutation.mutate(confirm.budget);
              else deleteMutation.mutate(confirm.budget);
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
        <p className="text-sm text-muted-foreground">No budgets in {month}.</p>
      ) : (
        <ul className="divide-y rounded border">
          {active.map((b) => (
            <li key={b.account_id} data-testid="budget-setting-row" className="space-y-2 p-3">
              <div
                className="flex flex-wrap items-center justify-between gap-2"
                style={{ paddingLeft: `${depthIn(b.account_name, names) * 1.25}rem` }}
              >
                <div className="text-sm">
                  <div className="font-medium">{b.account_name}</div>
                  <div className="text-xs text-muted-foreground">
                    <span data-testid="budget-currency">{currencyOf(b)}</span> · from{' '}
                    <span>{b.effective_month}</span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <span className="tabular-nums text-sm">
                    {formatCents(b.amount, currencyOf(b))}
                  </span>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setPanel({ kind: 'edit', budget: b })}
                  >
                    Edit
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setConfirm({ kind: 'stop', budget: b })}
                  >
                    Stop
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setHistoryFor(historyFor === b.account_id ? null : b.account_id)}
                  >
                    History
                  </Button>
                </div>
              </div>
              {panel?.kind === 'edit' && panel.budget.account_id === b.account_id && (
                <BudgetForm
                  initial={b}
                  lockAccount
                  defaultMonth={month}
                  onDone={() => setPanel(null)}
                />
              )}
              {historyFor === b.account_id && (
                <ul className="ml-4 space-y-1 text-xs">
                  {all
                    .filter((v) => v.account_id === b.account_id)
                    .map((v) => (
                      <li
                        key={v.id}
                        data-testid="budget-version"
                        className="flex items-center gap-3"
                      >
                        <span className="tabular-nums">{v.effective_month}</span>
                        <span className="tabular-nums">
                          {v.stopped ? 'stopped' : formatCents(v.amount, currencyOf(v))}
                        </span>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => setConfirm({ kind: 'delete', budget: v })}
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
