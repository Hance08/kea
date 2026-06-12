import { Button } from '@/components/ui/button';
import { cn } from '@/lib/cn';
import { LayoutGrid, List } from 'lucide-react';
import { useState } from 'react';

export type BalancesView = 'list' | 'cards';

interface Props {
  value: BalancesView;
  onChange: (next: BalancesView) => void;
}

export function ViewToggle({ value, onChange }: Props) {
  const [active, setActive] = useState<BalancesView>(value);

  const select = (next: BalancesView) => {
    if (active === next) return;
    setActive(next);
    onChange(next);
  };

  return (
    <div className="inline-flex items-center gap-1 rounded-md border bg-card p-0.5">
      <ToggleButton active={active === 'list'} label="List view" onClick={() => select('list')}>
        <List className="h-4 w-4" />
      </ToggleButton>
      <ToggleButton
        active={active === 'cards'}
        label="Cards view"
        onClick={() => select('cards')}
      >
        <LayoutGrid className="h-4 w-4" />
      </ToggleButton>
    </div>
  );
}

function ToggleButton({
  active,
  label,
  onClick,
  children,
}: {
  active: boolean;
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      aria-label={label}
      aria-pressed={active}
      onClick={onClick}
      className={cn('h-7 w-7 p-0', active && 'bg-muted text-foreground')}
    >
      {children}
    </Button>
  );
}
