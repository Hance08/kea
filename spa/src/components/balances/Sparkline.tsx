import { cn } from '@/lib/cn';
import type { BalanceHistoryPoint } from '@/lib/types';

interface Props {
  points: BalanceHistoryPoint[];
  strokeClassName: string;
  className?: string;
}

const VIEWBOX_W = 100;
const VIEWBOX_H = 30;
// Inset the drawing area so the line's stroke isn't clipped at the viewBox edges.
const PAD = 2;

export function Sparkline({ points, strokeClassName, className }: Props) {
  if (points.length < 2) return null;

  const balances = points.map((p) => p.balance);
  const min = Math.min(...balances);
  const max = Math.max(...balances);
  const range = max - min;
  const drawW = VIEWBOX_W - PAD * 2;
  const drawH = VIEWBOX_H - PAD * 2;
  const xStep = drawW / (points.length - 1);

  const coords = points
    .map((p, i) => {
      const x = PAD + i * xStep;
      const y = range === 0 ? VIEWBOX_H / 2 : PAD + drawH - ((p.balance - min) / range) * drawH;
      return `${x},${y}`;
    })
    .join(' ');

  return (
    <svg
      viewBox={`0 0 ${VIEWBOX_W} ${VIEWBOX_H}`}
      preserveAspectRatio="none"
      aria-hidden="true"
      className={cn('h-6 w-20', className)}
    >
      <polyline
        points={coords}
        fill="none"
        strokeWidth={1.5}
        vectorEffect="non-scaling-stroke"
        className={strokeClassName}
      />
    </svg>
  );
}
