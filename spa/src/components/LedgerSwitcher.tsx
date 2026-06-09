import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Skeleton } from '@/components/ui/skeleton';
import { getLedgers, switchLedger } from '@/lib/api';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, ChevronDown, Loader2 } from 'lucide-react';
import { useState } from 'react';
import { toast } from 'sonner';

export function LedgerSwitcher() {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const ledgersQuery = useQuery({ queryKey: ['ledgers'], queryFn: getLedgers });
  const mutation = useMutation({
    mutationFn: (name: string) => switchLedger(name),
    onSuccess: (info) => {
      setOpen(false);
      queryClient.invalidateQueries();
      toast.success(`Switched to ${info.name}`);
    },
    onError: (err) => {
      setOpen(false);
      toast.error(err instanceof Error ? err.message : 'Switch failed');
    },
  });

  if (ledgersQuery.isPending) {
    return <Skeleton data-testid="ledger-switcher-skeleton" className="mb-6 h-7 w-24" />;
  }

  if (ledgersQuery.isError) {
    return <div className="mb-6 text-lg font-semibold tracking-tight">kea</div>;
  }

  const { active, items } = ledgersQuery.data;

  return (
    <div className="mb-6">
      <DropdownMenu
        open={open}
        onOpenChange={(next) => {
          // While a switch is in-flight, keep the menu open.
          if (mutation.isPending) return;
          setOpen(next);
        }}
      >
        <DropdownMenuTrigger asChild disabled={mutation.isPending}>
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
            const isSwitching = mutation.isPending && mutation.variables === item.name;
            return (
              <DropdownMenuItem
                key={item.name}
                disabled={isActive || mutation.isPending}
                onSelect={(e) => {
                  // Always prevent the default close-on-select so we can
                  // keep the menu open while the mutation is in-flight and
                  // close it ourselves in onSuccess/onError.
                  e.preventDefault();
                  if (isActive || mutation.isPending) {
                    return;
                  }
                  mutation.mutate(item.name);
                }}
                className="flex items-center justify-between"
              >
                <span>{item.name}</span>
                {isSwitching ? (
                  <Loader2
                    data-testid="ledger-switching-spinner"
                    className="h-4 w-4 animate-spin"
                  />
                ) : isActive ? (
                  <Check data-testid="ledger-active-check" className="h-4 w-4" />
                ) : null}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
