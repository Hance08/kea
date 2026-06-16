import { TabNav } from '@/components/reports/TabNav';
import { Outlet, createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/reports')({
  component: ReportsLayout,
});

function ReportsLayout() {
  return (
    <div>
      <h1 className="mb-3 text-xl font-semibold">Reports</h1>
      <TabNav />
      <Outlet />
    </div>
  );
}
