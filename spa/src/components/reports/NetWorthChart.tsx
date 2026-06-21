import { cn } from '@/lib/cn';
import type { DailyBalancePoint } from '@/lib/types';

interface Props {
  points: DailyBalancePoint[];
  currency: string;
  formatCents: (cents: number) => string;
  asOfDate?: string; // "YYYY-MM-DD"
  className?: string;
}

const VIEWBOX_W = 600;
const VIEWBOX_H = 180;

export function NetWorthChart({ points, currency, formatCents, asOfDate, className }: Props) {
  if (points.length < 2) return null;

  const balances = points.map((p) => p.balance);
  const min = Math.min(...balances);
  const max = Math.max(...balances);
  const range = max - min || 1;

  const xStep = VIEWBOX_W / (points.length - 1);
  const xy = points.map((p, i) => {
    const x = i * xStep;
    const y = VIEWBOX_H - ((p.balance - min) / range) * VIEWBOX_H;
    return { x, y };
  });

  const polylinePoints = xy.map((c) => `${c.x},${c.y}`).join(' ');
  const areaPoints = `0,${VIEWBOX_H} ${polylinePoints} ${VIEWBOX_W},${VIEWBOX_H}`;

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

  return (
    <div className={cn('w-full', className)}>
      <div className="flex gap-2">
        <div className="flex h-40 w-14 flex-col justify-between text-right text-[10px] text-muted-foreground">
          <span>{formatCents(max)}</span>
          <span>{formatCents(min)}</span>
        </div>
        <div className="relative h-40 flex-1">
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
          </svg>
          {markerLeftPct !== null && markerTopPct !== null && (
            <div
              data-testid="net-worth-marker"
              aria-hidden="true"
              className="pointer-events-none absolute h-2 w-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary"
              style={{ left: `${markerLeftPct}%`, top: `${markerTopPct}%` }}
            />
          )}
        </div>
      </div>
      <div className="ml-16 mt-1 flex justify-between text-[10px] text-muted-foreground">
        <span>{firstDate}</span>
        <span>{midDate}</span>
        <span>{lastDate}</span>
      </div>
    </div>
  );
}
