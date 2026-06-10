// Placeholder — replaced by the full create form in Task 19.
import {
  parseTransactionsSearch,
  type TransactionsSearch,
} from '@/lib/transactions-search-params';
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/transactions/new')({
  validateSearch: (s): TransactionsSearch => parseTransactionsSearch(s),
  component: () => <div>New Transaction</div>,
});
