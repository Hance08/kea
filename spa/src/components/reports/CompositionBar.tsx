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

// Strip an account-type prefix like "Expenses:" or "Income:".
// Mirrors stripAccountTypePrefix from @/lib/accounts but inline so this
// helper has no surprise import cycles when used in tests.
function shortLabel(name: string): string {
  const i = name.indexOf(':');
  return i >= 0 ? name.slice(i + 1) : name;
}

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
    segments.push({
      label: shortLabel(row.account_name),
      amount,
      pct: denom === 0 ? 0 : (amount / denom) * 100,
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
      pct: denom === 0 ? 0 : (sum / denom) * 100,
      colorClass: OTHER_BG,
      textClass: OTHER_TEXT,
      isOther: true,
    });
    // swatchColors for rest indices remain OTHER_BG (already set above).
  }

  return { segments, swatchColors };
}
