import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/')({
  component: AccountsListPage,
});

function AccountsListPage() {
  return (
    <div>
      <h1 className="text-xl font-semibold">Accounts</h1>
      <p className="mt-2 text-sm text-muted-foreground">List view goes here.</p>
    </div>
  );
}
