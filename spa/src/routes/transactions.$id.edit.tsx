import { ReconciledBanner } from '@/components/transactions/ReconciledBanner';
import { TransactionForm } from '@/components/transactions/TransactionForm';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getTransaction, updateTransaction } from '@/lib/transactions';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/transactions/$id/edit')({
  component: EditTransactionPage,
});

function EditTransactionPage() {
  const { id } = Route.useParams();
  const txId = Number(id);
  const search = Route.useSearch();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ['transaction', txId],
    queryFn: () => getTransaction(txId),
  });

  if (query.isPending) return <Skeleton className="h-48 w-full" />;
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
  if (tx.status === 'Reconciled') {
    return (
      <div className="space-y-4">
        <ReconciledBanner />
        <Button asChild variant="outline">
          <Link
            to="/transactions/$id"
            params={{ id: String(tx.id) }}
            search={search}
          >
            ← Back to detail
          </Link>
        </Button>
      </div>
    );
  }

  return (
    <TransactionForm
      mode="edit"
      initial={tx}
      onSubmit={async (payload) => {
        const updated = await updateTransaction(payload as Parameters<typeof updateTransaction>[0]);
        queryClient.invalidateQueries({ queryKey: ['transactions'] });
        queryClient.invalidateQueries({ queryKey: ['transaction', txId] });
        queryClient.invalidateQueries({ queryKey: ['balances'] });
        return updated;
      }}
      onSuccess={(updated) =>
        navigate({
          to: '/transactions/$id',
          params: { id: String(updated.id) },
          search,
        })
      }
      onCancel={() =>
        navigate({
          to: '/transactions/$id',
          params: { id: String(tx.id) },
          search,
        })
      }
    />
  );
}
