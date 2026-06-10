import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { AccountsSearch } from '@/lib/accounts-search-params';

interface Props {
  search: AccountsSearch;
  onChange: (partial: Partial<AccountsSearch>) => void;
  onClear: () => void;
}

const TYPE_OPTIONS: { value: AccountsSearch['type']; label: string }[] = [
  { value: undefined, label: 'All types' },
  { value: 'A', label: 'Asset' },
  { value: 'L', label: 'Liability' },
  { value: 'C', label: 'Equity' },
  { value: 'R', label: 'Revenue' },
  { value: 'E', label: 'Expense' },
];

export function AccountFilters({ search, onChange, onClear }: Props) {
  return (
    <div className="mb-4 flex flex-wrap items-center gap-2">
      <Input
        value={search.q ?? ''}
        placeholder="Search accounts…"
        className="w-56"
        onChange={(e) => onChange({ q: e.target.value || undefined })}
      />
      <select
        value={search.type ?? ''}
        onChange={(e) =>
          onChange({ type: (e.target.value || undefined) as AccountsSearch['type'] })
        }
        className="rounded-md border bg-background px-2 py-1.5 text-sm"
      >
        {TYPE_OPTIONS.map((opt) => (
          <option key={opt.label} value={opt.value ?? ''}>
            {opt.label}
          </option>
        ))}
      </select>
      <label className="flex items-center gap-1.5 text-sm">
        <input
          type="checkbox"
          checked={search.include_hidden}
          onChange={(e) => onChange({ include_hidden: e.target.checked })}
        />
        Show hidden
      </label>
      {(search.q || search.type || search.include_hidden) && (
        <Button size="sm" variant="ghost" onClick={onClear}>
          Clear
        </Button>
      )}
    </div>
  );
}
