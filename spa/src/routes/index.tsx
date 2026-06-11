import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/')({
  beforeLoad: () => {
    throw redirect({
      to: '/balances',
      search: { a_offset: 0, a_sort: 'balance_desc', l_offset: 0, l_sort: 'balance_desc' },
    });
  },
});
