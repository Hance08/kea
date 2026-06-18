import { LedgerSwitcher } from '@/components/LedgerSwitcher';
import { cn } from '@/lib/cn';
import { Link, useRouterState } from '@tanstack/react-router';

interface NavItem {
  label: string;
  to?: string;
  prefix?: boolean; // when true, isActive matches any pathname starting with `to`
}

const NAV: NavItem[] = [
  { label: 'Balances', to: '/balances' },
  { label: 'Accounts', to: '/accounts' },
  { label: 'Transactions', to: '/transactions' },
  { label: 'Reports', to: '/reports', prefix: true },
  { label: 'Reconcile' },
];

const FOOTER_NAV: NavItem[] = [{ label: 'Settings', to: '/settings' }];

export function Sidebar() {
  const { location } = useRouterState();
  return (
    <nav
      aria-label="Main navigation"
      className="flex w-56 shrink-0 flex-col border-r bg-muted/30 p-4"
    >
      <LedgerSwitcher />
      <NavList items={NAV} pathname={location.pathname} className="flex-1" />
      <NavList items={FOOTER_NAV} pathname={location.pathname} />
    </nav>
  );
}

function NavList({
  items,
  pathname,
  className,
}: {
  items: NavItem[];
  pathname: string;
  className?: string;
}) {
  return (
    <ul className={cn('space-y-1', className)}>
      {items.map((item) => {
        if (!item.to) {
          return (
            <li key={item.label}>
              <span
                aria-disabled="true"
                className="block cursor-not-allowed rounded px-3 py-2 text-sm text-muted-foreground"
                title="Coming soon"
              >
                {item.label}
              </span>
            </li>
          );
        }
        const isActive = item.prefix
          ? pathname === item.to || pathname.startsWith(`${item.to}/`)
          : pathname === item.to;
        return (
          <li key={item.label}>
            <Link
              to={item.to}
              className={cn(
                'block rounded px-3 py-2 text-sm transition-colors',
                isActive ? 'bg-primary text-primary-foreground font-medium' : 'hover:bg-muted',
              )}
            >
              {item.label}
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
