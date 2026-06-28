interface Props {
  editing: boolean;
  onToggle: () => void;
}

export function EditModeHeader({ editing, onToggle }: Props) {
  return (
    <div className="mb-4 flex items-center justify-between">
      <h1 className="text-2xl font-semibold">Dashboard</h1>
      <button
        type="button"
        onClick={onToggle}
        className="rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-muted"
      >
        {editing ? 'Done' : 'Edit'}
      </button>
    </div>
  );
}
