import { stripAccountTypePrefix } from '@/lib/accounts';
import { cn } from '@/lib/cn';
import type { ReportRow } from '@/lib/types';

export type CompositionVariant = 'income' | 'expense';

export interface CompositionSegment {
  label: string; // account name (stripped of type prefix), or "Other (N)"
  amount: number; // |amount|
  pct: number; // 0..100, exact (not rounded)
  colorClass: string; // Tailwind background class
  textClass: string; // Tailwind text class for in-segment label contrast
  isOther: boolean;
}

const GRADIENT: Record<CompositionVariant, { bg: string; text: string }[]> = {
  expense: [
    { bg: 'bg-red-700', text: 'text-white' },
    { bg: 'bg-red-600', text: 'text-white' },
    { bg: 'bg-red-500', text: 'text-white' },
    { bg: 'bg-red-400', text: 'text-white' },
    { bg: 'bg-red-300', text: 'text-red-900' },
    { bg: 'bg-red-200', text: 'text-red-900' },
  ],
  income: [
    { bg: 'bg-emerald-700', text: 'text-white' },
    { bg: 'bg-emerald-600', text: 'text-white' },
    { bg: 'bg-emerald-500', text: 'text-white' },
    { bg: 'bg-emerald-400', text: 'text-white' },
    { bg: 'bg-emerald-300', text: 'text-emerald-900' },
    { bg: 'bg-emerald-200', text: 'text-emerald-900' },
  ],
};

const OTHER_BG = 'bg-slate-300';
const OTHER_TEXT = 'text-slate-700';

export function partitionForComposition(
  rows: ReportRow[],
  total: number,
  variant: CompositionVariant,
  topN = 6,
): { segments: CompositionSegment[]; swatchColors: string[] } {
  const palette = GRADIENT[variant];
  const denom = Math.abs(total);

  // Sort by |amount| descending, but remember each row's original index so
  // we can build a parallel swatchColors array.
  const indexed = rows.map((r, i) => ({ row: r, i }));
  indexed.sort((a, b) => Math.abs(b.row.amount) - Math.abs(a.row.amount));

  const swatchColors: string[] = new Array(rows.length).fill(OTHER_BG);
  const segments: CompositionSegment[] = [];

  const primary = indexed.slice(0, topN);
  const rest = indexed.slice(topN);

  primary.forEach(({ row, i }, segIdx) => {
    const color = palette[segIdx];
    const amount = Math.abs(row.amount);
    // Mixed-sign rows can push |amount|/|total| > 1; clamp so widths stay valid.
    segments.push({
      label: stripAccountTypePrefix(row.account_name),
      amount,
      pct: denom === 0 ? 0 : Math.min(100, (amount / denom) * 100),
      colorClass: color.bg,
      textClass: color.text,
      isOther: false,
    });
    swatchColors[i] = color.bg;
  });

  if (rest.length > 0) {
    const sum = rest.reduce((acc, { row }) => acc + Math.abs(row.amount), 0);
    segments.push({
      label: `Other (${rest.length})`,
      amount: sum,
      pct: denom === 0 ? 0 : Math.min(100, (sum / denom) * 100),
      colorClass: OTHER_BG,
      textClass: OTHER_TEXT,
      isOther: true,
    });
    // swatchColors for rest indices remain OTHER_BG (already set above).
  }

  return { segments, swatchColors };
}

interface Props {
  rows: ReportRow[];
  total: number;
  currency: string;
  variant: CompositionVariant;
  className?: string;
}

function buildAriaLabel(segments: CompositionSegment[], variant: CompositionVariant): string {
  const which = variant === 'income' ? 'Income' : 'Expense';
  const top = segments.slice(0, 3).map((s) => `${s.label} ${Math.round(s.pct)}%`).join(', ');
  return `${which} composition: ${top}`;
}

export function CompositionBar({ rows, total, currency: _currency, variant, className }: Props) {
  if (rows.length === 0 || total === 0) return null;
  const { segments } = partitionForComposition(rows, total, variant);
  return (
    <div
      data-testid="composition-bar"
      role="img"
      aria-label={buildAriaLabel(segments, variant)}
      className={cn('flex h-7 w-full overflow-hidden rounded text-[10px] font-medium', className)}
    >
      {segments.map((seg) => (
        <div
          // label is unique per segment (account name, or the single "Other" bucket).
          key={seg.label}
          data-testid="composition-segment"
          className={cn('flex items-center justify-start', seg.colorClass, seg.textClass)}
          style={{ width: `${seg.pct}%` }}
          title={`${seg.label}: ${Math.round(seg.pct)}%`}
        />
      ))}
    </div>
  );
}
