export function SystemAccountBanner() {
  return (
    <div className="mb-4 rounded-md border border-amber-500/40 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
      This is a system account. It cannot be deleted, and its name is managed automatically. Edit
      its description or hide it from views if needed.
    </div>
  );
}
