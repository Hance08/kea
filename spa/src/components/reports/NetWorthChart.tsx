import { cn } from '@/lib/cn';
import type { DailyBalancePoint } from '@/lib/types';
import { useState } from 'react';

interface Props {
  points: DailyBalancePoint[];
  currency: string;
  formatCents: (cents: number) => string;
  asOfDate?: string; // "YYYY-MM-DD"
  className?: string;
}

const VIEWBOX_W = 600;
const VIEWBOX_H = 180;
const PAD_Y = 4;

export function NetWorthChart({ points, currency, formatCents, asOfDate, className }: Props) {
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);

  if (points.length < 2) return null;

  const balances = points.map((p) => p.balance);
  const min = Math.min(...balances);
  const max = Math.max(...balances);
  const range = max - min || 1;

  const xStep = VIEWBOX_W / (points.length - 1);
  const drawH = VIEWBOX_H - PAD_Y * 2;
  const xy = points.map((p, i) => {
    const x = i * xStep;
    const y = PAD_Y + drawH - ((p.balance - min) / range) * drawH;
    return { x, y };
  });

  const polylinePoints = xy.map((c) => `${c.x},${c.y}`).join(' ');
  const areaPoints = `0,${VIEWBOX_H} ${polylinePoints} ${VIEWBOX_W},${VIEWBOX_H}`;

  let maxIdx = 0;
  let minIdx = 0;
  for (let i = 1; i < points.length; i++) {
    if (points[i].balance > points[maxIdx].balance) maxIdx = i;
    if (points[i].balance < points[minIdx].balance) minIdx = i;
  }
  const maxLeftPct = (xy[maxIdx].x / VIEWBOX_W) * 100;
  const maxTopPct = (xy[maxIdx].y / VIEWBOX_H) * 100;
  const minLeftPct = (xy[minIdx].x / VIEWBOX_W) * 100;
  const minTopPct = (xy[minIdx].y / VIEWBOX_H) * 100;

  const firstDate = points[0].date;
  const midDate = points[Math.floor(points.length / 2)].date;
  const lastDate = points[points.length - 1].date;

  let markerIdx: number | null = null;
  if (asOfDate) {
    let best = -1;
    for (let i = 0; i < points.length; i++) {
      if (points[i].date <= asOfDate) best = i;
      else break;
    }
    if (best >= 0) markerIdx = best;
  }
  const markerLeftPct = markerIdx !== null ? (xy[markerIdx].x / VIEWBOX_W) * 100 : null;
  const markerTopPct = markerIdx !== null ? (xy[markerIdx].y / VIEWBOX_H) * 100 : null;

  const hoverLeftPct = hoverIdx !== null ? (xy[hoverIdx].x / VIEWBOX_W) * 100 : null;
  const hoverTopPct = hoverIdx !== null ? (xy[hoverIdx].y / VIEWBOX_H) * 100 : null;

  const handleMove = (e: React.PointerEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    if (rect.width === 0) return;
    const xRatio = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width));
    const idx = Math.round(xRatio * (points.length - 1));
    setHoverIdx(idx);
  };

  return (
    <div className={cn('w-full', className)}>
      <div
        className="relative mt-4 h-40 w-full touch-none"
        onPointerMove={handleMove}
        onPointerEnter={handleMove}
        onPointerLeave={() => setHoverIdx(null)}
      >
        <svg
          role="img"
          aria-label={`Net worth over time, ${currency}`}
          viewBox={`0 0 ${VIEWBOX_W} ${VIEWBOX_H}`}
          preserveAspectRatio="none"
          className="h-full w-full"
        >
          <polygon points={areaPoints} className="fill-primary/10" />
          <polyline
            points={polylinePoints}
            fill="none"
            strokeWidth={1.5}
            vectorEffect="non-scaling-stroke"
            className="stroke-primary"
          />
          {markerIdx !== null && (
            <line
              x1={xy[markerIdx].x}
              x2={xy[markerIdx].x}
              y1={0}
              y2={VIEWBOX_H}
              strokeDasharray="3 3"
              vectorEffect="non-scaling-stroke"
              className="stroke-muted-foreground/40"
            />
          )}
          {hoverIdx !== null && (
            <line
              x1={xy[hoverIdx].x}
              x2={xy[hoverIdx].x}
              y1={0}
              y2={VIEWBOX_H}
              vectorEffect="non-scaling-stroke"
              className="stroke-muted-foreground/60"
            />
          )}
        </svg>

        <div
          aria-hidden="true"
          className="pointer-events-none absolute h-1.5 w-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary"
          style={{ left: `${maxLeftPct}%`, top: `${maxTopPct}%` }}
        />
        <div
          className="pointer-events-none absolute"
          style={{ left: `${maxLeftPct}%`, top: `${maxTopPct}%` }}
        >
          <span
            className={cn(
              'absolute whitespace-nowrap text-[10px] font-medium text-foreground',
              maxLeftPct > 50 ? 'bottom-1 right-2' : 'bottom-1 left-2',
            )}
          >
            {formatCents(max)}
          </span>
        </div>

        <div
          aria-hidden="true"
          className="pointer-events-none absolute h-1.5 w-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary"
          style={{ left: `${minLeftPct}%`, top: `${minTopPct}%` }}
        />
        <div
          className="pointer-events-none absolute"
          style={{ left: `${minLeftPct}%`, top: `${minTopPct}%` }}
        >
          <span
            className={cn(
              'absolute whitespace-nowrap text-[10px] font-medium text-foreground',
              minLeftPct > 50 ? 'bottom-1 right-2' : 'bottom-1 left-2',
            )}
          >
            {formatCents(min)}
          </span>
        </div>

        {markerLeftPct !== null && markerTopPct !== null && (
          <div
            data-testid="net-worth-marker"
            aria-hidden="true"
            className="pointer-events-none absolute h-2 w-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary"
            style={{ left: `${markerLeftPct}%`, top: `${markerTopPct}%` }}
          />
        )}

        {hoverIdx !== null && hoverLeftPct !== null && hoverTopPct !== null && (
          <>
            <div
              data-testid="net-worth-hover-dot"
              aria-hidden="true"
              className="pointer-events-none absolute h-2 w-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary ring-2 ring-background"
              style={{ left: `${hoverLeftPct}%`, top: `${hoverTopPct}%` }}
            />
            <div
              role="status"
              aria-live="polite"
              className={cn(
                'pointer-events-none absolute z-10 -translate-y-full rounded-md border border-border bg-popover px-2 py-1 text-[11px] text-popover-foreground shadow-sm',
                hoverLeftPct > 70 ? '-translate-x-full' : hoverLeftPct < 30 ? '' : '-translate-x-1/2',
              )}
              style={{ left: `${hoverLeftPct}%`, top: `${Math.max(hoverTopPct - 4, 0)}%` }}
            >
              <div className="font-medium">{formatCents(points[hoverIdx].balance)}</div>
              <div className="text-muted-foreground">{points[hoverIdx].date}</div>
            </div>
          </>
        )}
      </div>
      <div className="mt-1 flex justify-between text-[10px] text-muted-foreground">
        <span>{firstDate}</span>
        <span>{midDate}</span>
        <span>{lastDate}</span>
      </div>
    </div>
  );
}
