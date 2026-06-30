import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getAccountTree } from '@/lib/accounts';
import { getBalances } from '@/lib/api';
import { cn } from '@/lib/cn';
import { useAmountFormat } from '@/lib/server-config';
import type { Account, AccountNode } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';

function flattenLeaves(roots: AccountNode[]): Account[] {
  const out: Account[] = [];
  const walk = (node: AccountNode) => {
    if (node.children.length === 0) out.push(node.account);
    else for (const c of node.children) walk(c);
  };
  for (const r of roots) walk(r);
  return out;
}

export function AccountChooser() {
  const { formatCents } = useAmountFormat();
  const treeQuery = useQuery({
    queryKey: ['accounts', 'tree', { include_hidden: false }],
    queryFn: () => getAccountTree({ include_hidden: false }),
  });
  const balancesQuery = useQuery({ queryKey: ['balances'], queryFn: getBalances });

  if (treeQuery.isPending || balancesQuery.isPending) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-12 w-full" />
      </div>
    );
  }
  if (treeQuery.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load accounts</AlertTitle>
        <AlertDescription className="mt-2">
          <Button size="sm" onClick={() => treeQuery.refetch()}>
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  const accounts = flattenLeaves(treeQuery.data ?? []).filter(
    (a) => a.type === 'A' || a.type === 'L',
  );
  const balanceById = new Map((balancesQuery.data?.items ?? []).map((b) => [b.account_id, b]));

  if (accounts.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">No asset or liability accounts to reconcile.</p>
    );
  }

  return (
    <div>
      <h1 className="mb-3 text-xl font-semibold">Reconcile</h1>
      <p className="mb-4 text-sm text-muted-foreground">
        Pick an account to reconcile against a statement.
      </p>
      <ul className="rounded-md border">
        {accounts.map((a) => {
          const bal = balanceById.get(a.id);
          return (
            <li key={a.id} className="border-t first:border-t-0">
              <Link
                to="/reconcile/$id"
                params={{ id: String(a.id) }}
                className="flex items-center justify-between gap-3 px-4 py-3 hover:bg-muted/50"
              >
                <div>
                  <div className="font-medium">{a.name}</div>
                  <div className="text-xs text-muted-foreground">{a.currency}</div>
                </div>
                <div className={cn('tabular-nums', bal && bal.amount < 0 && 'text-destructive')}>
                  {bal ? formatCents(bal.amount, bal.currency) : '—'}
                </div>
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
