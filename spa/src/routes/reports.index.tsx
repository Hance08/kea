import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/reports/')({
  beforeLoad: () => {
    // TODO: remove cast when T14 adds /reports/income-statement
    throw redirect({ to: '/reports/income-statement' as string });
  },
});
