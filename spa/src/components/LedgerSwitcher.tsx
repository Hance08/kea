import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Skeleton } from '@/components/ui/skeleton';
import { getLedgers } from '@/lib/api';
import { useQuery } from '@tanstack/react-query';
import { Check, ChevronDown } from 'lucide-react';

export function LedgerSwitcher() {
  const ledgersQuery = useQuery({ queryKey: ['ledgers'], queryFn: getLedgers });

  if (ledgersQuery.isPending) {
    return <Skeleton data-testid="ledger-switcher-skeleton" className="mb-6 h-7 w-24" />;
  }

  if (ledgersQuery.isError) {
    return <div className="mb-6 text-lg font-semibold tracking-tight">kea</div>;
  }

  const { active, items } = ledgersQuery.data;

  return (
    <div className="mb-6">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            className="-mx-2 flex w-[calc(100%+1rem)] items-center justify-between px-2 text-lg font-semibold tracking-tight"
          >
            <span>{active}</span>
            <ChevronDown className="h-4 w-4 opacity-60" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="min-w-56">
          {items.map((item) => {
            const isActive = item.name === active;
            return (
              <DropdownMenuItem
                key={item.name}
                disabled={isActive}
                className="flex items-center justify-between"
              >
                <span>{item.name}</span>
                {isActive ? <Check data-testid="ledger-active-check" className="h-4 w-4" /> : null}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
