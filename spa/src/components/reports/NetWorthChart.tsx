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
const PAD_LEFT = 56;
const PAD_RIGHT = 12;
const PAD_TOP = 12;
const PAD_BOTTOM = 28;

export function NetWorthChart({ points, currency, formatCents, asOfDate, className }: Props) {
  if (points.length < 2) return null;

  const balances = points.map((p) => p.balance);
  const min = Math.min(...balances);
  const max = Math.max(...balances);
  const range = max - min || 1;

  const drawW = VIEWBOX_W - PAD_LEFT - PAD_RIGHT;
  const drawH = VIEWBOX_H - PAD_TOP - PAD_BOTTOM;
  const xStep = drawW / (points.length - 1);

  const xy = points.map((p, i) => {
    const x = PAD_LEFT + i * xStep;
    const y = PAD_TOP + drawH - ((p.balance - min) / range) * drawH;
    return { x, y, point: p };
  });

  const polylinePoints = xy.map((c) => `${c.x},${c.y}`).join(' ');
  const baselineY = PAD_TOP + drawH;
  const areaPoints = `${PAD_LEFT},${baselineY} ${polylinePoints} ${PAD_LEFT + (points.length - 1) * xStep},${baselineY}`;

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

  return (
    <svg
      role="img"
      aria-label={`Net worth over time, ${currency}`}
      viewBox={`0 0 ${VIEWBOX_W} ${VIEWBOX_H}`}
      preserveAspectRatio="none"
      className={cn('h-48 w-full', className)}
    >
      <polygon points={areaPoints} className="fill-primary/10" />
      <polyline
        points={polylinePoints}
        fill="none"
        strokeWidth={1.5}
        vectorEffect="non-scaling-stroke"
        className="stroke-primary"
      />

      {/* Y-axis labels */}
      <text
        x={PAD_LEFT - 6}
        y={PAD_TOP + 4}
        textAnchor="end"
        className="fill-muted-foreground text-[10px]"
      >
        {formatCents(max)}
      </text>
      <text
        x={PAD_LEFT - 6}
        y={baselineY}
        textAnchor="end"
        className="fill-muted-foreground text-[10px]"
      >
        {formatCents(min)}
      </text>

      {/* X-axis labels */}
      <text
        x={PAD_LEFT}
        y={VIEWBOX_H - 8}
        textAnchor="start"
        className="fill-muted-foreground text-[10px]"
      >
        {firstDate}
      </text>
      <text
        x={PAD_LEFT + drawW / 2}
        y={VIEWBOX_H - 8}
        textAnchor="middle"
        className="fill-muted-foreground text-[10px]"
      >
        {midDate}
      </text>
      <text
        x={PAD_LEFT + drawW}
        y={VIEWBOX_H - 8}
        textAnchor="end"
        className="fill-muted-foreground text-[10px]"
      >
        {lastDate}
      </text>

      {markerIdx !== null && (
        <>
          <line
            x1={xy[markerIdx].x}
            x2={xy[markerIdx].x}
            y1={PAD_TOP}
            y2={baselineY}
            strokeDasharray="3 3"
            vectorEffect="non-scaling-stroke"
            className="stroke-muted-foreground/40"
          />
          <circle
            cx={xy[markerIdx].x}
            cy={xy[markerIdx].y}
            r={3}
            className="fill-primary"
          />
        </>
      )}
    </svg>
  );
}
