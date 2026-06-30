import { MismatchDialog } from '@/components/reconcile/MismatchDialog';
import { ReconcileHeader } from '@/components/reconcile/ReconcileHeader';
import { UnreconciledTable } from '@/components/reconcile/UnreconciledTable';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getAccount, getAccountTree } from '@/lib/accounts';
import { ApiError } from '@/lib/api';
import { commitReconcile, getUnreconciled } from '@/lib/reconcile';
import type { AccountNode } from '@/lib/types';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { useMemo, useState } from 'react';
import { toast } from 'sonner';

interface Props {
  accountId: number;
}

function parseStatementCents(s: string): number | null {
  const trimmed = s.trim();
  if (trimmed === '') return null;
  const n = Number(trimmed);
  return Number.isFinite(n) ? Math.round(n * 100) : null;
}

function hasChildren(roots: AccountNode[] | undefined, id: number): boolean {
  if (!roots) return false;
  const stack = [...roots];
  while (stack.length > 0) {
    const node = stack.pop();
    if (!node) continue;
    if (node.account.id === id) return node.children.length > 0;
    stack.push(...node.children);
  }
  return false;
}

export function ReconcileWorkspace({ accountId }: Props) {
  const qc = useQueryClient();
  const accountQuery = useQuery({
    queryKey: ['account', accountId],
    queryFn: () => getAccount(accountId),
  });
  const treeQuery = useQuery({
    queryKey: ['accounts', 'tree', { include_hidden: true }],
    queryFn: () => getAccountTree({ include_hidden: true }),
  });
  const unreconciledQuery = useQuery({
    queryKey: ['unreconciled', accountId],
    queryFn: () => getUnreconciled(accountId),
  });

  const [statement, setStatement] = useState('');
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [mismatch, setMismatch] = useState<{
    open: boolean;
    difference: number;
    statementCents: number;
    transactionIds: number[];
  } | null>(null);

  const entries = unreconciledQuery.data?.entries ?? [];
  const lastReconciled = unreconciledQuery.data?.last_reconciled_balance ?? 0;

  const statementCents = parseStatementCents(statement);
  const statementInvalid = statement.trim() !== '' && statementCents === null;

  const clearedCents = useMemo(
    () =>
      entries
        .filter((e) => selectedIds.has(e.id))
        .reduce((sum, e) => sum + e.amount, 0),
    [entries, selectedIds],
  );
  const differenceCents =
    statementCents !== null ? statementCents - (lastReconciled + clearedCents) : null;

  const commit = useMutation({
    mutationFn: (vars: {
      allowMismatch: boolean;
      statementCents?: number;
      transactionIds?: number[];
    }) => {
      const sc = vars.statementCents ?? statementCents ?? 0;
      const ids = vars.transactionIds ?? [...selectedIds];
      return commitReconcile(accountId, sc, ids, vars.allowMismatch);
    },
    onSuccess: (resp) => {
      qc.invalidateQueries({ queryKey: ['unreconciled', accountId] });
      qc.invalidateQueries({ queryKey: ['balances'] });
      qc.invalidateQueries({ queryKey: ['transactions'] });
      qc.invalidateQueries({ queryKey: ['account', accountId, 'balance'] });
      qc.invalidateQueries({ queryKey: ['balances', 'history'] });
      toast.success(
        `Reconciled ${resp.reconciled_count} transaction${resp.reconciled_count === 1 ? '' : 's'}`,
      );
      setStatement('');
      setSelectedIds(new Set());
      setMismatch(null);
    },
    onError: (e: unknown) => {
      if (e instanceof ApiError && e.status === 409 && e.difference !== undefined) {
        // Snapshot the inputs that produced the 409 so re-firing with allow_mismatch:true
        // uses the same values, even if the user edits the form while the dialog is open.
        setMismatch({
          open: true,
          difference: e.difference,
          statementCents: statementCents ?? 0,
          transactionIds: [...selectedIds],
        });
        return;
      }
      if (e instanceof ApiError && e.status === 400) {
        qc.invalidateQueries({ queryKey: ['unreconciled', accountId] });
      }
      toast.error(e instanceof Error ? e.message : 'Reconcile failed');
    },
  });

  if (accountQuery.isPending) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (accountQuery.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Account not found</AlertTitle>
        <AlertDescription className="mt-2 space-y-3">
          <Button asChild size="sm">
            <Link to="/reconcile">Back to chooser</Link>
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  const account = accountQuery.data;
  if (account.type !== 'A' && account.type !== 'L') {
    return (
      <Alert variant="destructive">
        <AlertTitle>Reconcile only applies to Asset and Liability accounts</AlertTitle>
        <AlertDescription className="mt-2">
          <Button asChild size="sm">
            <Link to="/reconcile">Back to chooser</Link>
          </Button>
        </AlertDescription>
      </Alert>
    );
  }
  if (hasChildren(treeQuery.data, accountId)) {
    return (
      <Alert variant="destructive">
        <AlertTitle>This account has child accounts; reconcile a leaf instead.</AlertTitle>
        <AlertDescription className="mt-2">
          <Button asChild size="sm">
            <Link to="/reconcile">Back to chooser</Link>
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  const canCommit =
    statementCents !== null && selectedIds.size > 0 && !commit.isPending;

  return (
    <div>
      <ReconcileHeader
        accountName={account.name}
        currency={account.currency}
        unreconciledCount={entries.length}
        statement={statement}
        onStatementChange={setStatement}
        statementInvalid={statementInvalid}
        lastReconciledCents={lastReconciled}
        clearedCents={clearedCents}
        differenceCents={differenceCents}
      />
      <UnreconciledTable
        entries={entries}
        selectedIds={selectedIds}
        onToggle={(id) =>
          setSelectedIds((prev) => {
            const next = new Set(prev);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            return next;
          })
        }
      />
      <div className="mt-4 flex justify-end gap-2">
        <Button asChild variant="outline">
          <Link to="/reconcile">Back</Link>
        </Button>
        <Button
          onClick={() => commit.mutate({ allowMismatch: false })}
          disabled={!canCommit}
        >
          {commit.isPending ? 'Reconciling…' : 'Reconcile'}
        </Button>
      </div>
      <MismatchDialog
        open={mismatch?.open ?? false}
        differenceCents={mismatch?.difference ?? 0}
        pending={commit.isPending}
        onCancel={() => setMismatch(null)}
        onConfirm={() =>
          mismatch &&
          commit.mutate({
            allowMismatch: true,
            statementCents: mismatch.statementCents,
            transactionIds: mismatch.transactionIds,
          })
        }
      />
    </div>
  );
}
