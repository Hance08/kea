import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@radix-ui/react-dropdown-menu';
import { Plus } from 'lucide-react';
import { WIDGETS } from '../../lib/dashboard/registry';
import type { WidgetId } from '../../lib/dashboard/types';

interface Props {
  hidden: WidgetId[];
  onAdd: (id: WidgetId) => void;
}

export function AddWidgetMenu({ hidden, onAdd }: Props) {
  if (hidden.length === 0) {
    return (
      <button
        type="button"
        disabled
        className="flex items-center gap-1 rounded-md border px-3 py-1.5 text-sm font-medium opacity-50"
      >
        <Plus className="h-4 w-4" /> Add widget
      </button>
    );
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1 rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-muted"
        >
          <Plus className="h-4 w-4" /> Add widget
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="z-50 min-w-[12rem] rounded-md border bg-popover p-1 shadow-md"
      >
        {hidden.map((id) => (
          <DropdownMenuItem
            key={id}
            onSelect={() => onAdd(id)}
            className="cursor-pointer rounded px-2 py-1.5 text-sm outline-none hover:bg-muted"
          >
            {WIDGETS[id].title}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
