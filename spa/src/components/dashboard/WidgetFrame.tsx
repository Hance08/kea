import { X } from 'lucide-react';
import type { ReactNode } from 'react';
import type { WidgetMeta } from '../../lib/dashboard/registry';

interface Props {
  meta: WidgetMeta;
  editing: boolean;
  onHide: () => void;
  children: ReactNode;
}

export function WidgetFrame({ meta, editing, onHide, children }: Props) {
  if (!editing) return <>{children}</>;
  return (
    <div className="flex h-full flex-col">
      <div className="dashboard-drag-handle flex shrink-0 items-center justify-between border-b px-2 py-1 text-xs font-medium cursor-move bg-muted/30">
        <span className="truncate">{meta.title}</span>
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
      <div className="flex-1 overflow-hidden p-2">{children}</div>
    </div>
  );
}
