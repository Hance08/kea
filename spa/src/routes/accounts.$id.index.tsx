import { AccountDetailHeader } from '@/components/accounts/AccountDetailHeader';
import { ChildAccountsCard } from '@/components/accounts/ChildAccountsCard';
import { DeleteAccountButton } from '@/components/accounts/DeleteAccountButton';
import { RecentTransactionsCard } from '@/components/accounts/RecentTransactionsCard';
import { SystemAccountBanner } from '@/components/accounts/SystemAccountBanner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  deleteAccount,
  getAccount,
  getAccountBalance,
  getAccountTree,
  isOpeningBalancesAccount,
} from '@/lib/accounts';
import { getBalances } from '@/lib/api';
import { listTransactions } from '@/lib/transactions';
import type { AccountNode } from '@/lib/types';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';

export const Route = createFileRoute('/accounts/$id/')({
  component: AccountDetailPage,
});

function findChildren(roots: AccountNode[] | undefined, id: number): AccountNode[] | undefined {
  if (!roots) return undefined;
  const stack = [...roots];
  while (stack.length > 0) {
    const node = stack.pop();
    if (!node) continue;
    if (node.account.id === id) return node.children;
    stack.push(...node.children);
  }
  return undefined;
}

function AccountDetailPage() {
  const { id: idParam } = Route.useParams();
  const id = Number(idParam);
  const navigate = useNavigate();
  const qc = useQueryClient();

  const accountQuery = useQuery({ queryKey: ['account', id], queryFn: () => getAccount(id) });
  const balanceQuery = useQuery({
    queryKey: ['account', id, 'balance'],
    queryFn: () => getAccountBalance(id),
  });
  const treeQuery = useQuery({
    queryKey: ['accounts', 'tree', { include_hidden: true }],
    queryFn: () => getAccountTree({ include_hidden: true }),
  });
  const balancesQuery = useQuery({ queryKey: ['balances'], queryFn: getBalances });
  const txQuery = useQuery({
    queryKey: ['transactions', { account_id: id, limit: 20 }],
    queryFn: () => listTransactions({ account_id: id }, { limit: 20 }),
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteAccount(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['accounts'] });
      qc.invalidateQueries({ queryKey: ['account', id] });
      qc.invalidateQueries({ queryKey: ['balances'] });
      toast.success('Account deleted');
      navigate({ to: '/accounts', search: { include_hidden: false, show_parents: false } });
    },
    onError: (e: unknown) => {
      toast.error(e instanceof Error ? e.message : 'Delete failed');
    },
  });

  if (accountQuery.isPending) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }
  if (accountQuery.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load account</AlertTitle>
        <AlertDescription className="mt-2 space-y-3">
          <div>
            {accountQuery.error instanceof Error ? accountQuery.error.message : 'Unknown error'}
          </div>
          <Button onClick={() => accountQuery.refetch()} size="sm">
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  const account = accountQuery.data;
  const isSystem = isOpeningBalancesAccount(account.name);
  const children = findChildren(treeQuery.data, id) ?? [];
  const isParent = children.length > 0;
  const hasTransactions = (txQuery.data?.total_count ?? 0) > 0;

  let blockedReason: string | undefined;
  if (isSystem) blockedReason = 'System account cannot be deleted.';
  else if (isParent)
    blockedReason = `Has ${children.length} child accounts. Delete or reassign them first.`;
  else if (hasTransactions)
    blockedReason = `Has ${txQuery.data?.total_count} transactions. Delete or reassign them first.`;

  const deleteSlot = isSystem ? null : (
    <DeleteAccountButton
      blockedReason={blockedReason}
      onDelete={() => deleteMutation.mutate()}
      pending={deleteMutation.isPending}
    />
  );

  return (
    <div>
      {isSystem && <SystemAccountBanner />}
      <AccountDetailHeader account={account} balance={balanceQuery.data} deleteSlot={deleteSlot} />
      {isParent ? (
        <ChildAccountsCard accounts={children} balances={balancesQuery.data?.items} />
      ) : (
        <RecentTransactionsCard accountId={id} />
      )}
    </div>
  );
}
