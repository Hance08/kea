import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/reports/net-worth')({
  beforeLoad: () => {
    throw redirect({ to: '/reports/balance-sheet', search: {} });
  },
});
