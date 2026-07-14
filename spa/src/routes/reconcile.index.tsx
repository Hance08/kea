import { AccountChooser } from '@/components/reconcile/AccountChooser';
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/reconcile/')({
  component: AccountChooser,
});
