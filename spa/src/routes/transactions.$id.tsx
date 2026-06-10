// Pure layout for /transactions/$id — renders the matched child
// (`index` for the detail view, or `edit` for the edit form).
import { Outlet, createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/transactions/$id')({
  component: TransactionIdLayout,
});

function TransactionIdLayout() {
  return <Outlet />;
}
