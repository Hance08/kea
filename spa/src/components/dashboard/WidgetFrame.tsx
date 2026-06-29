import { Popover, PopoverContent, PopoverTrigger } from '@radix-ui/react-popover';
import { Settings, X } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '../../lib/cn';
import type { WidgetMeta } from '../../lib/dashboard/registry';

interface Props {
  meta: WidgetMeta<unknown>;
  editing: boolean;
  config: unknown;
  onConfigChange: (c: unknown) => void;
  onHide: () => void;
  children: ReactNode;
}

export function WidgetFrame({ meta, editing, config, onConfigChange, onHide, children }: Props) {
  const ConfigForm = meta.ConfigForm;
  return (
    <div className="flex h-full flex-col">
      <div
        className={cn(
          'flex shrink-0 items-center justify-between gap-1 border-b px-3 py-1.5 text-xs font-medium text-muted-foreground',
          editing && 'dashboard-drag-handle cursor-move bg-muted/30 text-foreground',
        )}
      >
        <span className="truncate">{meta.title}</span>
        {editing && (
          <div className="flex items-center gap-1">
            {ConfigForm && (
              <Popover>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    aria-label={`configure ${meta.id}`}
                    onClick={(e) => e.stopPropagation()}
                    onMouseDown={(e) => e.stopPropagation()}
                    className="rounded p-0.5 hover:bg-muted"
                  >
                    <Settings className="h-3.5 w-3.5" />
                  </button>
                </PopoverTrigger>
                <PopoverContent
                  align="end"
                  sideOffset={4}
                  className="z-50 min-w-[12rem] rounded-md border bg-popover p-3 shadow-md"
                  onClick={(e) => e.stopPropagation()}
                  onMouseDown={(e) => e.stopPropagation()}
                >
                  <ConfigForm config={config} onChange={onConfigChange} />
                </PopoverContent>
              </Popover>
            )}
            <button
              type="button"
              aria-label={`hide ${meta.id}`}
              onClick={(e) => {
                e.stopPropagation();
                onHide();
              }}
              onMouseDown={(e) => e.stopPropagation()}
              className="rounded p-0.5 hover:bg-muted"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        )}
      </div>
      <div className="flex-1 overflow-hidden px-3 py-2">{children}</div>
    </div>
  );
}
