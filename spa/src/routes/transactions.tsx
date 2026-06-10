import {
  parseTransactionsSearch,
  type TransactionsSearch,
} from '@/lib/transactions-search-params';
import { Outlet, createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/transactions')({
  validateSearch: (s): TransactionsSearch => parseTransactionsSearch(s),
  component: TransactionsLayout,
});

function TransactionsLayout() {
  return (
    <div>
      <Outlet />
    </div>
  );
}
