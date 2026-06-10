import { type AccountsSearch, parseAccountsSearch } from '@/lib/accounts-search-params';
import { Outlet, createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts')({
  validateSearch: (s): AccountsSearch => parseAccountsSearch(s),
  component: AccountsLayout,
});

function AccountsLayout() {
  return (
    <div>
      <Outlet />
    </div>
  );
}
