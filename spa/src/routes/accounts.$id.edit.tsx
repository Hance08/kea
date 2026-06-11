import { AccountForm } from '@/components/accounts/AccountForm';
import { SystemAccountBanner } from '@/components/accounts/SystemAccountBanner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getAccount, isOpeningBalancesAccount, updateAccount } from '@/lib/accounts';
import { ApiError } from '@/lib/api';
import type { UpdateAccountInput } from '@/lib/types';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { toast } from 'sonner';

export const Route = createFileRoute('/accounts/$id/edit')({
  component: EditAccountPage,
});

function EditAccountPage() {
  const { id: idParam } = Route.useParams();
  const id = Number(idParam);
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [fieldErrors, setFieldErrors] = useState<Record<string, string | undefined>>({});
  const [formError, setFormError] = useState<string | undefined>(undefined);

  const accountQuery = useQuery({ queryKey: ['account', id], queryFn: () => getAccount(id) });

  const mutation = useMutation({
    mutationFn: (patch: UpdateAccountInput) => updateAccount(id, patch),
    onSuccess: (acc) => {
      qc.invalidateQueries({ queryKey: ['accounts'] });
      qc.invalidateQueries({ queryKey: ['account', id] });
      qc.invalidateQueries({ queryKey: ['balances'] });
      toast.success('Account updated');
      navigate({
        to: '/accounts/$id',
        params: { id: String(acc.id) },
        search: { include_hidden: false, show_parents: false, limit: 10, offset: 0 },
      });
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        if (err.field) {
          setFieldErrors({ [err.field]: err.message });
          setFormError(undefined);
        } else {
          setFormError(err.message);
        }
        return;
      }
      setFormError(err instanceof Error ? err.message : 'Save failed');
    },
  });

  if (accountQuery.isPending) return <Skeleton className="h-96 w-full" />;
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
  const segments = account.name.split(':');
  const parentName = segments.length > 1 ? segments.slice(0, -1).join(':') : undefined;

  return (
    <div>
      {isSystem && <SystemAccountBanner />}
      <AccountForm
        mode="edit"
        account={account}
        parentName={parentName}
        systemReadOnlyName={isSystem}
        pending={mutation.isPending}
        formError={formError}
        fieldErrors={fieldErrors}
        onSubmit={(patch) => {
          setFieldErrors({});
          setFormError(undefined);
          mutation.mutate(patch);
        }}
      />
    </div>
  );
}
