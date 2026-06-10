import { Input } from '@/components/ui/input';
import { cn } from '@/lib/cn';
import { searchAccounts } from '@/lib/transactions';
import type { Account } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useId, useRef, useState } from 'react';

interface Props {
  value: string;             // account name (canonical identifier in API inputs)
  onChange: (name: string, account?: Account) => void;
  placeholder?: string;
  allowedTypes?: Account['type'][];
  disabled?: boolean;
  id?: string;
  'aria-invalid'?: boolean;
}

export function AccountCombobox({
  value,
  onChange,
  placeholder = 'Account…',
  allowedTypes,
  disabled,
  id,
  ...aria
}: Props) {
  const reactId = useId();
  const inputId = id ?? `acc-${reactId}`;
  const [query, setQuery] = useState(value);
  const [open, setOpen] = useState(false);
  const [debounced, setDebounced] = useState(query);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setQuery(value);
  }, [value]);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(query), 200);
    return () => clearTimeout(t);
  }, [query]);

  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (!containerRef.current?.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, []);

  const enabled = open && debounced.length > 0;
  const search = useQuery({
    queryKey: ['accounts', 'search', debounced],
    queryFn: () => searchAccounts(debounced),
    enabled,
    staleTime: 30_000,
  });

  const allItems = search.data?.items ?? [];
  const items = allowedTypes
    ? allItems.filter((a) => allowedTypes.includes(a.type))
    : allItems;

  return (
    <div ref={containerRef} className="relative">
      <Input
        id={inputId}
        value={query}
        disabled={disabled}
        placeholder={placeholder}
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
          onChange(e.target.value);
        }}
        {...aria}
      />
      {enabled && items.length > 0 && (
        <ul
          role="listbox"
          className="absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-md border bg-popover shadow-md"
        >
          {items.map((acc) => (
            <li
              key={acc.id}
              role="option"
              aria-selected={acc.name === value}
              className={cn(
                'cursor-pointer px-3 py-1.5 text-sm hover:bg-accent hover:text-accent-foreground',
                acc.name === value && 'bg-accent text-accent-foreground',
              )}
              onMouseDown={(e) => {
                e.preventDefault();
                setQuery(acc.name);
                setOpen(false);
                onChange(acc.name, acc);
              }}
            >
              <div>{acc.name}</div>
              <div className="text-xs text-muted-foreground">
                {acc.type} · {acc.currency}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
