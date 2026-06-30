import { Outlet, createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/reconcile')({
  component: ReconcileLayout,
});

function ReconcileLayout() {
  return (
    <div>
      <Outlet />
    </div>
  );
}
