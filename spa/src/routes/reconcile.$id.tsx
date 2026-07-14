import { ReconcileWorkspace } from '@/components/reconcile/ReconcileWorkspace';
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/reconcile/$id')({
  component: ReconcileRoute,
});

function ReconcileRoute() {
  const { id } = Route.useParams();
  return <ReconcileWorkspace accountId={Number(id)} />;
}
