import { Button } from '@/components/ui/button';
import { stripAccountTypePrefix } from '@/lib/accounts';
import { formatCents } from '@/lib/format';
import type { ReportRow } from '@/lib/types';
import { useState } from 'react';

interface Props {
  rows: ReportRow[];
  total: number; // same currency as rows; may be negative or zero
  currency: string;
  // If set, only the first `limit` rows render until the user expands.
  // Omit (or undefined) to render everything.
  limit?: number;
  variant?: 'income' | 'expense';
}

const COLOR: Record<NonNullable<Props['variant']>, string> = {
  income: 'bg-green-600',
  expense: 'bg-red-600',
};

export function ProportionBar({ rows, total, currency, limit, variant = 'income' }: Props) {
  const [expanded, setExpanded] = useState(false);

  if (rows.length === 0) {
    return <p className="text-sm text-muted-foreground">No data to display.</p>;
  }

  const denom = Math.abs(total);
  const shown = limit && !expanded ? rows.slice(0, limit) : rows;
  const hiddenCount = rows.length - shown.length;

  return (
    <div className="space-y-2">
      {shown.map((row) => {
        const pct = denom === 0 ? 0 : (Math.abs(row.amount) / denom) * 100;
        return (
          <div
            key={`${row.account_name}|${row.offset_account}`}
            data-testid="prop-bar"
            className="text-sm"
          >
            <div className="flex items-baseline justify-between gap-2">
              <span className="truncate" title={row.account_name}>
                {stripAccountTypePrefix(row.account_name)}
              </span>
              <span className="font-mono text-xs tabular-nums">
                {formatCents(row.amount, currency)}
              </span>
            </div>
            <div className="mt-1 h-2 w-full overflow-hidden rounded bg-muted">
              <div
                data-testid="bar-fill"
                className={COLOR[variant]}
                style={{ width: `${pct}%`, height: '100%' }}
              />
            </div>
          </div>
        );
      })}
      {hiddenCount > 0 && (
        <Button variant="ghost" size="sm" onClick={() => setExpanded(true)}>
          + {hiddenCount} more
        </Button>
      )}
    </div>
  );
}
