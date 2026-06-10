import { TransactionForm } from '@/components/transactions/TransactionForm';
import { createTransaction } from '@/lib/transactions';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/transactions/new')({
  component: NewTransactionPage,
});

function NewTransactionPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  return (
    <TransactionForm
      mode="create"
      onSubmit={async (payload) => {
        const created = await createTransaction(payload as Parameters<typeof createTransaction>[0]);
        queryClient.invalidateQueries({ queryKey: ['transactions'] });
        queryClient.invalidateQueries({ queryKey: ['balances'] });
        return created;
      }}
      onSuccess={(tx) =>
        navigate({
          to: '/transactions/$id',
          params: { id: String(tx.id) },
          search: { limit: 50, offset: 0 },
        })
      }
      onCancel={() => navigate({ to: '/transactions', search: { limit: 50, offset: 0 } })}
    />
  );
}
