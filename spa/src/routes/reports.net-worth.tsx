import { DEFAULT_BALANCE_SHEET_CHART_RANGE } from '@/lib/reports-search-params';
import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/reports/net-worth')({
  beforeLoad: () => {
    throw redirect({
      to: '/reports/balance-sheet',
      search: { chart_range: DEFAULT_BALANCE_SHEET_CHART_RANGE },
    });
  },
});
