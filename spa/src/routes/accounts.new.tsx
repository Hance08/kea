import { AccountForm } from '@/components/accounts/AccountForm';
import { createAccount } from '@/lib/accounts';
import { ApiError } from '@/lib/api';
import type { CreateAccountInput } from '@/lib/types';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { toast } from 'sonner';

export const Route = createFileRoute('/accounts/new')({
  component: NewAccountPage,
});

function NewAccountPage() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [fieldErrors, setFieldErrors] = useState<Record<string, string | undefined>>({});
  const [formError, setFormError] = useState<string | undefined>(undefined);

  const mutation = useMutation({
    mutationFn: (input: CreateAccountInput) => createAccount(input),
    onSuccess: (acc) => {
      qc.invalidateQueries({ queryKey: ['accounts'] });
      qc.invalidateQueries({ queryKey: ['balances'] });
      toast.success('Account created');
      navigate({
        to: '/accounts/$id',
        params: { id: String(acc.id) },
        search: { include_hidden: false },
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
      setFormError(err instanceof Error ? err.message : 'Create failed');
    },
  });

  return (
    <AccountForm
      mode="create"
      pending={mutation.isPending}
      formError={formError}
      fieldErrors={fieldErrors}
      onSubmit={(input) => {
        setFieldErrors({});
        setFormError(undefined);
        mutation.mutate(input);
      }}
    />
  );
}
