import { cn } from '@/lib/cn';
import { Link, useRouterState } from '@tanstack/react-router';

const TABS = [
  { to: '/reports/income-statement', label: 'Income Statement' },
  { to: '/reports/income-breakdown', label: 'Income Breakdown' },
  { to: '/reports/expense-breakdown', label: 'Expense Breakdown' },
  { to: '/reports/balance-sheet', label: 'Balance Sheet' },
] as const;

export function TabNav() {
  const { location } = useRouterState();
  return (
    <nav aria-label="Report types" className="mb-4 flex flex-wrap gap-1 border-b">
      {TABS.map((tab) => {
        const isActive = location.pathname === tab.to;
        return (
          <Link
            key={tab.to}
            to={tab.to as string}
            aria-current={isActive ? 'page' : undefined}
            className={cn(
              '-mb-px border-b-2 px-3 py-2 text-sm transition-colors',
              isActive
                ? 'border-primary font-medium text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            {tab.label}
          </Link>
        );
      })}
    </nav>
  );
}
