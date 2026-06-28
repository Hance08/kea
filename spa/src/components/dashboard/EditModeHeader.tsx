import type { WidgetId } from '../../lib/dashboard/types';
import { AddWidgetMenu } from './AddWidgetMenu';

interface Props {
  editing: boolean;
  onToggle: () => void;
  hidden: WidgetId[];
  onAdd: (id: WidgetId) => void;
  onReset: () => void;
}

export function EditModeHeader({ editing, onToggle, hidden, onAdd, onReset }: Props) {
  return (
    <div className="mb-4 flex items-center justify-between">
      <h1 className="text-2xl font-semibold">Dashboard</h1>
      <div className="flex items-center gap-2">
        {editing && <AddWidgetMenu hidden={hidden} onAdd={onAdd} />}
        {editing && (
          <button
            type="button"
            onClick={() => {
              if (confirm('Reset dashboard to defaults?')) onReset();
            }}
            className="rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-muted"
          >
            Reset
          </button>
        )}
        <button
          type="button"
          onClick={onToggle}
          className="rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-muted"
        >
          {editing ? 'Done' : 'Edit'}
        </button>
      </div>
    </div>
  );
}
