import { Popover, PopoverContent, PopoverTrigger } from '@radix-ui/react-popover';
import { Settings, X } from 'lucide-react';
import type { ReactNode } from 'react';
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
  if (!editing) return <>{children}</>;
  const ConfigForm = meta.ConfigForm;
  return (
    <div className="flex h-full flex-col">
      <div className="dashboard-drag-handle flex shrink-0 items-center justify-between gap-1 border-b px-2 py-1 text-xs font-medium cursor-move bg-muted/30">
        <span className="truncate">{meta.title}</span>
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
            className="rounded p-0.5 hover:bg-muted"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
      <div className="flex-1 overflow-hidden p-2">{children}</div>
    </div>
  );
}
