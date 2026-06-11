import { Outlet, createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/$id')({
  component: AccountIdLayout,
});

function AccountIdLayout() {
  return <Outlet />;
}
