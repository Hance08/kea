import { ReconciledBanner } from '@/components/transactions/ReconciledBanner';
import { StatusText } from '@/components/transactions/StatusText';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { formatCents } from '@/lib/format';
import { deleteTransaction, getTransaction } from '@/lib/transactions';
import { DEFAULT_TRANSACTIONS_LIMIT } from '@/lib/transactions-search-params';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';

export const Route = createFileRoute('/transactions/$id/')({
  component: TransactionDetailPage,
});

function TransactionDetailPage() {
  const { id } = Route.useParams();
  const txId = Number(id);
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const query = useQuery({
    queryKey: ['transaction', txId],
    queryFn: () => getTransaction(txId),
  });

  const deleteMut = useMutation({
    mutationFn: () => deleteTransaction(txId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['transactions'] });
      queryClient.invalidateQueries({ queryKey: ['balances'] });
      navigate({ to: '/transactions', search: { limit: DEFAULT_TRANSACTIONS_LIMIT, offset: 0 } });
    },
  });

  if (query.isPending) {
    return <Skeleton className="h-48 w-full" />;
  }

  if (query.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load transaction</AlertTitle>
        <AlertDescription>
          {query.error instanceof Error ? query.error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    );
  }

  const tx = query.data;
  const isReconciled = tx.status === 'Reconciled';
  const date = new Date(tx.timestamp * 1000).toISOString().slice(0, 10);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Link
          to="/transactions"
          search={{ limit: DEFAULT_TRANSACTIONS_LIMIT, offset: 0 }}
          className="text-sm text-muted-foreground hover:underline"
        >
          ← Back to transactions
        </Link>
        {!isReconciled && (
          <div className="flex gap-2">
            <Button asChild size="sm" variant="outline">
              <Link
                to="/transactions/$id/edit"
                params={{ id: String(tx.id) }}
                search={{ limit: DEFAULT_TRANSACTIONS_LIMIT, offset: 0 }}
              >
                Edit
              </Link>
            </Button>
            <Button
              size="sm"
              variant="destructive"
              onClick={() => setConfirmingDelete(true)}
              disabled={deleteMut.isPending}
            >
              Delete
            </Button>
          </div>
        )}
      </div>

      {isReconciled && <ReconciledBanner />}

      <div className="rounded-md border bg-card p-4">
        <div className="mb-3 flex items-center gap-3">
          <h2 className="text-lg font-semibold">{tx.description}</h2>
          <TypeBadge type={tx.type} />
        </div>
        <dl className="grid grid-cols-2 gap-2 text-sm">
          <dt className="text-muted-foreground">Date</dt>
          <dd>{date}</dd>
          <dt className="text-muted-foreground">Status</dt>
          <dd>
            <StatusText status={tx.status} />
          </dd>
        </dl>
      </div>

      <div className="rounded-md border bg-card">
        <div className="grid grid-cols-[1fr_120px_1fr] gap-3 bg-muted/50 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          <span>Account</span>
          <span className="text-right">Amount</span>
          <span>Memo</span>
        </div>
        {tx.splits.map((s) => (
          <div
            key={s.id}
            className="grid grid-cols-[1fr_120px_1fr] gap-3 border-t px-3 py-2 text-sm"
          >
            <span>{s.account_name}</span>
            <span
              className={`text-right tabular-nums ${
                s.amount < 0 ? 'text-red-600' : s.amount > 0 ? 'text-green-600' : ''
              }`}
            >
              {formatCents(s.amount, s.currency)}
            </span>
            <span className="text-muted-foreground">{s.memo || '—'}</span>
          </div>
        ))}
      </div>

      {confirmingDelete && (
        <Alert variant="destructive">
          <AlertTitle>Delete this transaction?</AlertTitle>
          <AlertDescription className="mt-2 space-y-3">
            <div>This cannot be undone.</div>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="destructive"
                onClick={() => deleteMut.mutate()}
                disabled={deleteMut.isPending}
              >
                Yes, delete
              </Button>
              <Button size="sm" variant="outline" onClick={() => setConfirmingDelete(false)}>
                Cancel
              </Button>
            </div>
            {deleteMut.isError && (
              <div className="text-sm">
                {deleteMut.error instanceof Error ? deleteMut.error.message : 'Delete failed'}
              </div>
            )}
          </AlertDescription>
        </Alert>
      )}
    </div>
  );
}
