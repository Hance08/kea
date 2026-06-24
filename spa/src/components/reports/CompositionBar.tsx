import { stripAccountTypePrefix } from '@/lib/accounts';
import { cn } from '@/lib/cn';
import { useAmountFormat } from '@/lib/server-config';
import type { ReportRow } from '@/lib/types';
import { useState } from 'react';

export type CompositionVariant = 'income' | 'expense';

export interface CompositionSegment {
  label: string; // account name (stripped of type prefix), or "Other (N)"
  fullName: string; // unmodified account name, or the label for the Other bucket
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
  // When the caller's `total` is smaller than the rows actually sum to (e.g. a
  // transient stale render where the KPI total updates ahead of the row data),
  // using `|total|` as denominator would let percentages exceed 100%. Use the
  // larger of `|total|` and the sum of `|row.amount|` so widths always sum to
  // at most 100%.
  const rowsSum = rows.reduce((acc, r) => acc + Math.abs(r.amount), 0);
  const denom = Math.max(Math.abs(total), rowsSum);

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
      fullName: row.account_name,
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
    const otherLabel = `Other (${rest.length})`;
    segments.push({
      label: otherLabel,
      fullName: otherLabel,
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
  const top = segments
    .slice(0, 3)
    .map((s) => `${s.label} ${Math.round(s.pct)}%`)
    .join(', ');
  return `${which} composition: ${top}`;
}

function labelFor(seg: CompositionSegment): string {
  const rounded = Math.round(seg.pct);
  if (seg.pct >= 9) return `${seg.label} · ${rounded}%`;
  if (seg.pct >= 5) return `${rounded}%`;
  return '';
}

export function CompositionBar({ rows, total, currency, variant, className }: Props) {
  const { formatCents } = useAmountFormat();
  const [activeIdx, setActiveIdx] = useState<number | null>(null);

  if (rows.length === 0 || total === 0) return null;
  const { segments } = partitionForComposition(rows, total, variant);

  return (
    <div className={cn('w-full', className)}>
      <div className="relative">
        <div
          data-testid="composition-bar"
          role="img"
          aria-label={buildAriaLabel(segments, variant)}
          className="flex h-7 w-full overflow-hidden rounded text-[10px] font-medium"
        >
          {segments.map((seg, i) => (
            <button
              // fullName is unique per segment (unmodified account name, or "Other (N)").
              key={seg.fullName}
              type="button"
              data-testid="composition-segment"
              className={cn(
                'flex items-center overflow-hidden whitespace-nowrap transition-opacity',
                seg.colorClass,
                seg.textClass,
                seg.pct >= 9 ? 'px-1.5' : seg.pct >= 5 ? 'px-1 justify-center' : '',
                activeIdx !== null && activeIdx !== i ? 'opacity-60' : '',
              )}
              style={{ width: `${seg.pct}%` }}
              onPointerEnter={() => setActiveIdx(i)}
              onPointerLeave={() => setActiveIdx((cur) => (cur === i ? null : cur))}
              onFocus={() => setActiveIdx(i)}
              onBlur={() => setActiveIdx((cur) => (cur === i ? null : cur))}
              aria-label={`${seg.fullName}: ${formatCents(seg.amount, currency)} (${Math.round(seg.pct)}%)`}
            >
              {labelFor(seg)}
            </button>
          ))}
        </div>

        {activeIdx !== null &&
          (() => {
            const seg = segments[activeIdx];
            // Anchor tooltip at the segment's left edge in % of bar width.
            // Center it under the segment using its width; flip when near the right edge.
            const leftPctSum = segments.slice(0, activeIdx).reduce((acc, s) => acc + s.pct, 0);
            const centerPct = leftPctSum + seg.pct / 2;
            const flip = centerPct > 70;
            return (
              <div
                aria-hidden="true"
                data-testid="composition-tooltip"
                className="pointer-events-none absolute z-10 mt-1 rounded-md border border-border bg-popover px-2 py-1 text-[11px] text-popover-foreground shadow-sm"
                style={{
                  left: `${centerPct}%`,
                  top: '100%',
                  transform: flip ? 'translateX(-100%)' : 'translateX(-50%)',
                }}
              >
                <div className="font-medium">{seg.fullName}</div>
                <div className="font-mono">{formatCents(seg.amount, currency)}</div>
                <div className="text-muted-foreground">{Math.round(seg.pct)}%</div>
              </div>
            );
          })()}
      </div>

      <div className="mt-1 flex justify-between text-[9px] text-muted-foreground">
        <span>0%</span>
        <span>50%</span>
        <span>100%</span>
      </div>
    </div>
  );
}
